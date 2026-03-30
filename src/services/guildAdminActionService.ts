import {
  ChannelType,
  type Client,
  type Guild,
  type GuildBasedChannel,
  type GuildMember,
  type Role,
  type User
} from 'discord.js';

import type { Env } from '../config/env.js';
import type { Logger } from '../lib/logger.js';
import {
  buildAssignmentAnnouncementIntro,
  buildAssignmentEmbed
} from '../discord/assignmentPresentation.js';
import type { AssignmentService, AssignmentRecord } from './assignmentService.js';
import type { AuditLogService } from './auditLogService.js';
import type { ChannelIngestionPolicyService } from './channelIngestionPolicyService.js';
import { CAPABILITIES, type Capability, type RolePolicyService } from './rolePolicyService.js';

type AllowedChannelType = ChannelType;
type SendableGuildChannel = GuildBasedChannel & {
  send: (options: {
    content?: string;
    embeds?: unknown[];
  }) => Promise<{
    id: string;
  }>;
};

interface GuildExecutionContext {
  guild: Guild;
  member: GuildMember;
  requester: User;
  requesterLabel: string;
}

interface BaseGuildAdminInput {
  client: Client;
  requester: User;
}

interface IngestionInput extends BaseGuildAdminInput {
  channelReference: string | null;
}

interface SetIngestionInput extends IngestionInput {
  enabled: boolean;
}

interface CreateAssignmentInput extends BaseGuildAdminInput {
  affectedRoleReferences: string[];
  channelReference: string | null;
  description: string;
  dueAt: string | null;
  title: string;
}

interface PublishAssignmentInput extends BaseGuildAdminInput {
  assignmentReference: string;
  channelReference: string | null;
}

export class GuildAdminActionService {
  constructor(
    private readonly env: Env,
    private readonly assignments: AssignmentService,
    private readonly auditLogs: AuditLogService,
    private readonly channelIngestionPolicies: ChannelIngestionPolicyService,
    private readonly rolePolicies: RolePolicyService,
    private readonly logger: Logger
  ) {}

  async getIngestionStatus(input: IngestionInput): Promise<string> {
    const context = await this.requireCapability(input.client, input.requester, CAPABILITIES.ingestionAdmin, {
      action: 'dm.tools.ingestion.status.permission_denied',
      targetType: 'channel_ingestion_policy'
    });
    if (typeof context === 'string') {
      return context;
    }

    const channel = await this.resolveChannelReference(
      context.guild,
      input.channelReference,
      allowedIngestionChannelTypes()
    );
    if (typeof channel === 'string') {
      return channel;
    }

    const policy = await this.channelIngestionPolicies.getPolicy(context.guild.id, channel.id);
    await this.recordAudit(context, 'dm.tools.ingestion.status.executed', channel.id, 'channel_ingestion_policy', {
      channelId: channel.id,
      channelName: channel.name
    });

    return [
      `Ingestion for ${formatGuildChannel(channel)} is ${policy?.enabled ? 'enabled' : 'disabled'}.`,
      policy?.updated_by_user_id ? `Last updated by <@${policy.updated_by_user_id}>.` : null,
      policy?.updated_at ? `Last updated at ${new Date(policy.updated_at).toISOString()}.` : null
    ]
      .filter((line): line is string => Boolean(line))
      .join(' ');
  }

  async setIngestionPolicy(input: SetIngestionInput): Promise<string> {
    const context = await this.requireCapability(input.client, input.requester, CAPABILITIES.ingestionAdmin, {
      action: 'dm.tools.ingestion.permission_denied',
      targetType: 'channel_ingestion_policy'
    });
    if (typeof context === 'string') {
      return context;
    }

    const channel = await this.resolveChannelReference(
      context.guild,
      input.channelReference,
      allowedIngestionChannelTypes()
    );
    if (typeof channel === 'string') {
      return channel;
    }

    const previous = await this.channelIngestionPolicies.getPolicy(context.guild.id, channel.id);
    const updated = await this.channelIngestionPolicies.setChannelEnabled({
      channelId: channel.id,
      enabled: input.enabled,
      guildId: context.guild.id,
      updatedByUserId: context.requester.id
    });
    const changed = previous?.enabled !== updated.enabled;

    await this.recordAudit(
      context,
      changed
        ? updated.enabled ? 'dm.tools.ingestion.enabled' : 'dm.tools.ingestion.disabled'
        : updated.enabled ? 'dm.tools.ingestion.enable_noop' : 'dm.tools.ingestion.disable_noop',
      channel.id,
      'channel_ingestion_policy',
      {
        channelId: channel.id,
        channelName: channel.name,
        previousEnabled: previous?.enabled ?? false,
        updatedEnabled: updated.enabled
      }
    );

    if (changed) {
      return `${updated.enabled ? 'Enabled' : 'Disabled'} ingestion for ${formatGuildChannel(channel)}.`;
    }

    return `Ingestion was already ${updated.enabled ? 'enabled' : 'disabled'} for ${formatGuildChannel(channel)}.`;
  }

