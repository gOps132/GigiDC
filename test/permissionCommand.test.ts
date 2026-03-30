import test from 'node:test';
import assert from 'node:assert/strict';

import { MessageFlags } from 'discord.js';

import { permissionCommand } from '../src/commands/permission.js';
import type { BotContext } from '../src/discord/types.js';

function createInteraction(overrides?: {
  capability?: string;
  command?: 'grant' | 'list' | 'revoke';
}) {
  const replyCalls: Array<{ content?: string; embeds?: Array<{ data?: { description?: string; title?: string } }>; flags?: number }> = [];
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
      getString(name: string, required?: boolean) {
        if (name === 'capability') {
          return overrides?.capability ?? 'permission_admin';
        }

        if (required) {
          throw new Error(`missing option ${name}`);
        }

        return null;
      },
      getSubcommand() {
        return overrides?.command ?? 'grant';
      },
      getUser() {
        return targetUser;
      }
    },
    user: {
      id: 'requester-1'
    },
    async reply(payload: { content?: string; embeds?: Array<{ data?: { description?: string; title?: string } }>; flags?: number }) {
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
      permissionAdmin: {
        async grantUserPermission(input: Record<string, unknown>) {
          calls.push({
            action: 'grant',
            ...input
          });
          return 'Granted `permission_admin` directly to <@target-1>.';
        },
        async listUserPermissions(input: Record<string, unknown>) {
          calls.push({
            action: 'list',
            ...input
          });
          return 'Permissions for <@target-1>:\nEffective capabilities: `permission_admin`';
        },
        async revokeUserPermission(input: Record<string, unknown>) {
          calls.push({
            action: 'revoke',
            ...input
          });
          return 'Revoked direct grant `permission_admin` from <@target-1>.';
        }
      },
      retrieval: {},
      rolePolicies: {},
      sensitiveData: {},
      userMemory: {}
    }
  } as unknown as BotContext;

  return {
    calls,
    context
  };
}

test('permission command grants a direct user capability', async () => {
  const { context, calls } = createContext();
  const { interaction, replyCalls } = createInteraction({
    command: 'grant'
  });

  await permissionCommand.execute(interaction as never, context);

  assert.equal(calls[0]?.action, 'grant');
  assert.equal(calls[0]?.capability, 'permission_admin');
  assert.equal(replyCalls[0]?.content, 'Granted `permission_admin` directly to <@target-1>.');
  assert.equal(replyCalls[0]?.flags, MessageFlags.Ephemeral);
});

test('permission command lists effective permissions with an embed', async () => {
  const { context, calls } = createContext();
  const { interaction, replyCalls } = createInteraction({
    command: 'list'
  });

  await permissionCommand.execute(interaction as never, context);

  assert.equal(calls[0]?.action, 'list');
  assert.equal(replyCalls[0]?.embeds?.[0]?.data?.title, 'Permission summary');
  assert.match(replyCalls[0]?.embeds?.[0]?.data?.description ?? '', /Effective capabilities/i);
});
