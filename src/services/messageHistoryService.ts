import type { Message, Snowflake, User } from 'discord.js';
import type OpenAI from 'openai';
import type { SupabaseClient } from '@supabase/supabase-js';

import type { Env } from '../config/env.js';
import type { Logger } from '../lib/logger.js';
import type { RolePolicyService } from './rolePolicyService.js';

export interface HistoryScope {
  kind: 'dm' | 'guild';
  channelId?: string | null;
  dmUserId?: string | null;
  guildId?: string | null;
}

export interface HistoryMessageRecord {
  id: string;
  guild_id: string | null;
  channel_id: string;
  thread_id: string | null;
  dm_user_id: string | null;
  author_user_id: string;
  author_username: string;
  author_is_bot: boolean;
  content: string;
  created_at: string;
}

export interface SemanticSearchHit extends HistoryMessageRecord {
  similarity: number;
}

const MAX_EMBEDDING_INPUT_CHARS = 4000;

export class MessageHistoryService {
  constructor(
    private readonly env: Env,
    private readonly supabase: SupabaseClient,
    private readonly openai: OpenAI,
    private readonly rolePolicies: RolePolicyService,
    private readonly logger: Logger
  ) {}

  async storeDiscordMessage(message: Message): Promise<void> {
    if (message.system) {
      return;
    }

    const scope = resolveScope(message);
    if (!scope) {
      return;
    }

    if (message.guild) {
      await this.rolePolicies.ensureGuild(message.guild);
    }

    const content = message.content.trim();
    const attachments = [...message.attachments.values()];

    const { error } = await this.supabase.from('messages').upsert(
      {
        id: message.id,
        guild_id: scope.guildId,
        channel_id: message.channelId,
        thread_id: message.channel.isThread() ? message.channel.id : null,
        dm_user_id: scope.dmUserId,
        author_user_id: message.author.id,
        author_username: message.author.username,
        author_is_bot: message.author.bot,
        content,
        attachment_count: attachments.length,
        created_at: message.createdAt.toISOString(),
        edited_at: message.editedTimestamp ? new Date(message.editedTimestamp).toISOString() : null,
        indexed_at: new Date().toISOString()
      },
      {
        onConflict: 'id'
      }
    );

    if (error) {
      throw new Error(`Failed to store Discord message: ${error.message}`);
    }

    if (attachments.length > 0) {
      const { error: attachmentError } = await this.supabase.from('message_attachments').upsert(
        attachments.map((attachment) => ({
          id: attachment.id,
          message_id: message.id,
          url: attachment.url,
          filename: attachment.name,
          content_type: attachment.contentType,
          size_bytes: attachment.size,
          height: attachment.height ?? null,
          width: attachment.width ?? null
        })),
        {
          onConflict: 'id'
        }
      );

      if (attachmentError) {
        throw new Error(`Failed to store message attachments: ${attachmentError.message}`);
      }
    }

    if (content.length === 0) {
      return;
    }

    try {
      const embedding = await this.openai.embeddings.create({
        model: this.env.OPENAI_EMBEDDING_MODEL,
        input: content.slice(0, MAX_EMBEDDING_INPUT_CHARS)
      });

      const vector = embedding.data[0]?.embedding;
      if (!vector) {
        this.logger.warn('OpenAI embedding response was empty', {
          messageId: message.id
        });
        return;
      }

      const { error: embeddingError } = await this.supabase.from('message_embeddings').upsert(
        {
          message_id: message.id,
          embedding: vector,
          model: this.env.OPENAI_EMBEDDING_MODEL,
          embedded_text: content.slice(0, MAX_EMBEDDING_INPUT_CHARS),
          updated_at: new Date().toISOString()
        },
        {
          onConflict: 'message_id'
        }
      );

      if (embeddingError) {
        throw new Error(embeddingError.message);
      }
    } catch (error) {
      const messageText = error instanceof Error ? error.message : 'Unknown embedding error';
      this.logger.warn('Failed to embed Discord message', {
        messageId: message.id,
        error: messageText
      });
    }
  }

  async listRecentMessages(scope: HistoryScope, limit = 8): Promise<HistoryMessageRecord[]> {
    let query = this.supabase
      .from('messages')
      .select('*')
      .order('created_at', { ascending: false })
      .limit(limit);

    if (scope.guildId) {
      query = query.eq('guild_id', scope.guildId);
    } else {
      query = query.is('guild_id', null);
    }

    if (scope.dmUserId) {
      query = query.eq('dm_user_id', scope.dmUserId);
    }

    if (scope.channelId) {
      query = query.eq('channel_id', scope.channelId);
    }

    const { data, error } = await query;
    if (error) {
      throw new Error(`Failed to load recent messages: ${error.message}`);
    }

    return ((data ?? []) as HistoryMessageRecord[]).reverse();
  }

  async countPhrase(
    scope: HistoryScope,
    phrase: string,
    authorUserId: string | null
  ): Promise<number> {
    const { data, error } = await this.supabase.rpc('count_message_phrase', {
      filter_author_user_id: authorUserId,
      filter_channel_id: scope.channelId ?? null,
      filter_dm_user_id: scope.dmUserId ?? null,
      filter_guild_id: scope.guildId ?? null,
      phrase
    });

    if (error) {
      throw new Error(`Failed to count phrase usage: ${error.message}`);
    }

    return Number(data ?? 0);
  }

  async searchSemantic(scope: HistoryScope, query: string, limit = 8): Promise<SemanticSearchHit[]> {
    const embedding = await this.openai.embeddings.create({
      model: this.env.OPENAI_EMBEDDING_MODEL,
      input: query.slice(0, MAX_EMBEDDING_INPUT_CHARS)
    });

    const vector = embedding.data[0]?.embedding;
    if (!vector) {
      return [];
    }

    const { data, error } = await this.supabase.rpc('match_message_embeddings', {
      filter_channel_id: scope.channelId ?? null,
      filter_dm_user_id: scope.dmUserId ?? null,
      filter_guild_id: scope.guildId ?? null,
      match_count: limit,
      query_embedding: vector
    });

    if (error) {
      throw new Error(`Failed to search message embeddings: ${error.message}`);
    }

    return (data ?? []) as SemanticSearchHit[];
  }
}

function resolveScope(
  message: Message
): { dmUserId: string | null; guildId: string | null } | null {
  if (message.inGuild()) {
    return {
      dmUserId: null,
      guildId: message.guildId
    };
  }

  const dmUserId = resolveDmUserId(message);
  if (!dmUserId) {
    return null;
  }

  return {
    dmUserId,
    guildId: null
  };
}

function resolveDmUserId(message: Message): Snowflake | null {
  const channel = message.channel;
  if (!channel.isDMBased()) {
    return null;
  }

  if ('recipientId' in channel && channel.recipientId) {
    return channel.recipientId;
  }

  if ('recipient' in channel && channel.recipient) {
    return channel.recipient.id;
  }

  if (!message.author.bot) {
    return message.author.id;
  }

  return null;
}

export function resolvePrimaryGuildScope(guildId: string): HistoryScope {
  return {
    kind: 'guild',
    guildId
  };
}

export function resolveDmScope(user: User): HistoryScope {
  return {
    kind: 'dm',
    dmUserId: user.id
  };
}