  async listAssignments(input: BaseGuildAdminInput): Promise<string> {
    const context = await this.requireCapability(input.client, input.requester, CAPABILITIES.assignmentAdmin, {
      action: 'dm.tools.assignment.list.permission_denied',
      targetType: 'assignment'
    });
    if (typeof context === 'string') {
      return context;
    }

    const assignments = await this.assignments.listAssignments(context.guild.id);
    await this.recordAudit(context, 'dm.tools.assignment.list.executed', null, 'assignment', {
      assignmentCount: assignments.length
    });

    if (assignments.length === 0) {
      return 'No assignments have been created yet.';
    }

    return [
      'Recent assignments:',
      ...assignments.map((assignment) => formatAssignmentSummary(assignment))
    ].join('\n');
  }

  async createAssignment(input: CreateAssignmentInput): Promise<string> {
    const context = await this.requireCapability(input.client, input.requester, CAPABILITIES.assignmentAdmin, {
      action: 'dm.tools.assignment.create.permission_denied',
      targetType: 'assignment'
    });
    if (typeof context === 'string') {
      return context;
    }

    const dueAt = normalizeDueAt(input.dueAt);
    if (input.dueAt && !dueAt) {
      return 'I could not create that assignment because the due date was not a full ISO-8601 timestamp.';
    }

    const channel = input.channelReference
      ? await this.resolveChannelReference(context.guild, input.channelReference, allowedAssignmentChannelTypes())
      : null;
    if (typeof channel === 'string') {
      return channel;
    }

    const roles = await this.resolveRoleReferences(context.guild, input.affectedRoleReferences);
    if (typeof roles === 'string') {
      return roles;
    }

    const assignment = await this.assignments.createAssignment({
      guildId: context.guild.id,
      title: input.title.trim(),
      description: input.description.trim(),
      dueAt: dueAt?.toISOString() ?? null,
      announcementChannelId: channel?.id ?? null,
      mentionedRoleIds: roles.map((role) => role.id),
      createdByUserId: context.requester.id
    });

    await this.recordAudit(context, 'dm.tools.assignment.create.created', assignment.id, 'assignment', {
      announcementChannelId: assignment.announcement_channel_id,
      mentionedRoleIds: assignment.mentioned_role_ids
    });

    return [
      `Created assignment \`${assignment.id}\`: ${assignment.title}.`,
      assignment.announcement_channel_id ? `Announcement channel: <#${assignment.announcement_channel_id}>.` : null,
      assignment.due_at ? `Due ${new Date(assignment.due_at).toISOString()}.` : null
    ]
      .filter((line): line is string => Boolean(line))
      .join(' ');
  }

