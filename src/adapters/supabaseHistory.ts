import type { SupabaseClient } from '@supabase/supabase-js';

import type { PendingDmScopeSelection, PendingDmScopeSelectionStore } from '../ports/conversation.js';
import type {
  HistoryMessageRecord,
  HistoryScope,
  MessageEmbeddingRecord,
  MessageHistoryRepository,
  SemanticSearchHit,
  StoredMessageAttachmentInput,
  StoredMessageInput
} from '../ports/history.js';

interface PendingDmScopeSelectionRow {
  created_at: string;
  expires_at: string;
  id: string;
  query: string;
  scope_options: PendingDmScopeSelection['scopeOptions'];
  user_id: string;
}

export class SupabaseMessageHistoryRepository implements MessageHistoryRepository {
  constructor(private readonly supabase: SupabaseClient) {}

  async saveMessage(message: StoredMessageInput): Promise<void> {
    const { error } = await this.supabase.from('messages').upsert(
      {
        id: message.id,
        guild_id: message.guildId,
        channel_id: message.channelId,
        thread_id: message.threadId,
        dm_user_id: message.dmUserId,
        author_user_id: message.authorUserId,
        author_username: message.authorUsername,
        author_is_bot: message.authorIsBot,
        content: message.content,
        attachment_count: message.attachmentCount,
        created_at: message.createdAt,
        edited_at: message.editedAt,
        indexed_at: message.indexedAt
      },
      {
        onConflict: 'id'
      }
    );

    if (error) {
      throw new Error(`Failed to store Discord message: ${error.message}`);
    }
  }

  async saveMessageAttachments(attachments: StoredMessageAttachmentInput[]): Promise<void> {
    if (attachments.length === 0) {
      return;
    }

    const { error } = await this.supabase.from('message_attachments').upsert(
      attachments.map((attachment) => ({
        id: attachment.id,
        message_id: attachment.messageId,
        url: attachment.url,
        filename: attachment.filename,
        content_type: attachment.contentType,
        size_bytes: attachment.sizeBytes,
        height: attachment.height,
        width: attachment.width
      })),
      {
        onConflict: 'id'
      }
    );

    if (error) {
      throw new Error(`Failed to store message attachments: ${error.message}`);
    }
  }

  async upsertMessageEmbedding(record: MessageEmbeddingRecord): Promise<void> {
    const { error } = await this.supabase.from('message_embeddings').upsert(
      {
        message_id: record.messageId,
        embedding: record.embedding,
        model: record.model,
        embedded_text: record.embeddedText,
        updated_at: record.updatedAt
      },
      {
        onConflict: 'message_id'
      }
    );

    if (error) {
      throw new Error(error.message);
    }
  }

  async listRecentMessages(scope: HistoryScope, limit: number): Promise<HistoryMessageRecord[]> {
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

  async countPhrase(scope: HistoryScope, phrase: string, authorUserId: string | null): Promise<number> {
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

  async searchSemantic(scope: HistoryScope, queryEmbedding: number[], limit: number): Promise<SemanticSearchHit[]> {
    const { data, error } = await this.supabase.rpc('match_message_embeddings', {
      filter_channel_id: scope.channelId ?? null,
      filter_dm_user_id: scope.dmUserId ?? null,
      filter_guild_id: scope.guildId ?? null,
      match_count: limit,
      query_embedding: queryEmbedding
    });

    if (error) {
      throw new Error(`Failed to search message embeddings: ${error.message}`);
    }

    return (data ?? []) as SemanticSearchHit[];
  }
}

export class SupabasePendingDmScopeSelectionStore implements PendingDmScopeSelectionStore {
  constructor(private readonly supabase: SupabaseClient) {}

  async save(selection: PendingDmScopeSelection, expiresAt: Date): Promise<void> {
    const { error } = await this.supabase.from('pending_dm_scope_selections').upsert(
      {
        id: selection.id,
        user_id: selection.userId,
        query: selection.query,
        scope_options: selection.scopeOptions,
        created_at: new Date(selection.createdAt).toISOString(),
        expires_at: expiresAt.toISOString()
      },
      {
        onConflict: 'id'
      }
    );

    if (error) {
      throw new Error(`Failed to save pending DM scope selection: ${error.message}`);
    }
  }

  async get(selectionId: string): Promise<PendingDmScopeSelection | null> {
    const { data, error } = await this.supabase
      .from('pending_dm_scope_selections')
      .select('*')
      .eq('id', selectionId)
      .maybeSingle();

    if (error) {
      throw new Error(`Failed to load pending DM scope selection: ${error.message}`);
    }

    const row = (data as PendingDmScopeSelectionRow | null) ?? null;
    if (!row) {
      return null;
    }

    return {
      createdAt: new Date(row.created_at).getTime(),
      id: row.id,
      query: row.query,
      scopeOptions: row.scope_options,
      userId: row.user_id
    };
  }

  async delete(selectionId: string): Promise<void> {
    const { error } = await this.supabase
      .from('pending_dm_scope_selections')
      .delete()
      .eq('id', selectionId);

    if (error) {
      throw new Error(`Failed to delete pending DM scope selection: ${error.message}`);
    }
  }

  async deleteExpired(now: Date): Promise<void> {
    const { error } = await this.supabase
      .from('pending_dm_scope_selections')
      .delete()
      .lt('expires_at', now.toISOString());

    if (error) {
      throw new Error(`Failed to clean up expired DM scope selections: ${error.message}`);
    }
  }
}
