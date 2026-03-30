import type { SupabaseClient } from '@supabase/supabase-js';

import type {
  SensitiveDataRecord,
  SensitiveDataRecordSummary,
  SensitiveDataStore,
  UpsertSensitiveDataRecordInput
} from '../ports/sensitive.js';

export class SupabaseSensitiveDataStore implements SensitiveDataStore {
  constructor(private readonly supabase: SupabaseClient) {}

  async upsertRecord(input: UpsertSensitiveDataRecordInput): Promise<void> {
    const { error } = await this.supabase
      .from('sensitive_data_records')
      .upsert(
        {
          created_by_user_id: input.createdByUserId,
          description: input.description,
          encrypted_value: input.encryptedValue,
          guild_id: input.guildId,
          label: input.label,
          nonce: input.nonce,
          owner_user_id: input.ownerUserId,
          updated_at: new Date().toISOString(),
          updated_by_user_id: input.updatedByUserId
        },
        {
          onConflict: 'guild_id,owner_user_id,label'
        }
      );

    if (error) {
      throw new Error(`Failed to upsert sensitive data record: ${error.message}`);
    }
  }

  async deleteRecord(guildId: string, ownerUserId: string, label: string): Promise<boolean> {
    const { data, error } = await this.supabase
      .from('sensitive_data_records')
      .delete()
      .eq('guild_id', guildId)
      .eq('owner_user_id', ownerUserId)
      .eq('label', label)
      .select('id');

    if (error) {
      throw new Error(`Failed to delete sensitive data record: ${error.message}`);
    }

    return (data ?? []).length > 0;
  }

  async getRecord(guildId: string, ownerUserId: string, label: string): Promise<SensitiveDataRecord | null> {
    const { data, error } = await this.supabase
      .from('sensitive_data_records')
      .select('*')
      .eq('guild_id', guildId)
      .eq('owner_user_id', ownerUserId)
      .eq('label', label)
      .maybeSingle();

    if (error) {
      throw new Error(`Failed to load sensitive data record: ${error.message}`);
    }

    return (data as SensitiveDataRecord | null) ?? null;
  }

  async listRecordSummaries(guildId: string, ownerUserId: string): Promise<SensitiveDataRecordSummary[]> {
    const { data, error } = await this.supabase
      .from('sensitive_data_records')
      .select('description,label,updated_at')
      .eq('guild_id', guildId)
      .eq('owner_user_id', ownerUserId)
      .order('label', { ascending: true });

    if (error) {
      throw new Error(`Failed to list sensitive data records: ${error.message}`);
    }

    return (data ?? []) as SensitiveDataRecordSummary[];
  }
}
