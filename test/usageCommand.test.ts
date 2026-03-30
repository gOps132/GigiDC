import test from 'node:test';
import assert from 'node:assert/strict';

import { MessageFlags } from 'discord.js';

import { usageCommand } from '../src/commands/usage.js';
import type { BotContext } from '../src/discord/types.js';

function createInteraction(overrides?: {
  command?: 'summary' | 'user';
  days?: number | null;
}) {
  const replyCalls: Array<{ embeds?: Array<{ data?: { description?: string; title?: string } }>; flags?: number }> = [];
  const targetUser = {
    id: 'target-1',
    username: 'mina'
  };

  const interaction = {
    client: {},
    guild: {
      id: 'guild-1'
    },
    inGuild: () => true,
    options: {
      getInteger(name: string) {
        if (name === 'days') {
          return overrides?.days ?? null;
        }

        return null;
      },
      getSubcommand() {
        return overrides?.command ?? 'summary';
      },
      getUser() {
        return targetUser;
      }
    },
    user: {
      id: 'requester-1'
    },
    async reply(payload: { embeds?: Array<{ data?: { description?: string; title?: string } }>; flags?: number }) {
      replyCalls.push(payload);
    }
  };

  return {
    interaction,
    replyCalls,
    targetUser
  };
}

function createContext() {
  const calls: Array<Record<string, unknown>> = [];
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
      actionConfirmations: {},
      agentActions: {},
      agentTools: {},
      assignments: {},
      auditLogs: {},
      channelIngestionPolicies: {},
      dmConversation: {},
      messageHistory: {},
      messageIndexing: {},
      permissionAdmin: {},
      retrieval: {},
      rolePolicies: {},
      sensitiveData: {},
      usageAdmin: {
        async getUsageSummary(input: Record<string, unknown>) {
          calls.push({
            action: 'summary',
            ...input
          });
          return 'Server model usage for the last 7 days:\nEstimated cost: $0.100000';
        },
        async getUserUsageSummary(input: Record<string, unknown>) {
          calls.push({
            action: 'user',
            ...input
          });
          return 'Usage for <@target-1> for the last 3 days:\nEstimated cost: $0.010000';
        }
      },
      userMemory: {}
    }
  } as unknown as BotContext;

  return {
    calls,
    context
  };
}

test('usage command shows server summary', async () => {
  const { context, calls } = createContext();
  const { interaction, replyCalls } = createInteraction({
    command: 'summary'
  });

  await usageCommand.execute(interaction as never, context);

  assert.equal(calls[0]?.action, 'summary');
  assert.equal(calls[0]?.days, 7);
  assert.equal(replyCalls[0]?.embeds?.[0]?.data?.title, 'Usage summary');
  assert.match(replyCalls[0]?.embeds?.[0]?.data?.description ?? '', /Estimated cost/i);
  assert.equal(replyCalls[0]?.flags, MessageFlags.Ephemeral);
});

test('usage command shows per-user summary', async () => {
  const { context, calls } = createContext();
  const { interaction, replyCalls } = createInteraction({
    command: 'user',
    days: 3
  });

  await usageCommand.execute(interaction as never, context);

  assert.equal(calls[0]?.action, 'user');
  assert.equal(calls[0]?.days, 3);
  const targetUser = calls[0]?.targetUser as { id?: string } | undefined;
  assert.equal(targetUser?.id, 'target-1');
  assert.equal(replyCalls[0]?.embeds?.[0]?.data?.title, 'User usage summary');
  assert.match(replyCalls[0]?.embeds?.[0]?.data?.description ?? '', /<@target-1>/);
});
