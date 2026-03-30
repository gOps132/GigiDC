import type { Client, Guild, GuildMember, User } from 'discord.js';

import type { Env } from '../config/env.js';
import type { AuditLogService } from './auditLogService.js';
import {
  CAPABILITIES,
  isCapability,
  type Capability,
  type RolePolicyService
} from './rolePolicyService.js';

interface BasePermissionInput {
  client: Client;
  guild?: Guild | null;
  requester: User;
}

interface ListPermissionInput extends BasePermissionInput {
  targetUser: User;
}

interface MutatePermissionInput extends ListPermissionInput {
  capability: string;
}

interface PermissionExecutionContext {
  guild: Guild;
  requester: User;
}

export class PermissionAdminService {
  constructor(
    private readonly env: Env,
    private readonly auditLogs: AuditLogService,
    private readonly rolePolicies: RolePolicyService
  ) {}

  async listUserPermissions(input: ListPermissionInput): Promise<string> {
    const context = await this.requirePermissionAdmin(input, {
      action: 'permission.list.permission_denied',
      targetUserId: input.targetUser.id
    });
    if (typeof context === 'string') {
      return context;
    }

    const targetMember = await context.guild.members.fetch(input.targetUser.id).catch(() => null);
    if (!targetMember) {
      return 'I can only inspect permissions for users who are still in the primary server.';
    }

    const directGrants = await this.rolePolicies.listDirectUserCapabilities(context.guild.id, targetMember.id);
    const effectiveCapabilities = await this.rolePolicies.listEffectiveCapabilities(context.guild, targetMember);

    await this.auditLogs.record({
      guildId: context.guild.id,
      actorUserId: input.requester.id,
      action: 'permission.list.executed',
      targetType: 'user_capability_grant',
      targetId: targetMember.id,
      metadata: {
        directGrantCount: directGrants.length,
        effectiveCapabilityCount: effectiveCapabilities.length,
        targetUserId: targetMember.id
      }
    });

    return [
      `Permissions for <@${targetMember.id}>:`,
      effectiveCapabilities.length > 0
        ? `Effective capabilities: ${effectiveCapabilities.map(formatCapability).join(', ')}`
        : 'Effective capabilities: none',
      directGrants.length > 0
        ? `Direct grants: ${directGrants.map(formatCapability).join(', ')}`
        : 'Direct grants: none'
    ].join('\n');
  }

  async grantUserPermission(input: MutatePermissionInput): Promise<string> {
    const capability = normalizeCapability(input.capability);
    if (!capability) {
      return unsupportedCapabilitySummary(input.capability);
    }

    const context = await this.requirePermissionAdmin(input, {
      action: 'permission.grant.permission_denied',
      targetUserId: input.targetUser.id
    });
    if (typeof context === 'string') {
      return context;
    }

    const targetMember = await context.guild.members.fetch(input.targetUser.id).catch(() => null);
    if (!targetMember) {
      return 'I can only grant permissions to users who are still in the primary server.';
    }

    const alreadyGranted = (await this.rolePolicies.listDirectUserCapabilities(context.guild.id, targetMember.id))
      .includes(capability);

    await this.rolePolicies.grantUserCapability({
      capability,
      grantedByUserId: input.requester.id,
      guildId: context.guild.id,
      userId: targetMember.id
    });

    await this.auditLogs.record({
      guildId: context.guild.id,
      actorUserId: input.requester.id,
      action: alreadyGranted ? 'permission.grant.noop' : 'permission.grant.executed',
      targetType: 'user_capability_grant',
      targetId: targetMember.id,
      metadata: {
        capability,
        targetUserId: targetMember.id
      }
    });

    return alreadyGranted
      ? `Direct grant ${formatCapability(capability)} was already present for <@${targetMember.id}>.`
      : `Granted ${formatCapability(capability)} directly to <@${targetMember.id}>.`;
  }

  async revokeUserPermission(input: MutatePermissionInput): Promise<string> {
    const capability = normalizeCapability(input.capability);
    if (!capability) {
      return unsupportedCapabilitySummary(input.capability);
    }

    const context = await this.requirePermissionAdmin(input, {
      action: 'permission.revoke.permission_denied',
      targetUserId: input.targetUser.id
    });
    if (typeof context === 'string') {
      return context;
    }

    const targetMember = await context.guild.members.fetch(input.targetUser.id).catch(() => null);
    if (!targetMember) {
      return 'I can only revoke permissions from users who are still in the primary server.';
    }

    const revoked = await this.rolePolicies.revokeUserCapability(context.guild.id, targetMember.id, capability);

    await this.auditLogs.record({
      guildId: context.guild.id,
      actorUserId: input.requester.id,
      action: revoked ? 'permission.revoke.executed' : 'permission.revoke.noop',
      targetType: 'user_capability_grant',
      targetId: targetMember.id,
      metadata: {
        capability,
        targetUserId: targetMember.id
      }
    });

    return revoked
      ? `Revoked direct grant ${formatCapability(capability)} from <@${targetMember.id}>.`
      : `Direct grant ${formatCapability(capability)} was not present for <@${targetMember.id}>.`;
  }

  private async requirePermissionAdmin(
    input: BasePermissionInput,
    metadata: {
      action: string;
      targetUserId: string;
    }
  ): Promise<PermissionExecutionContext | string> {
    const guild = input.guild ?? await this.resolvePrimaryGuild(input.client);
    if (!guild) {
      return 'I could not resolve the primary server for that action.';
    }

    const member = await guild.members.fetch(input.requester.id).catch(() => null);
    if (!member) {
      return 'I can only do that for users who are still in the primary server.';
    }

    const allowed = await this.rolePolicies.memberHasCapability(
      guild,
      member,
      CAPABILITIES.permissionAdmin
    );
    if (!allowed) {
      await this.auditLogs.record({
        guildId: guild.id,
        actorUserId: input.requester.id,
        action: metadata.action,
        targetType: 'user_capability_grant',
        targetId: metadata.targetUserId,
        metadata: {
          capability: CAPABILITIES.permissionAdmin,
          targetUserId: metadata.targetUserId
        }
      });

      return 'You do not have permission to manage Gigi direct user permissions.';
    }

    return {
      guild,
      requester: input.requester
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

function normalizeCapability(value: string): Capability | null {
  const normalized = value.trim().toLowerCase().replace(/\s+/g, '_');
  return isCapability(normalized) ? normalized : null;
}

function formatCapability(capability: Capability): string {
  return `\`${capability}\``;
}

function unsupportedCapabilitySummary(value: string): string {
  return `I do not recognize "${value}" as a supported capability.`;
}
