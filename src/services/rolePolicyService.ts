import {
  PermissionFlagsBits,
  type Guild,
  type GuildMember
} from 'discord.js';
import type { SupabaseClient } from '@supabase/supabase-js';

export const CAPABILITIES = {
  assignmentAdmin: 'assignment_admin',
  clawbotDispatch: 'clawbot_dispatch',
  ingestionAdmin: 'ingestion_admin'
} as const;

export type Capability = (typeof CAPABILITIES)[keyof typeof CAPABILITIES];

interface RolePolicyRow {
  discord_role_id: string;
}

export class RolePolicyService {
  constructor(private readonly supabase: SupabaseClient) {}

  async memberHasCapability(
    guild: Guild,
    member: GuildMember,
    capability: Capability
  ): Promise<boolean> {
    if (member.permissions.has(PermissionFlagsBits.Administrator)) {
      return true;
    }

    await this.ensureGuild(guild);

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
