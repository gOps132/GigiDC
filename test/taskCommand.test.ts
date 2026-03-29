import test from 'node:test';
import assert from 'node:assert/strict';

import { MessageFlags } from 'discord.js';

import { taskCommand } from '../src/commands/task.js';
import type { BotContext } from '../src/discord/types.js';
import { AGENT_ACTION_SCOPES, AGENT_ACTION_STATUSES, AGENT_ACTION_TYPES } from '../src/services/agentActionService.js';

function createInteraction(overrides?: {
  allowed?: boolean;
  assigneeId?: string;
  assigneeUsername?: string;
  command?: 'create' | 'list' | 'complete';
  dueAt?: string | null;
  existingTask?: Record<string, unknown> | null;
  taskId?: string;
  title?: string;
  userId?: string;
}) {
  const auditCalls: Array<Record<string, unknown>> = [];
  const createCalls: Array<Record<string, unknown>> = [];
  const listCalls: Array<Record<string, unknown>> = [];
  const completeCalls: Array<Record<string, unknown>> = [];
  const replyCalls: Array<{ content?: string; embeds?: Array<{ data?: { title?: string; description?: string } }>; flags?: number }> = [];

  const assignee = {
    id: overrides?.assigneeId ?? 'assignee-1',
    username: overrides?.assigneeUsername ?? 'mina'
  };

  const interaction = {
    channelId: 'channel-1',
    guild: {
      id: 'guild-1',
      members: {
        async fetch(userId: string) {
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
        if (name === 'title') {
          return overrides?.title ?? 'Prepare release notes';
        }
        if (name === 'details') {
          return 'Draft the release notes before standup.';
        }
        if (name === 'due_at') {
          return overrides?.dueAt ?? '2026-04-01T09:00:00Z';
        }
        if (name === 'task_id') {
          return overrides?.taskId ?? 'task-1';
        }
        if (name === 'result') {
          return 'Done';
        }
        if (required) {
          throw new Error(`missing option ${name}`);
        }
        return null;
      },
      getSubcommand() {
        return overrides?.command ?? 'create';
      },
      getUser(name: string) {
        if (name !== 'user') {
          return null;
        }
        return assignee;
      }
    },
    user: {
      id: overrides?.userId ?? 'requester-1',
      username: 'erick'
    },
    async reply(payload: { content?: string; embeds?: Array<{ data?: { title?: string; description?: string } }>; flags?: number }) {
      replyCalls.push(payload);
    }
  };

  const existingTask = overrides?.existingTask ?? {
    id: 'task-1',
    action_scope: AGENT_ACTION_SCOPES.task,
    guild_id: 'guild-1',
    channel_id: 'channel-1',
    requester_user_id: 'requester-1',
    requester_username: 'Erick',
    recipient_user_id: 'assignee-1',
    recipient_username: 'mina',
    action_type: AGENT_ACTION_TYPES.followUpTask,
    status: AGENT_ACTION_STATUSES.requested,
    visibility: 'participants',
    title: 'Prepare release notes',
    instructions: 'Draft the release notes before standup.',
    result_summary: null,
    error_message: null,
    metadata: {},
    due_at: '2026-04-01T09:00:00Z',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    completed_at: null
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
        async createFollowUpTask(input: Record<string, unknown>) {
          createCalls.push(input);
          return {
            ...existingTask,
            id: 'task-1',
            recipient_user_id: input.assigneeUserId,
            recipient_username: input.assigneeUsername,
            title: input.title,
            instructions: input.instructions,
            due_at: input.dueAt
          };
        },
        async getActionById() {
          return existingTask;
        },
        async listOpenTasksForUser(userId: string) {
          listCalls.push({ userId });
          return existingTask ? [existingTask] : [];
        },
        async markCompleted(task: Record<string, unknown>, input: Record<string, unknown>) {
          completeCalls.push({ task, input });
          return {
            ...task,
            status: AGENT_ACTION_STATUSES.completed,
            result_summary: input.resultSummary
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
      messageHistory: {},
      messageIndexing: {},
      retrieval: {},
      rolePolicies: {
        async memberHasCapability() {
          return overrides?.allowed ?? true;
        }
      }
    }
  } as unknown as BotContext;

  return {
    auditCalls,
    completeCalls,
    context,
    createCalls,
    interaction,
    listCalls,
    replyCalls
  };
}

test('task create requires shared-action dispatch capability', async () => {
  const { auditCalls, context, interaction, replyCalls } = createInteraction({
    allowed: false,
    command: 'create'
  });

  await taskCommand.execute(interaction as never, context);

  assert.equal(auditCalls[0]?.action, 'task.create.permission_denied');
  assert.equal(replyCalls[0]?.content, 'You do not have permission to create shared Gigi tasks.');
  assert.equal(replyCalls[0]?.flags, MessageFlags.Ephemeral);
});

test('task create stores a follow-up task', async () => {
  const { auditCalls, context, createCalls, interaction, replyCalls } = createInteraction({
    command: 'create'
  });

  await taskCommand.execute(interaction as never, context);

  assert.equal(createCalls.length, 1);
  assert.equal(createCalls[0]?.title, 'Prepare release notes');
  assert.equal(replyCalls[0]?.embeds?.[0]?.data?.title, 'Task created');
  assert.equal(auditCalls[0]?.action, 'task.created');
});

test('task list allows self-views without dispatch capability', async () => {
  const { context, interaction, listCalls, replyCalls } = createInteraction({
    allowed: false,
    command: 'list',
    assigneeId: 'requester-1'
  });

  await taskCommand.execute(interaction as never, context);

  assert.equal(listCalls.length, 1);
  assert.equal(listCalls[0]?.userId, 'requester-1');
  assert.equal(replyCalls[0]?.embeds?.[0]?.data?.title, 'Your open Gigi tasks');
});

test('task complete lets a participant close the task', async () => {
  const { auditCalls, completeCalls, context, interaction, replyCalls } = createInteraction({
    allowed: false,
    command: 'complete',
    userId: 'assignee-1'
  });

  await taskCommand.execute(interaction as never, context);

  assert.equal(completeCalls.length, 1);
  assert.equal(auditCalls[0]?.action, 'task.completed');
  assert.equal(replyCalls[0]?.embeds?.[0]?.data?.title, 'Task completed');
});
