import type { Env } from '../config/env.js';
import type { Logger } from '../lib/logger.js';
import type { EmbeddingClient } from '../ports/ai.js';
import type { MessageHistoryRepository } from '../ports/history.js';
import type { ModelUsageService } from './modelUsageService.js';

const MAX_EMBEDDING_INPUT_CHARS = 4000;

export interface MessageIndexingTask {
  content: string;
  messageId: string;
}

export interface MessageIndexingStatus {
  activeMessageId: string | null;
  indexedCount: number;
  lastError: string | null;
  lastErrorAt: string | null;
  lastSuccessAt: string | null;
  pendingJobs: number;
  processing: boolean;
  startedAt: string;
}

export class MessageIndexingService {
  private activeMessageId: string | null = null;
  private indexedCount = 0;
  private lastError: string | null = null;
  private lastErrorAt: string | null = null;
  private lastSuccessAt: string | null = null;
  private readonly pendingJobs = new Map<string, MessageIndexingTask>();
  private processing = false;
  private readonly startedAt = new Date().toISOString();

  constructor(
    private readonly env: Env,
    private readonly embeddings: EmbeddingClient,
    private readonly history: MessageHistoryRepository,
    private readonly modelUsage: ModelUsageService,
    private readonly logger: Logger
  ) {}

  enqueue(task: MessageIndexingTask): void {
    const content = task.content.trim();
    if (content.length === 0) {
      return;
    }

    this.pendingJobs.set(task.messageId, {
      ...task,
      content: content.slice(0, MAX_EMBEDDING_INPUT_CHARS)
    });

    void this.drainQueue();
  }

  getStatus(): MessageIndexingStatus {
    return {
      activeMessageId: this.activeMessageId,
      indexedCount: this.indexedCount,
      lastError: this.lastError,
      lastErrorAt: this.lastErrorAt,
      lastSuccessAt: this.lastSuccessAt,
      pendingJobs: this.pendingJobs.size,
      processing: this.processing,
      startedAt: this.startedAt
    };
  }

  private async drainQueue(): Promise<void> {
    if (this.processing) {
      return;
    }

    this.processing = true;

    try {
      while (this.pendingJobs.size > 0) {
        const [messageId, task] = this.pendingJobs.entries().next().value as [string, MessageIndexingTask];
        this.pendingJobs.delete(messageId);
        this.activeMessageId = messageId;

        try {
          await this.indexMessage(task);
          this.indexedCount += 1;
          this.lastSuccessAt = new Date().toISOString();
        } catch (error) {
          const message = error instanceof Error ? error.message : 'Unknown indexing error';
          this.lastError = message;
          this.lastErrorAt = new Date().toISOString();
          this.logger.warn('Failed to index Discord message embedding', {
            messageId,
            error: message
          });
        } finally {
          this.activeMessageId = null;
        }
      }
    } finally {
      this.processing = false;
    }
  }

  private async indexMessage(task: MessageIndexingTask): Promise<void> {
    const embedding = await this.embeddings.createEmbedding(this.env.OPENAI_EMBEDDING_MODEL, task.content);

    if (embedding.usage) {
      await this.modelUsage.record({
        channelId: null,
        guildId: null,
        inputTokens: embedding.usage.inputTokens,
        messageId: task.messageId,
        metadata: {
          contentLength: task.content.length
        },
        model: this.env.OPENAI_EMBEDDING_MODEL,
        operation: 'message_index_embedding',
        outputTokens: embedding.usage.outputTokens,
        provider: 'openai',
        requesterUserId: null,
        surface: 'background',
        totalTokens: embedding.usage.totalTokens
      });
    }

    await this.history.upsertMessageEmbedding({
      embeddedText: task.content,
      embedding: embedding.vector,
      messageId: task.messageId,
      model: this.env.OPENAI_EMBEDDING_MODEL,
      updatedAt: new Date().toISOString()
    });
  }
}
