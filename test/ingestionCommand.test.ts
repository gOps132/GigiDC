import test from 'node:test';
import assert from 'node:assert/strict';

import { MessageFlags } from 'discord.js';

import { ingestionCommand } from '../src/commands/ingestion.js';
import type { BotContext } from '../src/discord/types.js';

function createInteraction(overrides?: {
  channelId?: string;
  channelName?: string;
  command?: 'enable' | 'disable' | 'status';
  selectedChannelId?: string | null;
  selectedChannelName?: string | null;
  userId?: string;
}) {
  const replyCalls: Array<{ content?: string; embeds?: Array<{ data?: { title?: string } }>; flags?: number }> = [];
  const command = overrides?.command ?? 'enable';
  const channelId = overrides?.channelId ?? 'channel-1';
  const channelName = overrides?.channelName ?? 'general';
  const selectedChannelId = overrides?.selectedChannelId ?? null;
  const selectedChannelName = overrides?.selectedChannelName ?? null;

  const interaction = {
    channel: {
      id: channelId,
      name: channelName
    },
    guild: {
      id: 'guild-1',
      members: {
        fetch: async (userId: string) => ({
          id: userId
        })
      }
    },
    inGuild: () => true,
    options: {
      getChannel(name: string) {
        if (name !== 'channel' || !selectedChannelId) {
          return null;
        }

        return {
          id: selectedChannelId,
          name: selectedChannelName ?? selectedChannelId
        };
      },
      getSubcommand() {
        return command;
      }
    },
    user: {
      id: overrides?.userId ?? 'user-1'
    },
    async reply(payload: { content?: string; embeds?: Array<{ data?: { title?: string } }>; flags?: number }) {
      replyCalls.push(payload);
    }
  };

  return {
    interaction,
    replyCalls
  };
}

function createContext(overrides?: {
  allowed?: boolean;
  existingEnabled?: boolean | null;
}) {
  const auditCalls: Array<Record<string, unknown>> = [];
  const setCalls: Array<Record<string, unknown>> = [];

  const context = {
    env: {},
    logger: {
      debug() {},
      error() {},
      info() {},
      warn() {}
    },
    runtime: {},
    services: {
      agentActions: {},
      assignments: {},
      auditLogs: {
        async record(input: Record<string, unknown>) {
          auditCalls.push(input);
        }
      },
      channelIngestionPolicies: {
        async getPolicy(guildId: string, channelId: string) {
          if (overrides?.existingEnabled == null) {
            return null;
          }

          return {
            channel_id: channelId,
            created_at: new Date().toISOString(),
            enabled: overrides.existingEnabled,
            guild_id: guildId,
            id: `${guildId}:${channelId}`,
            updated_at: new Date().toISOString(),
            updated_by_user_id: 'user-2'
          };
        },
        async setChannelEnabled(input: Record<string, unknown>) {
          setCalls.push(input);
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
      },
      dmConversation: {},
      messageHistory: {},
      messageIndexing: {},
      retrieval: {},
      rolePolicies: {
        async memberHasCapability() {
          return overrides?.allowed ?? false;
        }
      }
    }
  } as unknown as BotContext;

  return {
    auditCalls,
    context,
    setCalls
  };
}

test('ingestion command records a permission-denied audit log', async () => {
  const { context, auditCalls, setCalls } = createContext({
    allowed: false
  });
  const { interaction, replyCalls } = createInteraction();

  await ingestionCommand.execute(interaction as never, context);

  assert.equal(setCalls.length, 0);
  assert.equal(auditCalls.length, 1);
  assert.equal(auditCalls[0]?.action, 'ingestion.permission_denied');
  assert.equal(replyCalls[0]?.content, 'You do not have permission to manage ingestion policies.');
  assert.equal(replyCalls[0]?.flags, MessageFlags.Ephemeral);
});

test('ingestion enable updates the policy and writes an audit log', async () => {
  const { context, auditCalls, setCalls } = createContext({
    allowed: true,
    existingEnabled: false
  });
  const { interaction, replyCalls } = createInteraction({
    command: 'enable',
    selectedChannelId: 'channel-9',
    selectedChannelName: 'shipping'
  });

  await ingestionCommand.execute(interaction as never, context);

  assert.equal(setCalls.length, 1);
  assert.deepEqual(setCalls[0], {
    channelId: 'channel-9',
    enabled: true,
    guildId: 'guild-1',
    updatedByUserId: 'user-1'
  });
  assert.equal(auditCalls.length, 1);
  assert.equal(auditCalls[0]?.action, 'ingestion.enabled');
  assert.equal(replyCalls[0]?.embeds?.[0]?.data?.title, 'Ingestion enabled');
});
