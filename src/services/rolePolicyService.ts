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
  historyGuildWide: 'history_guild_wide'
} as const;

export type Capability = (typeof CAPABILITIES)[keyof typeof CAPABILITIES];

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
}
