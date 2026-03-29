import test from 'node:test';
import assert from 'node:assert/strict';

import { ChannelIngestionPolicyService } from '../src/services/channelIngestionPolicyService.js';

function createStoreStub(
  resolver: (guildId: string, channelId: string) => boolean
) {
  let queryCount = 0;
  let setCount = 0;

  const store = {
    async getPolicy(guildId: string, channelId: string) {
      return {
        channel_id: channelId,
        created_at: new Date().toISOString(),
        enabled: resolver(guildId, channelId),
        guild_id: guildId,
        id: `${guildId}:${channelId}`,
        updated_at: new Date().toISOString(),
        updated_by_user_id: 'user-1'
      };
    },
    async isChannelEnabled(guildId: string, channelId: string) {
      queryCount += 1;
      return resolver(guildId, channelId);
    },
    async setChannelEnabled(input: {
      channelId: string;
      enabled: boolean;
      guildId: string;
      updatedByUserId: string;
    }) {
      setCount += 1;
      return {
        channel_id: input.channelId,
        created_at: new Date().toISOString(),
        enabled: input.enabled,
        guild_id: input.guildId,
        id: `${input.guildId}:${input.channelId}`,
        updated_at: new Date().toISOString(),
        updated_by_user_id: input.updatedByUserId
      };
    }
  };

  return {
    getSetCount: () => setCount,
    store,
    getQueryCount: () => queryCount
  };
}

test('ChannelIngestionPolicyService caches enabled results briefly', async () => {
  const stub = createStoreStub(() => true);

  const service = new ChannelIngestionPolicyService(stub.store);

  assert.equal(await service.isChannelEnabled('guild-1', 'channel-1'), true);
  assert.equal(await service.isChannelEnabled('guild-1', 'channel-1'), true);
  assert.equal(stub.getQueryCount(), 1);
});

test('ChannelIngestionPolicyService defaults to disabled when no row exists', async () => {
  const stub = createStoreStub(() => false);

  const service = new ChannelIngestionPolicyService(stub.store);

  assert.equal(await service.isChannelEnabled('guild-1', 'channel-2'), false);
  assert.equal(stub.getQueryCount(), 1);
});

test('ChannelIngestionPolicyService updates the cache after a policy write', async () => {
  const stub = createStoreStub(() => false);
  const service = new ChannelIngestionPolicyService(stub.store);

  assert.equal(await service.isChannelEnabled('guild-1', 'channel-3'), false);

  await service.setChannelEnabled({
    channelId: 'channel-3',
    enabled: true,
    guildId: 'guild-1',
    updatedByUserId: 'user-1'
  });

  assert.equal(await service.isChannelEnabled('guild-1', 'channel-3'), true);
  assert.equal(stub.getQueryCount(), 1);
  assert.equal(stub.getSetCount(), 1);
});
