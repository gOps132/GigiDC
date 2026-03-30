export interface SensitiveDataRecord {
  created_at: string;
  created_by_user_id: string;
  description: string | null;
  encrypted_value: string;
  guild_id: string;
  id: string;
  label: string;
  nonce: string;
  owner_user_id: string;
  updated_at: string;
  updated_by_user_id: string;
}

export interface SensitiveDataRecordSummary {
  description: string | null;
  label: string;
  updated_at: string;
}

export interface UpsertSensitiveDataRecordInput {
  createdByUserId: string;
  description: string | null;
  encryptedValue: string;
  guildId: string;
  label: string;
  nonce: string;
  ownerUserId: string;
  updatedByUserId: string;
}

export interface SensitiveDataStore {
  deleteRecord(guildId: string, ownerUserId: string, label: string): Promise<boolean>;
  getRecord(guildId: string, ownerUserId: string, label: string): Promise<SensitiveDataRecord | null>;
  listRecordSummaries(guildId: string, ownerUserId: string): Promise<SensitiveDataRecordSummary[]>;
  upsertRecord(input: UpsertSensitiveDataRecordInput): Promise<void>;
}
