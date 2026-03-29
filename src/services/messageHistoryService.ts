import type { Message, Snowflake, User } from 'discord.js';

import type { Env } from '../config/env.js';
import type { Logger } from '../lib/logger.js';
import type { EmbeddingClient } from '../ports/ai.js';
import type {
  HistoryMessageRecord,
  HistoryScope,
  MessageHistoryRepository,
  SemanticSearchHit
} from '../ports/history.js';
import type { ChannelIngestionPolicyService } from './channelIngestionPolicyService.js';
import type { MessageIndexingService } from './messageIndexingService.js';
import type { RolePolicyService } from './rolePolicyService.js';

export type { HistoryMessageRecord, HistoryScope, SemanticSearchHit } from '../ports/history.js';

export interface MessageStoreResult {
  reason: 'stored' | 'skipped_by_ingestion_policy' | 'unsupported_scope' | 'system_message';
  stored: boolean;
}

export class MessageHistoryService {
  constructor(
    private readonly env: Env,
    private readonly history: MessageHistoryRepository,
    private readonly embeddings: EmbeddingClient,
    private readonly channelIngestionPolicies: ChannelIngestionPolicyService,
    private readonly messageIndexing: MessageIndexingService,
    private readonly rolePolicies: RolePolicyService,
    private readonly logger: Logger
  ) {}

  async storeDiscordMessage(message: Message): Promise<MessageStoreResult> {
    if (message.system) {
      return {
        reason: 'system_message',
        stored: false
      };
    }

    const scope = resolveScope(message);
    if (!scope) {
      return {
        reason: 'unsupported_scope',
        stored: false
      };
    }

    if (message.guild) {
      await this.rolePolicies.ensureGuild(message.guild);

      const enabled = await this.channelIngestionPolicies.isChannelEnabled(message.guild.id, message.channelId);
      if (!enabled) {
        this.logger.debug('Skipped guild message storage because ingestion is disabled for the channel', {
          channelId: message.channelId,
          guildId: message.guild.id,
          messageId: message.id
        });

        return {
          reason: 'skipped_by_ingestion_policy',
          stored: false
        };
      }
    }

    const content = message.content.trim();
    const attachments = [...message.attachments.values()];

    await this.history.saveMessage({
      id: message.id,
      guildId: scope.guildId,
      channelId: message.channelId,
      threadId: message.channel.isThread() ? message.channel.id : null,
      dmUserId: scope.dmUserId,
      authorUserId: message.author.id,
      authorUsername: message.author.username,
      authorIsBot: message.author.bot,
      content,
      attachmentCount: attachments.length,
      createdAt: message.createdAt.toISOString(),
      editedAt: message.editedTimestamp ? new Date(message.editedTimestamp).toISOString() : null,
      indexedAt: new Date().toISOString()
    });

    if (attachments.length > 0) {
      await this.history.saveMessageAttachments(
        attachments.map((attachment) => ({
          id: attachment.id,
          messageId: message.id,
          url: attachment.url,
          filename: attachment.name,
          contentType: attachment.contentType ?? null,
          sizeBytes: attachment.size,
          height: attachment.height ?? null,
          width: attachment.width ?? null
        }))
      );
    }

    if (content.length === 0) {
      return {
        reason: 'stored',
        stored: true
      };
    }

    this.messageIndexing.enqueue({
      content,
      messageId: message.id
    });

    return {
      reason: 'stored',
      stored: true
    };
  }

  async listRecentMessages(scope: HistoryScope, limit = 8): Promise<HistoryMessageRecord[]> {
    return this.history.listRecentMessages(scope, limit);
  }

  async countPhrase(
    scope: HistoryScope,
    phrase: string,
    authorUserId: string | null
  ): Promise<number> {
    return this.history.countPhrase(scope, phrase, authorUserId);
  }

  async searchSemantic(scope: HistoryScope, query: string, limit = 8): Promise<SemanticSearchHit[]> {
    const vector = await this.embeddings.createEmbedding(this.env.OPENAI_EMBEDDING_MODEL, query.slice(0, 4000));
    return this.history.searchSemantic(scope, vector, limit);
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
