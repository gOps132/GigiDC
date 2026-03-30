export const USER_MEMORY_SNAPSHOT_KINDS = {
  identitySummary: 'identity_summary',
  preferences: 'preferences',
  workingContext: 'working_context'
} as const;

export type UserMemorySnapshotKind =
  (typeof USER_MEMORY_SNAPSHOT_KINDS)[keyof typeof USER_MEMORY_SNAPSHOT_KINDS];

export interface UserProfileRecord {
  created_at: string;
  display_name: string | null;
  first_seen_at: string;
  global_name: string | null;
  guild_id: string;
  last_seen_at: string;
  updated_at: string;
  user_id: string;
  username: string;
}

export interface UserMemorySnapshotRecord {
  created_at: string;
  expires_at: string | null;
  generated_at: string;
  guild_id: string;
  id: string;
  snapshot_kind: UserMemorySnapshotKind;
  source_action_ids: string[];
  source_message_ids: string[];
  summary_text: string;
  updated_at: string;
  user_id: string;
}

export interface UpsertUserProfileInput {
  displayName?: string | null;
  globalName?: string | null;
  guildId: string;
  observedAt: string;
  userId: string;
  username: string;
}

export interface UpsertUserMemorySnapshotInput {
  expiresAt?: string | null;
  generatedAt: string;
  guildId: string;
  snapshotKind: UserMemorySnapshotKind;
  sourceActionIds: string[];
  sourceMessageIds: string[];
  summaryText: string;
  userId: string;
}

export interface UserProfileStore {
  getProfile(guildId: string, userId: string): Promise<UserProfileRecord | null>;
  upsertProfile(input: UpsertUserProfileInput): Promise<UserProfileRecord>;
}

export interface UserMemorySnapshotStore {
  listSnapshotsForUser(guildId: string, userId: string): Promise<UserMemorySnapshotRecord[]>;
  upsertSnapshot(input: UpsertUserMemorySnapshotInput): Promise<UserMemorySnapshotRecord>;
}
