import type { Guild, GuildMember } from 'discord.js';

import type { AssignmentRecord, CreateAssignmentInput } from '../services/assignmentService.js';
import type { AuditLogInput } from '../services/auditLogService.js';
import type {
  ChannelIngestionPolicyRecord,
  SetChannelIngestionPolicyInput
} from '../services/channelIngestionPolicyService.js';
import type {
  AgentActionRecord,
  CreateAgentActionInput,
  AgentActionScope,
  AgentActionStatus,
  UpdateAgentActionStatusInput
} from '../services/agentActionService.js';
import type { RecordModelUsageInput } from '../services/modelUsageService.js';
import type { Capability } from '../services/rolePolicyService.js';

export interface AssignmentStore {
  createAssignment(input: CreateAssignmentInput): Promise<AssignmentRecord>;
  getAssignmentById(guildId: string, assignmentId: string): Promise<AssignmentRecord | null>;
  listAssignments(guildId: string): Promise<AssignmentRecord[]>;
  markPublished(guildId: string, assignmentId: string, messageId: string, channelId: string): Promise<AssignmentRecord>;
}

export interface AuditLogStore {
  record(input: AuditLogInput): Promise<void>;
}

export interface ModelUsageStore {
  record(input: RecordModelUsageInput): Promise<void>;
  listDailySummary(input: {
    days: number;
    guildId: string;
  }): Promise<Array<{
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
  }>>;
  listRequesterDailySummary(input: {
    days: number;
    guildId: string;
    requesterUserId: string;
  }): Promise<Array<{
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
  }>>;
}

export interface AgentActionStore {
  createAction(input: CreateAgentActionInput): Promise<AgentActionRecord>;
  getActionById(actionId: string): Promise<AgentActionRecord | null>;
  listVisibleRecentForUser(
    userId: string,
    limit: number,
    options?: {
      actionScope?: AgentActionScope;
      statuses?: AgentActionStatus[];
    }
  ): Promise<AgentActionRecord[]>;
  updateActionStatus(input: UpdateAgentActionStatusInput): Promise<AgentActionRecord>;
}

export interface ChannelIngestionPolicyStore {
  getPolicy(guildId: string, channelId: string): Promise<ChannelIngestionPolicyRecord | null>;
  isChannelEnabled(guildId: string, channelId: string): Promise<boolean>;
  setChannelEnabled(input: SetChannelIngestionPolicyInput): Promise<ChannelIngestionPolicyRecord>;
}

export interface RolePolicyStore {
  ensureGuild(guild: Guild): Promise<void>;
  grantUserCapability(input: {
    capability: Capability;
    grantedByUserId: string;
    guildId: string;
    userId: string;
  }): Promise<void>;
  listDirectUserCapabilities(guildId: string, userId: string): Promise<Capability[]>;
  memberHasCapability(guild: Guild, member: GuildMember, capability: Capability): Promise<boolean>;
  revokeUserCapability(guildId: string, userId: string, capability: Capability): Promise<boolean>;
}
