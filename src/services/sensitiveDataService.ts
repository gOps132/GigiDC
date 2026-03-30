import type { Client, Guild, User } from 'discord.js';

import type { Env } from '../config/env.js';
import {
  decryptSensitiveValue,
  encryptSensitiveValue,
  parseSensitiveDataKey
} from '../lib/sensitiveDataCrypto.js';
import type { Logger } from '../lib/logger.js';
import type { SensitiveDataStore } from '../ports/sensitive.js';

export interface SensitiveDmReply {
  persistReply: boolean;
  reply: string;
}

export class SensitiveDataService {
  private readonly encryptionKey: Buffer | null;

  constructor(
    private readonly env: Env,
    private readonly store: SensitiveDataStore,
    private readonly logger: Logger
  ) {
    this.encryptionKey = parseSensitiveDataKey(this.env.SENSITIVE_DATA_ENCRYPTION_KEY);
  }

  shouldBypassHistoryStorage(query: string): boolean {
    return parseSensitiveWriteAttempt(query) !== null;
  }

  async maybeHandleDmQuery(
    query: string,
    requester: User,
    client: Client
  ): Promise<SensitiveDmReply | null> {
    const writeAttempt = parseSensitiveWriteAttempt(query);
    if (writeAttempt) {
      return {
        persistReply: false,
        reply: [
          'Do not send raw sensitive values through normal DM chat.',
          'Use the local sensitive-data admin script to store them instead, so Gigi can retrieve them later without sending the value through Discord or OpenAI.'
        ].join(' ')
      };
    }

    const intent = parseSensitiveQuery(query);
    if (!intent) {
      return null;
    }

    const guild = await this.resolvePrimaryGuild(client);
    if (!guild) {
      return {
        persistReply: false,
        reply: 'Sensitive-data retrieval is not configured because the primary server could not be resolved.'
      };
    }

    const member = await guild.members.fetch(requester.id).catch(() => null);
    if (!member) {
      return {
        persistReply: false,
        reply: 'I can only retrieve sensitive data for users who are still in the primary server.'
      };
    }

    if (!this.encryptionKey) {
      return {
        persistReply: false,
        reply: 'Sensitive-data retrieval is not configured yet.'
      };
    }

    if (intent.kind === 'list') {
      const summaries = await this.store.listRecordSummaries(guild.id, requester.id);
      if (summaries.length === 0) {
        return {
          persistReply: false,
          reply: 'I do not have any sensitive records stored for you.'
        };
      }

      return {
        persistReply: false,
        reply: [
          'Sensitive records available for you:',
          ...summaries.map((summary) =>
            summary.description
              ? `- ${summary.label}: ${summary.description}`
              : `- ${summary.label}`
          )
        ].join('\n')
      };
    }

    const normalizedLabel = normalizeLabel(intent.label);
    const record = await this.store.getRecord(guild.id, requester.id, normalizedLabel);
    if (!record) {
      return {
        persistReply: false,
        reply: `I do not have a sensitive record stored for you under "${normalizedLabel}".`
      };
    }

    try {
      const value = decryptSensitiveValue(record.encrypted_value, record.nonce, this.encryptionKey);
      return {
        persistReply: false,
        reply: [
          `Sensitive record "${record.label}" for you:`,
          '```text',
          value,
          '```'
        ].join('\n')
      };
    } catch (error) {
      this.logger.error('Failed to decrypt sensitive data record', {
        label: record.label,
        ownerUserId: requester.id,
        recordId: record.id,
        error: error instanceof Error ? error.message : 'Unknown sensitive-data decrypt error'
      });

      return {
        persistReply: false,
        reply: `I found "${record.label}", but I could not decrypt it safely.`
      };
    }
  }

  async putRecord(input: {
    createdByUserId: string;
    description: string | null;
    guildId: string;
    label: string;
    ownerUserId: string;
    value: string;
  }): Promise<void> {
    if (!this.encryptionKey) {
      throw new Error('SENSITIVE_DATA_ENCRYPTION_KEY is required to store sensitive data.');
    }

    const encrypted = encryptSensitiveValue(input.value, this.encryptionKey);
    await this.store.upsertRecord({
      createdByUserId: input.createdByUserId,
      description: input.description,
      encryptedValue: encrypted.ciphertext,
      guildId: input.guildId,
      label: normalizeLabel(input.label),
      nonce: encrypted.nonce,
      ownerUserId: input.ownerUserId,
      updatedByUserId: input.createdByUserId
    });
  }

  async deleteRecord(guildId: string, ownerUserId: string, label: string): Promise<boolean> {
    return this.store.deleteRecord(guildId, ownerUserId, normalizeLabel(label));
  }

  async listRecordSummaries(guildId: string, ownerUserId: string) {
    return this.store.listRecordSummaries(guildId, ownerUserId);
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

function parseSensitiveQuery(query: string): { kind: 'get'; label: string } | { kind: 'list' } | null {
  const trimmed = query.trim();

  if (
    /^(show|list|what(?:'s| is))\s+(?:my\s+)?(?:sensitive data|secrets?)[.!?]*$/i.test(trimmed)
    || /^what\s+(?:sensitive data|secrets?)\s+do you have\s+for me[.!?]*$/i.test(trimmed)
  ) {
    return {
      kind: 'list'
    };
  }

  const patterns = [
    /^(?:show|reveal|get|what(?:'s| is))\s+my\s+(?<label>.+?)\s+(?:password|token|secret|credential|api key|key)[.!?]*$/i,
    /^(?:show|reveal|get|what(?:'s| is))\s+my\s+(?:password|token|secret|credential|api key|key)\s+(?:for|of)\s+(?<label>.+?)[.!?]*$/i,
    /^(?:show|reveal|get|what(?:'s| is))\s+(?:the\s+)?(?:sensitive data|secret)\s+(?:for|of)\s+(?<label>.+?)[.!?]*$/i
  ];

  for (const pattern of patterns) {
    const label = pattern.exec(trimmed)?.groups?.label?.trim();
    if (label) {
      return {
        kind: 'get',
        label
      };
    }
  }

  return null;
}

function parseSensitiveWriteAttempt(query: string): { label: string } | null {
  const trimmed = query.trim();
  const match = trimmed.match(
    /^(?:remember|store|save)\s+(?:my\s+)?(?<label>.+?)\s+(?:password|token|secret|credential|api key|key)\s*(?:is|=)\s*.+$/i
  );

  if (!match?.groups?.label) {
    return null;
  }

  return {
    label: match.groups.label
  };
}

function normalizeLabel(value: string): string {
  return value.trim().toLowerCase().replace(/\s+/g, '_');
}
