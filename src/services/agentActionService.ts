import type { AgentActionStore } from '../ports/controlPlane.js';

export const AGENT_ACTION_TYPES = {
  dmRelay: 'dm_relay'
} as const;

export const AGENT_ACTION_VISIBILITIES = {
  guild: 'guild',
  participants: 'participants',
  requesterOnly: 'requester_only'
} as const;

export const AGENT_ACTION_STATUSES = {
  cancelled: 'cancelled',
  completed: 'completed',
  failed: 'failed',
  requested: 'requested'
} as const;

export type AgentActionType = (typeof AGENT_ACTION_TYPES)[keyof typeof AGENT_ACTION_TYPES];
export type AgentActionVisibility = (typeof AGENT_ACTION_VISIBILITIES)[keyof typeof AGENT_ACTION_VISIBILITIES];
export type AgentActionStatus = (typeof AGENT_ACTION_STATUSES)[keyof typeof AGENT_ACTION_STATUSES];

export interface AgentActionRecord {
  id: string;
  guild_id: string | null;
  channel_id: string | null;
  requester_user_id: string;
  requester_username: string;
  recipient_user_id: string | null;
  recipient_username: string | null;
  action_type: AgentActionType;
  status: AgentActionStatus;
  visibility: AgentActionVisibility;
  title: string;
  instructions: string;
  result_summary: string | null;
  error_message: string | null;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  completed_at: string | null;
}

export interface CreateAgentActionInput {
  guildId: string | null;
  channelId: string | null;
  requesterUserId: string;
  requesterUsername: string;
  recipientUserId: string | null;
  recipientUsername: string | null;
  actionType: AgentActionType;
  visibility: AgentActionVisibility;
  title: string;
  instructions: string;
  metadata?: Record<string, unknown>;
}

export interface UpdateAgentActionStatusInput {
  actionId: string;
  status: AgentActionStatus;
  resultSummary?: string | null;
  errorMessage?: string | null;
  metadata?: Record<string, unknown>;
  completedAt?: string | null;
}

export interface CreateDirectMessageRelayInput {
  guildId: string | null;
  channelId: string | null;
  requesterUserId: string;
  requesterUsername: string;
  recipientUserId: string;
  recipientUsername: string;
  message: string;
  context?: string | null;
  metadata?: Record<string, unknown>;
}

export interface MarkAgentActionResultInput {
  metadata?: Record<string, unknown>;
  resultSummary?: string | null;
}

export class AgentActionService {
  constructor(private readonly store: AgentActionStore) {}

  async createDirectMessageRelay(input: CreateDirectMessageRelayInput): Promise<AgentActionRecord> {
    return this.store.createAction({
      guildId: input.guildId,
      channelId: input.channelId,
      requesterUserId: input.requesterUserId,
      requesterUsername: input.requesterUsername,
      recipientUserId: input.recipientUserId,
      recipientUsername: input.recipientUsername,
      actionType: AGENT_ACTION_TYPES.dmRelay,
      visibility: AGENT_ACTION_VISIBILITIES.participants,
      title: `DM relay from ${input.requesterUsername} to ${input.recipientUsername}`,
      instructions: input.message,
      metadata: {
        ...(input.context ? { context: input.context } : {}),
        ...(input.metadata ?? {})
      }
    });
  }

  async markCompleted(
    action: AgentActionRecord,
    input: MarkAgentActionResultInput = {}
  ): Promise<AgentActionRecord> {
    return this.store.updateActionStatus({
      actionId: action.id,
      completedAt: new Date().toISOString(),
      errorMessage: null,
      metadata: {
        ...(action.metadata ?? {}),
        ...(input.metadata ?? {})
      },
      resultSummary: input.resultSummary ?? null,
      status: AGENT_ACTION_STATUSES.completed
    });
  }

  async markFailed(
    action: AgentActionRecord,
    errorMessage: string,
    metadata?: Record<string, unknown>
  ): Promise<AgentActionRecord> {
    return this.store.updateActionStatus({
      actionId: action.id,
      completedAt: new Date().toISOString(),
      errorMessage,
      metadata: {
        ...(action.metadata ?? {}),
        ...(metadata ?? {})
      },
      resultSummary: null,
      status: AGENT_ACTION_STATUSES.failed
    });
  }

  async listRelevantVisibleActionsForUser(
    userId: string,
    query: string,
    limit = 4
  ): Promise<AgentActionRecord[]> {
    const recent = await this.store.listVisibleRecentForUser(userId, 20);
    const queryTokens = tokenize(query);
    const ranked = recent
      .map((action) => ({
        action,
        score: scoreAction(action, queryTokens)
      }))
      .filter(({ score }, index) => score > 0 || (queryTokens.size === 0 && index < limit))
      .sort((left, right) => {
        if (right.score !== left.score) {
          return right.score - left.score;
        }

        return Date.parse(right.action.created_at) - Date.parse(left.action.created_at);
      });

    if (ranked.length === 0) {
      return recent.slice(0, limit);
    }

    return ranked.slice(0, limit).map(({ action }) => action);
  }
}

function scoreAction(action: AgentActionRecord, queryTokens: Set<string>): number {
  if (queryTokens.size === 0) {
    return 0;
  }

  const haystack = tokenize([
    action.title,
    action.instructions,
    action.requester_username,
    action.recipient_username,
    action.result_summary,
    action.error_message,
    typeof action.metadata.context === 'string' ? action.metadata.context : ''
  ].join(' '));

  let score = 0;
  for (const token of queryTokens) {
    if (haystack.has(token)) {
      score += 1;
    }
  }

  if (action.status === AGENT_ACTION_STATUSES.completed) {
    score += 1;
  }

  if (action.action_type === AGENT_ACTION_TYPES.dmRelay) {
    score += 1;
  }

  return score;
}

function tokenize(input: string): Set<string> {
  return new Set(
    input
      .toLowerCase()
      .split(/[^a-z0-9_]+/i)
      .map((token) => token.trim())
      .filter((token) => token.length >= 3)
  );
}
