import {
  type Guild,
  type GuildMember
} from 'discord.js';
import type { RolePolicyStore } from '../ports/controlPlane.js';

export const CAPABILITIES = {
  agentActionDispatch: 'agent_action_dispatch',
  agentActionReceive: 'agent_action_receive',
  assignmentAdmin: 'assignment_admin',
  ingestionAdmin: 'ingestion_admin',
  historyGuildWide: 'history_guild_wide',
  permissionAdmin: 'permission_admin',
  usageAdmin: 'usage_admin'
} as const;

export type Capability = (typeof CAPABILITIES)[keyof typeof CAPABILITIES];
export const ALL_CAPABILITIES = Object.values(CAPABILITIES) as Capability[];

export interface GrantUserCapabilityInput {
  capability: Capability;
  grantedByUserId: string;
  guildId: string;
  userId: string;
}

export class RolePolicyService {
  constructor(private readonly store: RolePolicyStore) {}

  async memberHasCapability(
    guild: Guild,
    member: GuildMember,
    capability: Capability
  ): Promise<boolean> {
    return this.store.memberHasCapability(guild, member, capability);
  }

  async ensureGuild(guild: Guild): Promise<void> {
    await this.store.ensureGuild(guild);
  }

  async grantUserCapability(input: GrantUserCapabilityInput): Promise<void> {
    await this.store.grantUserCapability(input);
  }

  async revokeUserCapability(guildId: string, userId: string, capability: Capability): Promise<boolean> {
    return this.store.revokeUserCapability(guildId, userId, capability);
  }

  async listDirectUserCapabilities(guildId: string, userId: string): Promise<Capability[]> {
    return this.store.listDirectUserCapabilities(guildId, userId);
  }

  async listEffectiveCapabilities(guild: Guild, member: GuildMember): Promise<Capability[]> {
    const capabilities: Capability[] = [];

    for (const capability of ALL_CAPABILITIES) {
      if (await this.memberHasCapability(guild, member, capability)) {
        capabilities.push(capability);
      }
    }

    return capabilities;
  }
}

export function isCapability(value: string): value is Capability {
  return ALL_CAPABILITIES.includes(value as Capability);
}
