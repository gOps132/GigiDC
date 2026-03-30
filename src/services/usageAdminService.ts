import type { Client, Guild, User } from 'discord.js';

import type { Env } from '../config/env.js';
import type { AuditLogService } from './auditLogService.js';
import { CAPABILITIES, type RolePolicyService } from './rolePolicyService.js';
import type {
  ModelUsageDailySummaryRow,
  ModelUsageRequesterDailySummaryRow,
  ModelUsageService
} from './modelUsageService.js';

interface BaseUsageInput {
  client: Client;
  requester: User;
}

interface UsageSummaryInput extends BaseUsageInput {
  days: number;
}

interface UserUsageSummaryInput extends UsageSummaryInput {
  targetUser: User;
}

interface UsageExecutionContext {
  guild: Guild;
  requester: User;
}

export class UsageAdminService {
  constructor(
    private readonly env: Env,
    private readonly auditLogs: AuditLogService,
    private readonly modelUsage: ModelUsageService,
    private readonly rolePolicies: RolePolicyService
  ) {}

  async getUsageSummary(input: UsageSummaryInput): Promise<string> {
    const context = await this.requireUsageAdmin(input.client, input.requester, {
      action: 'usage.summary.permission_denied',
      targetId: null
    });
    if (typeof context === 'string') {
      return context;
    }

    const rows = await this.modelUsage.listDailySummary({
      days: input.days,
      guildId: context.guild.id
    });

    await this.auditLogs.record({
      guildId: context.guild.id,
      actorUserId: input.requester.id,
      action: 'usage.summary.executed',
      targetType: 'model_usage',
      targetId: null,
      metadata: {
        days: input.days,
        rowCount: rows.length
      }
    });

    return formatUsageSummary(rows, input.days, 'Server model usage');
  }

  async getUserUsageSummary(input: UserUsageSummaryInput): Promise<string> {
    const context = await this.requireUsageAdmin(input.client, input.requester, {
      action: 'usage.user.permission_denied',
      targetId: input.targetUser.id
    });
    if (typeof context === 'string') {
      return context;
    }

    const targetMember = await context.guild.members.fetch(input.targetUser.id).catch(() => null);
    if (!targetMember) {
      return 'I can only inspect usage for users who are still in the primary server.';
    }

    const rows = await this.modelUsage.listRequesterDailySummary({
      days: input.days,
      guildId: context.guild.id,
      requesterUserId: targetMember.id
    });

    await this.auditLogs.record({
      guildId: context.guild.id,
      actorUserId: input.requester.id,
      action: 'usage.user.executed',
      targetType: 'model_usage',
      targetId: targetMember.id,
      metadata: {
        days: input.days,
        rowCount: rows.length,
        targetUserId: targetMember.id
      }
    });

    return formatUsageSummary(rows, input.days, `Usage for <@${targetMember.id}>`);
  }

  private async requireUsageAdmin(
    client: Client,
    requester: User,
    metadata: {
      action: string;
      targetId: string | null;
    }
  ): Promise<UsageExecutionContext | string> {
    const guild = await this.resolvePrimaryGuild(client);
    if (!guild) {
      return 'I could not resolve the primary server for that action.';
    }

    const member = await guild.members.fetch(requester.id).catch(() => null);
    if (!member) {
      return 'I can only do that for users who are still in the primary server.';
    }

    const allowed = await this.rolePolicies.memberHasCapability(
      guild,
      member,
      CAPABILITIES.usageAdmin
    );
    if (!allowed) {
      await this.auditLogs.record({
        guildId: guild.id,
        actorUserId: requester.id,
        action: metadata.action,
        targetType: 'model_usage',
        targetId: metadata.targetId,
        metadata: {
          capability: CAPABILITIES.usageAdmin,
          targetId: metadata.targetId
        }
      });

      return 'You do not have permission to inspect Gigi usage summaries.';
    }

    return {
      guild,
      requester
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
}

function formatUsageSummary(
  rows: Array<ModelUsageDailySummaryRow | ModelUsageRequesterDailySummaryRow>,
  days: number,
  title: string
): string {
  if (rows.length === 0) {
    return `${title}\nNo usage events were recorded in the last ${days} day${days === 1 ? '' : 's'}.`;
  }

  const totalCost = rows.reduce((sum, row) => sum + row.estimatedCostUsd, 0);
  const totalEvents = rows.reduce((sum, row) => sum + row.eventCount, 0);
  const totalTokens = rows.reduce((sum, row) => sum + row.totalTokens, 0);

  return [
    `${title} for the last ${days} day${days === 1 ? '' : 's'}:`,
    `Estimated cost: $${totalCost.toFixed(6)}`,
    `Events: ${totalEvents}`,
    `Tokens: ${totalTokens}`,
    '',
    'Top rows:',
    ...rows.slice(0, 8).map((row) => {
      const parts = [
        formatUsageDay(row.usageDay),
        row.surface,
        row.operation,
        'model' in row ? row.model : row.provider
      ].filter(Boolean);

      return `- ${parts.join(' | ')} | ${row.totalTokens} tok | $${row.estimatedCostUsd.toFixed(6)}`;
    })
  ].join('\n');
}

function formatUsageDay(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) {
    return value;
  }

  return date.toISOString().slice(0, 10);
}
