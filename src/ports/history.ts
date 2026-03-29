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

export interface StoredMessageInput {
  attachmentCount: number;
  authorIsBot: boolean;
  authorUserId: string;
  authorUsername: string;
  channelId: string;
  content: string;
  createdAt: string;
  dmUserId: string | null;
  editedAt: string | null;
  guildId: string | null;
  id: string;
  indexedAt: string;
  threadId: string | null;
}

export interface StoredMessageAttachmentInput {
  contentType: string | null;
  filename: string | null;
  height: number | null;
  id: string;
  messageId: string;
  sizeBytes: number;
  url: string;
  width: number | null;
}

export interface MessageEmbeddingRecord {
  embeddedText: string;
  embedding: number[];
  messageId: string;
  model: string;
  updatedAt: string;
}

export interface MessageHistoryRepository {
  countPhrase(scope: HistoryScope, phrase: string, authorUserId: string | null): Promise<number>;
  listRecentMessages(scope: HistoryScope, limit: number): Promise<HistoryMessageRecord[]>;
  saveMessage(message: StoredMessageInput): Promise<void>;
  saveMessageAttachments(attachments: StoredMessageAttachmentInput[]): Promise<void>;
  searchSemantic(scope: HistoryScope, queryEmbedding: number[], limit: number): Promise<SemanticSearchHit[]>;
  upsertMessageEmbedding(record: MessageEmbeddingRecord): Promise<void>;
}
