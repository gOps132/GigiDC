import type { ChannelIngestionPolicyStore } from '../ports/controlPlane.js';

const CACHE_TTL_MS = 30 * 1000;

interface PolicyCacheEntry {
  enabled: boolean;
  expiresAt: number;
}

export interface ChannelIngestionPolicyRecord {
  channel_id: string;
  created_at: string;
  enabled: boolean;
  guild_id: string;
  id: string;
  updated_at: string;
  updated_by_user_id: string;
}

export interface SetChannelIngestionPolicyInput {
  channelId: string;
  enabled: boolean;
  guildId: string;
  updatedByUserId: string;
}

export class ChannelIngestionPolicyService {
  private readonly cache = new Map<string, PolicyCacheEntry>();

  constructor(private readonly store: ChannelIngestionPolicyStore) {}

  async isChannelEnabled(guildId: string, channelId: string): Promise<boolean> {
    const cacheKey = `${guildId}:${channelId}`;
    const cached = this.cache.get(cacheKey);
    if (cached && cached.expiresAt > Date.now()) {
      return cached.enabled;
    }

    const enabled = await this.store.isChannelEnabled(guildId, channelId);

    this.cache.set(cacheKey, {
      enabled,
      expiresAt: Date.now() + CACHE_TTL_MS
    });

    return enabled;
  }

  async getPolicy(guildId: string, channelId: string): Promise<ChannelIngestionPolicyRecord | null> {
    return this.store.getPolicy(guildId, channelId);
  }

  async setChannelEnabled(input: SetChannelIngestionPolicyInput): Promise<ChannelIngestionPolicyRecord> {
    const record = await this.store.setChannelEnabled(input);

    this.cache.set(`${input.guildId}:${input.channelId}`, {
      enabled: record.enabled,
      expiresAt: Date.now() + CACHE_TTL_MS
    });

    return record;
  }

  invalidate(guildId: string, channelId: string): void {
    this.cache.delete(`${guildId}:${channelId}`);
  }
}