  async publishAssignment(input: PublishAssignmentInput): Promise<string> {
    const context = await this.requireCapability(input.client, input.requester, CAPABILITIES.assignmentAdmin, {
      action: 'dm.tools.assignment.publish.permission_denied',
      targetType: 'assignment'
    });
    if (typeof context === 'string') {
      return context;
    }

    const assignment = await this.resolveAssignmentReference(context.guild.id, input.assignmentReference);
    if (!assignment) {
      return `I could not find an assignment matching "${input.assignmentReference}".`;
    }

    const explicitChannel = input.channelReference
      ? await this.resolveChannelReference(context.guild, input.channelReference, allowedAssignmentChannelTypes())
      : null;
    if (typeof explicitChannel === 'string') {
      return explicitChannel;
    }

    const targetChannel = explicitChannel ?? await this.resolveStoredAnnouncementChannel(context.guild, assignment);
    if (!targetChannel) {
      return 'I could not publish that assignment because no valid announcement channel was available. Mention a text channel explicitly or set one on the assignment first.';
    }

    if (!('send' in targetChannel)) {
      return 'The resolved announcement channel cannot send messages.';
    }

    const sentMessage = await targetChannel.send({
      content: buildAssignmentAnnouncementIntro(assignment, context.requesterLabel),
      embeds: [buildAssignmentEmbed(assignment, 'New assignment')]
    });

    const published = await this.assignments.markPublished(
      context.guild.id,
      assignment.id,
      sentMessage.id,
      targetChannel.id
    );

    await this.recordAudit(context, 'dm.tools.assignment.publish.published', published.id, 'assignment', {
      channelId: targetChannel.id,
      messageId: sentMessage.id
    });

    return `Published assignment \`${published.id}\` to <#${targetChannel.id}>.`;
  }

  private async requireCapability(
    client: Client,
    requester: User,
    capability: Capability,
    audit: {
      action: string;
      targetType: 'assignment' | 'channel_ingestion_policy';
    }
  ): Promise<GuildExecutionContext | string> {
    const guild = await this.resolvePrimaryGuild(client);
    if (!guild) {
      return 'I could not resolve the primary server for that action.';
    }

    const member = await guild.members.fetch(requester.id).catch(() => null);
    if (!member) {
      return 'I can only do that for users who are still in the primary server.';
    }

    const allowed = await this.rolePolicies.memberHasCapability(guild, member, capability);
    if (!allowed) {
      await this.auditLogs.record({
        guildId: guild.id,
        actorUserId: requester.id,
        action: audit.action,
        targetType: audit.targetType,
        targetId: null,
        metadata: {
          capability
        }
      });

      return capability === CAPABILITIES.ingestionAdmin
        ? 'You do not have permission to manage ingestion policies.'
        : 'You do not have permission to manage assignments.';
    }

    return {
      guild,
      member,
      requester,
      requesterLabel: member.displayName ?? requester.username
    };
  }

  private async resolvePrimaryGuild(client: Client): Promise<Guild | null> {
    const primaryGuildId = this.env.PRIMARY_GUILD_ID ?? this.env.DISCORD_GUILD_ID;
    if (!primaryGuildId) {
      return null;
    }

    return client.guilds.cache.get(primaryGuildId)
      ?? (await client.guilds.fetch(primaryGuildId).catch(() => null));
  }

  private async resolveChannelReference(
    guild: Guild,
    reference: string | null,
    allowedTypes: AllowedChannelType[]
  ): Promise<GuildBasedChannel | string> {
    const normalized = normalizeReference(reference);
    if (!normalized) {
      return 'Mention the target channel explicitly or use its exact name.';
    }

    const channels = await guild.channels.fetch().catch(() => null);
    if (!channels) {
      return 'I could not load channels from the primary server.';
    }

    const mentionedId = parseChannelId(normalized);
    if (mentionedId) {
      const byMention = channels.get(mentionedId);
      if (byMention && allowedTypes.includes(byMention.type)) {
        return byMention;
      }
      return 'That channel is not a supported text channel or thread for this action.';
    }

    const byId = channels.get(normalized);
    if (byId && allowedTypes.includes(byId.type)) {
      return byId;
    }

    const matches = [...channels.values()].flatMap((channel) => {
      if (
        !channel
        || !allowedTypes.includes(channel.type)
        || normalizeReference(channel.name) !== normalized
      ) {
        return [];
      }

      return [channel];
    });

    if (matches.length === 1) {
      return matches[0]!;
    }

    if (matches.length > 1) {
      return `I found multiple channels named "${reference}". Mention the channel explicitly instead.`;
    }

    return `I could not resolve "${reference}" as a supported channel in the primary server.`;
  }

