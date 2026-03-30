import test from 'node:test';
import assert from 'node:assert/strict';

import type {
  SensitiveDataRecord,
  SensitiveDataRecordSummary,
  SensitiveDataStore,
  UpsertSensitiveDataRecordInput
} from '../src/ports/sensitive.js';
import { SensitiveDataService } from '../src/services/sensitiveDataService.js';

class InMemorySensitiveDataStore implements SensitiveDataStore {
  private readonly records = new Map<string, SensitiveDataRecord>();

  async upsertRecord(input: UpsertSensitiveDataRecordInput): Promise<void> {
    const key = `${input.guildId}:${input.ownerUserId}:${input.label}`;
    const existing = this.records.get(key);

    this.records.set(key, {
      created_at: existing?.created_at ?? new Date().toISOString(),
      created_by_user_id: existing?.created_by_user_id ?? input.createdByUserId,
      description: input.description,
      encrypted_value: input.encryptedValue,
      guild_id: input.guildId,
      id: existing?.id ?? `record-${this.records.size + 1}`,
      label: input.label,
      nonce: input.nonce,
      owner_user_id: input.ownerUserId,
      updated_at: new Date().toISOString(),
      updated_by_user_id: input.updatedByUserId
    });
  }

  async deleteRecord(guildId: string, ownerUserId: string, label: string): Promise<boolean> {
    return this.records.delete(`${guildId}:${ownerUserId}:${label}`);
  }

  async getRecord(guildId: string, ownerUserId: string, label: string): Promise<SensitiveDataRecord | null> {
    return this.records.get(`${guildId}:${ownerUserId}:${label}`) ?? null;
  }

  async listRecordSummaries(guildId: string, ownerUserId: string): Promise<SensitiveDataRecordSummary[]> {
    return [...this.records.values()]
      .filter((record) => record.guild_id === guildId && record.owner_user_id === ownerUserId)
      .map((record) => ({
        description: record.description,
        label: record.label,
        updated_at: record.updated_at
      }));
  }
}

function createService() {
  const store = new InMemorySensitiveDataStore();
  const service = new SensitiveDataService(
    {
      DISCORD_GUILD_ID: 'guild-1',
      PRIMARY_GUILD_ID: 'guild-1',
      SENSITIVE_DATA_ENCRYPTION_KEY: Buffer.alloc(32, 7).toString('base64')
    } as never,
    store,
    {
      debug() {},
      error() {},
      info() {},
      warn() {}
    } as never
  );

  const client = {
    guilds: {
      cache: new Map([['guild-1', {
        id: 'guild-1',
        members: {
          async fetch() {
            return {
              id: 'user-1'
            };
          }
        }
      }]]),
      async fetch() {
        return {
          id: 'guild-1',
          members: {
            async fetch() {
              return {
                id: 'user-1'
              };
            }
          }
        };
      }
    }
  };

  return {
    client,
    service,
    store
  };
}

test('SensitiveDataService stores and retrieves a sensitive record without OpenAI', async () => {
  const { client, service } = createService();
  await service.putRecord({
    createdByUserId: 'admin-1',
    description: 'GitHub personal access token',
    guildId: 'guild-1',
    label: 'github',
    ownerUserId: 'user-1',
    value: 'ghp_example_secret'
  });

  const reply = await service.maybeHandleDmQuery(
    'what is my github token',
    {
      id: 'user-1',
      username: 'erick'
    } as never,
    client as never
  );

  assert.ok(reply);
  assert.equal(reply.persistReply, false);
  assert.match(reply.reply, /Sensitive record "github" for you/i);
  assert.match(reply.reply, /ghp_example_secret/);
});

test('SensitiveDataService lists only labels and descriptions for list requests', async () => {
  const { client, service } = createService();
  await service.putRecord({
    createdByUserId: 'admin-1',
    description: 'GitHub personal access token',
    guildId: 'guild-1',
    label: 'github',
    ownerUserId: 'user-1',
    value: 'ghp_example_secret'
  });

  const reply = await service.maybeHandleDmQuery(
    'show my sensitive data',
    {
      id: 'user-1',
      username: 'erick'
    } as never,
    client as never
  );

  assert.ok(reply);
  assert.match(reply.reply, /Sensitive records available for you/i);
  assert.doesNotMatch(reply.reply, /ghp_example_secret/);
});

test('SensitiveDataService flags raw sensitive write attempts for history bypass', () => {
  const { service } = createService();
  assert.equal(service.shouldBypassHistoryStorage('remember my github token is ghp_example_secret'), true);
  assert.equal(service.shouldBypassHistoryStorage('what is my github token'), false);
});
