import type { SupabaseClient } from '@supabase/supabase-js';

export interface ChannelIngestionPolicyRecord {
  guild_id: string;
  channel_id: string;
  enabled: boolean;
  updated_by_user_id: string;
  updated_at: string;
}

export class ChannelIngestionPolicyService {
  constructor(private readonly supabase: SupabaseClient) {}

  async isEnabled(guildId: string, channelId: string): Promise<boolean> {
    const { data, error } = await this.supabase
      .from('channel_ingestion_policies')
      .select('enabled')
      .eq('guild_id', guildId)
      .eq('channel_id', channelId)
      .maybeSingle();

    if (error) {
      throw new Error(`Failed to load ingestion policy: ${error.message}`);
    }

    return Boolean(data?.enabled);
  }

  async setPolicy(
    guildId: string,
    channelId: string,
    enabled: boolean,
    updatedByUserId: string
  ): Promise<ChannelIngestionPolicyRecord> {
    const { data, error } = await this.supabase
      .from('channel_ingestion_policies')
      .upsert(
        {
          guild_id: guildId,
          channel_id: channelId,
          enabled,
          updated_by_user_id: updatedByUserId,
          updated_at: new Date().toISOString()
        },
        {
          onConflict: 'guild_id,channel_id'
        }
      )
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to update ingestion policy: ${error?.message ?? 'Unknown error'}`);
    }

    return data as ChannelIngestionPolicyRecord;
  }
}