  private async resolveRoleReferences(guild: Guild, references: string[]): Promise<Role[] | string> {
    if (references.length === 0) {
      return [];
    }

    const roles = await guild.roles.fetch().catch(() => null);
    if (!roles) {
      return 'I could not load roles from the primary server.';
    }

    const resolved: Role[] = [];
    for (const reference of references) {
      const normalized = normalizeReference(reference);
      if (!normalized) {
        continue;
      }

      const mentionedId = parseRoleId(normalized);
      const byMention = mentionedId ? roles.get(mentionedId) ?? null : null;
      const byId = roles.get(normalized) ?? null;
      const exactMatches = [...roles.values()].filter((role) =>
        normalizeReference(role.name) === normalized
      );

      const role = byMention ?? byId ?? (exactMatches.length === 1 ? exactMatches[0] : null);
      if (!role) {
        return exactMatches.length > 1
          ? `I found multiple roles named "${reference}". Mention the role explicitly instead.`
          : `I could not resolve "${reference}" as a role in the primary server.`;
      }

      if (!resolved.some((entry) => entry.id === role.id)) {
        resolved.push(role);
      }
    }

    return resolved;
  }

  private async resolveAssignmentReference(
    guildId: string,
    reference: string
  ): Promise<AssignmentRecord | null> {
    const trimmed = reference.trim();
    const byId = await this.assignments.getAssignmentById(guildId, trimmed);
    if (byId) {
      return byId;
    }

    const assignments = await this.assignments.listAssignments(guildId);
    const normalized = normalizeReference(trimmed);
    if (!normalized) {
      return null;
    }

    const exact = assignments.find((assignment) => normalizeReference(assignment.title) === normalized);
    if (exact) {
      return exact;
    }

    const partialMatches = assignments.filter((assignment) =>
      normalizeReference(assignment.title)?.includes(normalized)
    );

    return partialMatches.length === 1 ? partialMatches[0] ?? null : null;
  }

  private async resolveStoredAnnouncementChannel(
    guild: Guild,
    assignment: AssignmentRecord
  ): Promise<SendableGuildChannel | null> {
    if (!assignment.announcement_channel_id) {
      return null;
    }

    const channel = await guild.channels.fetch(assignment.announcement_channel_id).catch(() => null);
    return isSendableGuildChannel(channel) ? channel : null;
  }

  private async recordAudit(
    context: GuildExecutionContext,
    action: string,
    targetId: string | null,
    targetType: 'assignment' | 'channel_ingestion_policy',
    metadata?: Record<string, unknown>
  ): Promise<void> {
    await this.auditLogs.record({
      guildId: context.guild.id,
      actorUserId: context.requester.id,
      action,
      targetId,
      targetType,
      metadata
    });
  }
}

function normalizeDueAt(value: string | null): Date | null {
  if (!value) {
    return null;
  }

  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
}

function normalizeReference(value: string | null | undefined): string | null {
  const normalized = value?.trim().replace(/^[@#]/, '').toLowerCase() ?? '';
  return normalized.length > 0 ? normalized : null;
}

function parseChannelId(value: string): string | null {
  const match = value.match(/^<#(?<id>\d+)>$/);
  return match?.groups?.id ?? null;
}

function parseRoleId(value: string): string | null {
  const match = value.match(/^<@&(?<id>\d+)>$/);
  return match?.groups?.id ?? null;
}

function allowedIngestionChannelTypes(): AllowedChannelType[] {
  return [
    ChannelType.GuildText,
    ChannelType.GuildAnnouncement,
    ChannelType.PublicThread,
    ChannelType.PrivateThread,
    ChannelType.AnnouncementThread
  ];
}

function allowedAssignmentChannelTypes(): AllowedChannelType[] {
  return [
    ChannelType.GuildText,
    ChannelType.GuildAnnouncement
  ];
}

function formatGuildChannel(channel: GuildBasedChannel): string {
  return `<#${channel.id}>`;
}

function isSendableGuildChannel(channel: GuildBasedChannel | null): channel is SendableGuildChannel {
  return channel !== null && 'send' in channel;
}

function formatAssignmentSummary(assignment: AssignmentRecord): string {
  const dueText = assignment.due_at ? ` due ${new Date(assignment.due_at).toISOString()}` : ' no due date';
  return `- \`${assignment.id}\` ${assignment.title} [${assignment.status}]${dueText}`;
}
