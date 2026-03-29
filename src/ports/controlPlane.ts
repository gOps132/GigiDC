import type { Guild, GuildMember } from 'discord.js';

import type { AssignmentRecord, CreateAssignmentInput } from '../services/assignmentService.js';
import type { AuditLogInput } from '../services/auditLogService.js';
import type {
  ChannelIngestionPolicyRecord,
  SetChannelIngestionPolicyInput
} from '../services/channelIngestionPolicyService.js';
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

export interface ChannelIngestionPolicyStore {
  getPolicy(guildId: string, channelId: string): Promise<ChannelIngestionPolicyRecord | null>;
  isChannelEnabled(guildId: string, channelId: string): Promise<boolean>;
  setChannelEnabled(input: SetChannelIngestionPolicyInput): Promise<ChannelIngestionPolicyRecord>;
}

export interface RolePolicyStore {
  ensureGuild(guild: Guild): Promise<void>;
  memberHasCapability(guild: Guild, member: GuildMember, capability: Capability): Promise<boolean>;
}
