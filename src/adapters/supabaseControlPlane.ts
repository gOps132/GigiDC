import {
  PermissionFlagsBits,
  type Guild,
  type GuildMember
} from 'discord.js';
import type { SupabaseClient } from '@supabase/supabase-js';

import type {
  AgentActionStore,
  AssignmentStore,
  AuditLogStore,
  ChannelIngestionPolicyStore,
  RolePolicyStore
} from '../ports/controlPlane.js';
import type {
  AgentActionRecord,
  CreateAgentActionInput,
  UpdateAgentActionStatusInput
} from '../services/agentActionService.js';
import type { AssignmentRecord, CreateAssignmentInput } from '../services/assignmentService.js';
import type { AuditLogInput } from '../services/auditLogService.js';
import type {
  ChannelIngestionPolicyRecord,
  SetChannelIngestionPolicyInput
} from '../services/channelIngestionPolicyService.js';
import type { Capability } from '../services/rolePolicyService.js';

interface RolePolicyRow {
  discord_role_id: string;
}

interface ChannelIngestionPolicyRow {
  channel_id: string;
  created_at: string;
  enabled: boolean;
  guild_id: string;
  id: string;
  updated_at: string;
  updated_by_user_id: string;
}

interface AgentActionRow {
  action_type: AgentActionRecord['action_type'];
  channel_id: string | null;
  completed_at: string | null;
  created_at: string;
  error_message: string | null;
  guild_id: string | null;
  id: string;
  instructions: string;
  metadata: Record<string, unknown>;
  recipient_user_id: string | null;
  recipient_username: string | null;
  requester_user_id: string;
  requester_username: string;
  result_summary: string | null;
  status: AgentActionRecord['status'];
  title: string;
  updated_at: string;
  visibility: AgentActionRecord['visibility'];
}

export class SupabaseAssignmentStore implements AssignmentStore {
  constructor(private readonly supabase: SupabaseClient) {}

  async createAssignment(input: CreateAssignmentInput): Promise<AssignmentRecord> {
    const { data, error } = await this.supabase
      .from('assignments')
      .insert({
        guild_id: input.guildId,
        title: input.title,
        description: input.description,
        due_at: input.dueAt,
        announcement_channel_id: input.announcementChannelId,
        mentioned_role_ids: input.mentionedRoleIds,
        created_by_user_id: input.createdByUserId
      })
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to create assignment: ${error?.message ?? 'Unknown error'}`);
    }

    return data as AssignmentRecord;
  }

  async getAssignmentById(guildId: string, assignmentId: string): Promise<AssignmentRecord | null> {
    const { data, error } = await this.supabase
      .from('assignments')
      .select('*')
      .eq('guild_id', guildId)
      .eq('id', assignmentId)
      .maybeSingle();

    if (error) {
      throw new Error(`Failed to load assignment: ${error.message}`);
    }

    return (data as AssignmentRecord | null) ?? null;
  }

  async listAssignments(guildId: string): Promise<AssignmentRecord[]> {
    const { data, error } = await this.supabase
      .from('assignments')
      .select('*')
      .eq('guild_id', guildId)
      .order('created_at', { ascending: false })
      .limit(10);

    if (error) {
      throw new Error(`Failed to list assignments: ${error.message}`);
    }

    return (data ?? []) as AssignmentRecord[];
  }

  async markPublished(
    guildId: string,
    assignmentId: string,
    messageId: string,
    channelId: string
  ): Promise<AssignmentRecord> {
    const { data, error } = await this.supabase
      .from('assignments')
      .update({
        status: 'published',
        published_message_id: messageId,
        announcement_channel_id: channelId,
        updated_at: new Date().toISOString()
      })
      .eq('guild_id', guildId)
      .eq('id', assignmentId)
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to publish assignment: ${error?.message ?? 'Unknown error'}`);
    }

    return data as AssignmentRecord;
  }
}

export class SupabaseAuditLogStore implements AuditLogStore {
  constructor(private readonly supabase: SupabaseClient) {}

  async record(input: AuditLogInput): Promise<void> {
    const { error } = await this.supabase.from('audit_logs').insert({
      guild_id: input.guildId,
      actor_user_id: input.actorUserId,
      action: input.action,
      target_type: input.targetType,
      target_id: input.targetId ?? null,
      metadata: input.metadata ?? {}
    });

    if (error) {
      throw new Error(`Failed to write audit log: ${error.message}`);
    }
  }
}

export class SupabaseAgentActionStore implements AgentActionStore {
  constructor(private readonly supabase: SupabaseClient) {}

