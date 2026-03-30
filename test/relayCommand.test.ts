import test from 'node:test';
import assert from 'node:assert/strict';

import { MessageFlags } from 'discord.js';

import { relayCommand } from '../src/commands/relay.js';
import type { BotContext } from '../src/discord/types.js';

function createInteraction(overrides?: {
  dispatchAllowed?: boolean;
  context?: string | null;
  message?: string;
  recipientAllowed?: boolean;
  recipientInGuild?: boolean;
  sendError?: Error | null;
  targetUserId?: string;
  targetUsername?: string;
  userId?: string;
}) {
  const auditCalls: Array<Record<string, unknown>> = [];
  const replyCalls: Array<{ content?: string; flags?: number }> = [];
  const confirmationCalls: Array<Record<string, unknown>> = [];

  const targetUser = {
    id: overrides?.targetUserId ?? 'target-1',
    username: overrides?.targetUsername ?? 'mina',
    toString() {
      return `<@${this.id}>`;
    },
    async send() {}
  };

  const interaction = {
    channelId: 'channel-1',
    guild: {
      id: 'guild-1',
      name: 'Gigi HQ',
      members: {
        async fetch(userId: string) {
          if (userId === (overrides?.targetUserId ?? 'target-1')) {
            if (overrides?.recipientInGuild === false) {
              throw new Error('Unknown guild member');
            }

            return {
              displayName: overrides?.targetUsername ?? 'mina',
              id: userId
            };
          }

          return {
            displayName: 'Erick',
            id: userId
          };
        }
      }
    },
    inGuild: () => true,
    options: {
      getString(name: string, required?: boolean) {
        if (name === 'message') {
          return overrides?.message ?? 'Please review the launch checklist.';
        }

        if (name === 'context') {
          return overrides?.context ?? 'This is for tonight’s release.';
        }

        if (required) {
          throw new Error(`missing option ${name}`);
        }

        return null;
      },
      getSubcommand() {
        return 'dm';
      },
      getUser(name: string) {
        if (name !== 'user') {
          return null;
        }

        return targetUser;
      }
    },
    user: {
      id: overrides?.userId ?? 'requester-1'
    },
    async reply(payload: { content?: string; flags?: number }) {
      replyCalls.push(payload);
    }
  };

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
      actionConfirmations: {
        async requestRelayConfirmation(input: Record<string, unknown>) {
          confirmationCalls.push(input);
          return {
            action: {
              id: 'action-1'
            },
            components: [{ type: 1 }],
            reply: 'Confirm within 15 minutes and I will send that DM relay.'
          };
        }
      },
      agentActions: {},
      assignments: {},
      auditLogs: {
        async record(input: Record<string, unknown>) {
          auditCalls.push(input);
        }
      },
      channelIngestionPolicies: {},
      dmConversation: {},
      messageHistory: {},
      messageIndexing: {},
      retrieval: {},
      rolePolicies: {
        async memberHasCapability(_guild: unknown, member: { id: string }, capability: string) {
          if (capability === 'agent_action_receive') {
            return member.id === (overrides?.targetUserId ?? 'target-1')
              ? overrides?.recipientAllowed ?? true
              : false;
          }

          return overrides?.dispatchAllowed ?? true;
        }
      },
      userMemory: {}
    }
  } as unknown as BotContext;

  return {
    auditCalls,
    confirmationCalls,
    context,
    interaction,
    replyCalls
  };
}

test('relay command denies users without dispatch capability', async () => {
  const { auditCalls, context, interaction, replyCalls } = createInteraction({
    dispatchAllowed: false
  });

  await relayCommand.execute(interaction as never, context);

  assert.equal(auditCalls.length, 1);
  assert.equal(auditCalls[0]?.action, 'relay.dm.permission_denied');
  assert.equal(replyCalls[0]?.content, 'You do not have permission to dispatch shared Gigi actions.');
  assert.equal(replyCalls[0]?.flags, MessageFlags.Ephemeral);
});

test('relay command denies relays when the recipient lacks receive permission', async () => {
  const { auditCalls, context, interaction, replyCalls } = createInteraction({
    dispatchAllowed: true,
    recipientAllowed: false
  });

  await relayCommand.execute(interaction as never, context);

  assert.equal(auditCalls[0]?.action, 'relay.dm.recipient_permission_denied');
  assert.match(replyCalls[0]?.content ?? '', /agent_action_receive/i);
  assert.equal(replyCalls[0]?.flags, MessageFlags.Ephemeral);
});

test('relay command requests a persisted confirmation instead of sending immediately', async () => {
  const {
    confirmationCalls,
    context,
    interaction,
    replyCalls
  } = createInteraction();

  await relayCommand.execute(interaction as never, context);

  assert.equal(confirmationCalls.length, 1);
  assert.equal(confirmationCalls[0]?.requesterUsername, 'Erick');
  assert.equal(confirmationCalls[0]?.recipientUsername, 'mina');
  assert.match(replyCalls[0]?.content ?? '', /confirm within 15 minutes/i);
  assert.equal(replyCalls[0]?.flags, MessageFlags.Ephemeral);
});
