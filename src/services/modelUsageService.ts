import type { ModelUsageStore } from '../ports/controlPlane.js';

export type ModelUsageOperation =
  | 'retrieval_response'
  | 'dm_tool_planning'
  | 'semantic_query_embedding'
  | 'message_index_embedding';

export type ModelUsageSurface = 'background' | 'dm' | 'guild_mention';

export interface RecordModelUsageInput {
  channelId?: string | null;
  guildId?: string | null;
  inputTokens: number | null;
  messageId?: string | null;
  metadata?: Record<string, unknown>;
  model: string;
  operation: ModelUsageOperation;
  outputTokens: number | null;
  provider: 'openai';
  requesterUserId?: string | null;
  surface: ModelUsageSurface;
  totalTokens: number | null;
}

export interface ModelUsageDailySummaryRow {
  estimatedCostUsd: number;
  eventCount: number;
  inputTokens: number;
  model: string;
  operation: string;
  outputTokens: number;
  provider: string;
  surface: string;
  totalTokens: number;
  usageDay: string;
}

export interface ModelUsageRequesterDailySummaryRow {
  estimatedCostUsd: number;
  eventCount: number;
  inputTokens: number;
  operation: string;
  outputTokens: number;
  provider: string;
  requesterUserId: string;
  surface: string;
  totalTokens: number;
  usageDay: string;
}

export class ModelUsageService {
  constructor(private readonly store: ModelUsageStore) {}

  async record(input: RecordModelUsageInput): Promise<void> {
    await this.store.record({
      channelId: input.channelId ?? null,
      guildId: input.guildId ?? null,
      inputTokens: input.inputTokens,
      messageId: input.messageId ?? null,
      metadata: input.metadata ?? {},
      model: input.model,
      operation: input.operation,
      outputTokens: input.outputTokens,
      provider: input.provider,
      requesterUserId: input.requesterUserId ?? null,
      surface: input.surface,
      totalTokens: input.totalTokens
    });
  }

  async listDailySummary(input: {
    days: number;
    guildId: string;
  }): Promise<ModelUsageDailySummaryRow[]> {
    return this.store.listDailySummary(input);
  }

  async listRequesterDailySummary(input: {
    days: number;
    guildId: string;
    requesterUserId: string;
  }): Promise<ModelUsageRequesterDailySummaryRow[]> {
    return this.store.listRequesterDailySummary(input);
  }
}