  async createAction(input: CreateAgentActionInput): Promise<AgentActionRecord> {
    const { data, error } = await this.supabase
      .from('agent_actions')
      .insert({
        guild_id: input.guildId,
        channel_id: input.channelId,
        requester_user_id: input.requesterUserId,
        requester_username: input.requesterUsername,
        recipient_user_id: input.recipientUserId,
        recipient_username: input.recipientUsername,
        action_type: input.actionType,
        visibility: input.visibility,
        title: input.title,
        instructions: input.instructions,
        metadata: input.metadata ?? {}
      })
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to create agent action: ${error?.message ?? 'Unknown error'}`);
    }

    return data as AgentActionRecord;
  }

  async listVisibleRecentForUser(userId: string, limit: number): Promise<AgentActionRecord[]> {
    const { data, error } = await this.supabase
      .from('agent_actions')
      .select('*')
      .or(`requester_user_id.eq.${userId},recipient_user_id.eq.${userId}`)
      .order('created_at', { ascending: false })
      .limit(limit);

    if (error) {
      throw new Error(`Failed to list visible agent actions: ${error.message}`);
    }

    return ((data ?? []) as AgentActionRow[]).filter((row) => isVisibleToUser(row, userId));
  }

  async updateActionStatus(input: UpdateAgentActionStatusInput): Promise<AgentActionRecord> {
    const { data, error } = await this.supabase
      .from('agent_actions')
      .update({
        status: input.status,
        result_summary: input.resultSummary ?? null,
        error_message: input.errorMessage ?? null,
        metadata: input.metadata ?? {},
        completed_at: input.completedAt ?? null,
        updated_at: new Date().toISOString()
      })
      .eq('id', input.actionId)
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to update agent action: ${error?.message ?? 'Unknown error'}`);
    }

    return data as AgentActionRecord;
  }
}

export class SupabaseChannelIngestionPolicyStore implements ChannelIngestionPolicyStore {
  constructor(private readonly supabase: SupabaseClient) {}

  async getPolicy(guildId: string, channelId: string): Promise<ChannelIngestionPolicyRecord | null> {
    const { data, error } = await this.supabase
      .from('channel_ingestion_policies')
      .select('*')
      .eq('guild_id', guildId)
      .eq('channel_id', channelId)
      .maybeSingle();

    if (error) {
      throw new Error(`Failed to load channel ingestion policy: ${error.message}`);
    }

    return (data as ChannelIngestionPolicyRow | null) ?? null;
  }

  async isChannelEnabled(guildId: string, channelId: string): Promise<boolean> {
    const row = await this.getPolicy(guildId, channelId);
    return row?.enabled === true;
  }

  async setChannelEnabled(input: SetChannelIngestionPolicyInput): Promise<ChannelIngestionPolicyRecord> {
    const { data, error } = await this.supabase
      .from('channel_ingestion_policies')
      .upsert(
        {
          guild_id: input.guildId,
          channel_id: input.channelId,
          enabled: input.enabled,
          updated_by_user_id: input.updatedByUserId,
          updated_at: new Date().toISOString()
        },
        {
          onConflict: 'guild_id,channel_id'
        }
      )
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to update channel ingestion policy: ${error?.message ?? 'Unknown error'}`);
    }

    return data as ChannelIngestionPolicyRecord;
  }
}

function isVisibleToUser(row: AgentActionRow, userId: string): boolean {
  if (row.visibility === 'requester_only') {
    return row.requester_user_id === userId;
  }

  if (row.visibility === 'participants') {
    return row.requester_user_id === userId || row.recipient_user_id === userId;
  }

  return row.requester_user_id === userId || row.recipient_user_id === userId;
}

export class SupabaseRolePolicyStore implements RolePolicyStore {
  constructor(private readonly supabase: SupabaseClient) {}

  async memberHasCapability(guild: Guild, member: GuildMember, capability: Capability): Promise<boolean> {
    await this.ensureGuild(guild);

    if (member.permissions.has(PermissionFlagsBits.Administrator)) {
      return true;
    }

    const { data, error } = await this.supabase
      .from('role_policies')
      .select('discord_role_id')
      .eq('guild_id', guild.id)
      .eq('capability', capability);

    if (error) {
      throw new Error(`Failed to load role policies: ${error.message}`);
    }

    const rows = (data ?? []) as RolePolicyRow[];
    return rows.some((row) => member.roles.cache.has(row.discord_role_id));
  }

  async ensureGuild(guild: Guild): Promise<void> {
    const { error } = await this.supabase.from('guilds').upsert(
      {
        id: guild.id,
        name: guild.name,
        updated_at: new Date().toISOString()
      },
      {
        onConflict: 'id'
      }
    );

    if (error) {
      throw new Error(`Failed to upsert guild: ${error.message}`);
    }
  }
}
