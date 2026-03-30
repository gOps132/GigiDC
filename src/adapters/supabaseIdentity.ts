import type { SupabaseClient } from '@supabase/supabase-js';

import type {
  UpsertUserMemorySnapshotInput,
  UpsertUserProfileInput,
  UserMemorySnapshotRecord,
  UserMemorySnapshotStore,
  UserProfileRecord,
  UserProfileStore
} from '../ports/identity.js';

export class SupabaseUserProfileStore implements UserProfileStore {
  constructor(private readonly supabase: SupabaseClient) {}

  async getProfile(guildId: string, userId: string): Promise<UserProfileRecord | null> {
    const { data, error } = await this.supabase
      .from('user_profiles')
      .select('*')
      .eq('guild_id', guildId)
      .eq('user_id', userId)
      .maybeSingle();

    if (error) {
      throw new Error(`Failed to load user profile: ${error.message}`);
    }

    return (data as UserProfileRecord | null) ?? null;
  }

  async upsertProfile(input: UpsertUserProfileInput): Promise<UserProfileRecord> {
    const existing = await this.getProfile(input.guildId, input.userId);

    const { data, error } = await this.supabase
      .from('user_profiles')
      .upsert(
        {
          guild_id: input.guildId,
          user_id: input.userId,
          username: input.username,
          global_name: input.globalName ?? null,
          display_name: input.displayName ?? null,
          first_seen_at: existing?.first_seen_at ?? input.observedAt,
          last_seen_at: input.observedAt,
          updated_at: input.observedAt
        },
        {
          onConflict: 'guild_id,user_id'
        }
      )
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to upsert user profile: ${error?.message ?? 'Unknown error'}`);
    }

    return data as UserProfileRecord;
  }
}

export class SupabaseUserMemorySnapshotStore implements UserMemorySnapshotStore {
  constructor(private readonly supabase: SupabaseClient) {}

  async listSnapshotsForUser(guildId: string, userId: string): Promise<UserMemorySnapshotRecord[]> {
    const { data, error } = await this.supabase
      .from('user_memory_snapshots')
      .select('*')
      .eq('guild_id', guildId)
      .eq('user_id', userId)
      .order('generated_at', { ascending: false });

    if (error) {
      throw new Error(`Failed to list user memory snapshots: ${error.message}`);
    }

    return (data ?? []) as UserMemorySnapshotRecord[];
  }

  async upsertSnapshot(input: UpsertUserMemorySnapshotInput): Promise<UserMemorySnapshotRecord> {
    const { data, error } = await this.supabase
      .from('user_memory_snapshots')
      .upsert(
        {
          guild_id: input.guildId,
          user_id: input.userId,
          snapshot_kind: input.snapshotKind,
          summary_text: input.summaryText,
          source_message_ids: input.sourceMessageIds,
          source_action_ids: input.sourceActionIds,
          generated_at: input.generatedAt,
          expires_at: input.expiresAt ?? null,
          updated_at: input.generatedAt
        },
        {
          onConflict: 'guild_id,user_id,snapshot_kind'
        }
      )
      .select('*')
      .single();

    if (error || !data) {
      throw new Error(`Failed to upsert user memory snapshot: ${error?.message ?? 'Unknown error'}`);
    }

    return data as UserMemorySnapshotRecord;
  }
}
