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
  const completedCalls: Array<Record<string, unknown>> = [];
  const failedCalls: Array<Record<string, unknown>> = [];
  const createdActions: Array<Record<string, unknown>> = [];
  const sentPayloads: Array<{ content: string }> = [];
  const storedBotMessages: string[] = [];

  const targetUser = {
    id: overrides?.targetUserId ?? 'target-1',
    username: overrides?.targetUsername ?? 'mina',
    toString() {
      return `<@${this.id}>`;
    },
    async send(payload: { content: string }) {
      sentPayloads.push(payload);
      if (overrides?.sendError) {
        throw overrides.sendError;
      }

      return {
        channelId: 'dm-channel-1',
        id: 'dm-message-1',
        author: {
          bot: true
        }
      };
    }
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
      agentActions: {
        async createDirectMessageRelay(input: Record<string, unknown>) {
          createdActions.push(input);
          return {
            id: 'action-1',
            metadata: input.metadata ?? {}
          };
        },
        async markCompleted(action: Record<string, unknown>, input: Record<string, unknown>) {
          completedCalls.push({
            action,
            input
          });
          return {
            id: action.id
          };
        },
        async markFailed(action: Record<string, unknown>, errorMessage: string) {
          failedCalls.push({
            action,
            errorMessage
          });
          return {
            id: action.id
          };
        }
      },
      assignments: {},
      auditLogs: {
        async record(input: Record<string, unknown>) {
          auditCalls.push(input);
        }
      },
      channelIngestionPolicies: {},
      dmConversation: {},
      messageHistory: {
        async storeBotAuthoredMessage(message: { id: string }) {
          storedBotMessages.push(message.id);
          return {
            reason: 'stored' as const,
            stored: true
          };
        }
      },
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
      }
    }
  } as unknown as BotContext;

  return {
    auditCalls,
    completedCalls,
    context,
    createdActions,
    failedCalls,
    interaction,
    replyCalls,
    sentPayloads,
    storedBotMessages
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

test('relay command stores the action, sends the DM, and records success', async () => {
  const {
    auditCalls,
    completedCalls,
    context,
    createdActions,
    interaction,
    replyCalls,
    sentPayloads,
    storedBotMessages
  } = createInteraction();

  await relayCommand.execute(interaction as never, context);

  assert.equal(createdActions.length, 1);
  assert.equal(createdActions[0]?.requesterUsername, 'Erick');
  assert.equal(createdActions[0]?.recipientUsername, 'mina');
  assert.equal(sentPayloads.length, 1);
  assert.deepEqual(storedBotMessages, ['dm-message-1']);
  assert.match(sentPayloads[0]?.content ?? '', /Erick asked me to pass this along/i);
  assert.equal(completedCalls.length, 1);
  assert.equal(completedCalls[0]?.input?.metadata?.historyStored, true);
  assert.equal(auditCalls[0]?.action, 'relay.dm.sent');
  assert.equal(replyCalls[0]?.content, 'Sent a DM to <@target-1>.');
});

test('relay command marks the action failed when Discord rejects the DM', async () => {
  const {
    auditCalls,
    context,
    failedCalls,
    interaction,
    replyCalls
  } = createInteraction({
    dispatchAllowed: true,
    sendError: new Error('Cannot send messages to this user')
  });

  await relayCommand.execute(interaction as never, context);

  assert.equal(failedCalls.length, 1);
  assert.match(String(failedCalls[0]?.errorMessage ?? ''), /cannot send/i);
  assert.equal(auditCalls[0]?.action, 'relay.dm.failed');
  assert.match(replyCalls[0]?.content ?? '', /could not DM/i);
});
