import test from 'node:test';
import assert from 'node:assert/strict';

import { AgentToolService } from '../src/services/agentToolService.js';
import { AGENT_ACTION_SCOPES, AGENT_ACTION_STATUSES, AGENT_ACTION_TYPES } from '../src/services/agentActionService.js';

function createService(overrides?: {
  capabilityAllowed?: boolean;
  getActionByIdResult?: Record<string, unknown> | null;
  listOpenTasksResult?: Array<Record<string, unknown>>;
  plan?: {
    toolCalls: Array<Record<string, unknown>>;
  };
}) {
  const auditCalls: Array<Record<string, unknown>> = [];
  const createTaskCalls: Array<Record<string, unknown>> = [];
  const listOpenTaskCalls: Array<Record<string, unknown>> = [];
  const markCompletedCalls: Array<Record<string, unknown>> = [];

  const taskRecord = {
    id: 'task-1',
    action_scope: AGENT_ACTION_SCOPES.task,
    guild_id: 'guild-1',
    channel_id: 'dm-channel-1',
    requester_user_id: 'requester-1',
    requester_username: 'Erick',
    recipient_user_id: 'requester-1',
    recipient_username: 'erick',
    action_type: AGENT_ACTION_TYPES.followUpTask,
    status: AGENT_ACTION_STATUSES.requested,
    visibility: 'requester_only' as const,
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

  const guild = {
    id: 'guild-1',
    members: {
      async fetch(_userId: string) {
        return {
          displayName: 'Erick'
        };
      },
      async search() {
        return new Map();
      }
    }
  };

  const client = {
    guilds: {
      cache: new Map([['guild-1', guild]]),
      async fetch() {
        return guild;
      }
    },
    users: {
      async fetch(userId: string) {
        return {
          id: userId,
          send: async () => ({
            author: {
              bot: true
            },
            channelId: 'dm-target-channel',
            id: 'relay-msg-1'
          }),
          username: 'mina'
        };
      }
    }
  };

  const service = new AgentToolService(
    {
      DISCORD_GUILD_ID: 'guild-1',
      OPENAI_RESPONSE_MODEL: 'gpt-test'
    } as never,
    {
      async planDmTools() {
        return overrides?.plan ?? { toolCalls: [] };
      }
    },
    {
      async createDirectMessageRelay() {
        throw new Error('not needed in this test');
      },
      async createFollowUpTask(input: Record<string, unknown>) {
        createTaskCalls.push(input);
        return {
          ...taskRecord,
          due_at: input.dueAt ?? null,
          recipient_user_id: input.assigneeUserId,
          recipient_username: input.assigneeUsername,
          title: input.title,
          instructions: input.instructions
        };
      },
      async getActionById() {
        return overrides?.getActionByIdResult ?? null;
      },
      async listOpenTasksForUser(userId: string) {
        listOpenTaskCalls.push({ userId });
        return (overrides?.listOpenTasksResult as never) ?? [taskRecord];
      },
      async markCompleted(task: Record<string, unknown>, input: Record<string, unknown>) {
        markCompletedCalls.push({ input, task });
        return {
          ...task,
          completed_at: new Date().toISOString(),
          result_summary: input.resultSummary ?? null,
          status: AGENT_ACTION_STATUSES.completed
        };
      },
      async markFailed() {
        throw new Error('not needed in this test');
      }
    } as never,
    {
      async record(input: Record<string, unknown>) {
        auditCalls.push(input);
      }
    } as never,
    {
      async storeBotAuthoredMessage() {
        return {
          reason: 'stored' as const,
          stored: true
        };
      }
    } as never,
    {
      async memberHasCapability() {
        return overrides?.capabilityAllowed ?? false;
      }
    } as never,
    {
      debug() {},
      error() {},
      info() {},
      warn() {}
    } as never
  );

  return {
    auditCalls,
    client,
    createTaskCalls,
    listOpenTaskCalls,
    markCompletedCalls,
    service
  };
}

test('AgentToolService can execute multiple planned tool calls in one DM turn', async () => {
  const { client, createTaskCalls, listOpenTaskCalls, service } = createService({
    capabilityAllowed: true,
    plan: {
      toolCalls: [
        {
          assigneeReference: 'me',
          details: 'Draft the release notes before standup.',
          dueAt: '2026-04-01T09:00:00Z',
          name: 'create_follow_up_task',
          title: 'Prepare release notes'
        },
        {
          name: 'list_open_tasks',
          userReference: 'me'
        }
      ]
    }
  });

  const result = await service.maybeHandleDmQuery(
    'Create a task for me to prepare release notes tomorrow and show me my tasks.',
    {
      id: 'requester-1',
      username: 'erick'
    } as never,
    client as never,
    'dm-channel-1'
  );

  assert.ok(result);
  assert.deepEqual(result.executedToolNames, ['create_follow_up_task', 'list_open_tasks']);
  assert.equal(createTaskCalls.length, 1);
  assert.equal(listOpenTaskCalls.length, 1);
  assert.match(result.reply, /Created task `task-1`/);
  assert.match(result.reply, /Your open Gigi tasks/i);
});

test('AgentToolService denies DM relay execution without shared dispatch capability', async () => {
  const { auditCalls, client, service } = createService({
    capabilityAllowed: false,
    plan: {
      toolCalls: [
        {
          context: null,
          message: 'Please review the checklist.',
          name: 'send_dm_relay',
          recipientReference: 'mina'
        }
      ]
    }
  });

  const result = await service.maybeHandleDmQuery(
    'Send Mina a DM to review the checklist.',
    {
      id: 'requester-1',
      username: 'erick'
    } as never,
    client as never,
    'dm-channel-1'
  );

  assert.ok(result);
  assert.match(result.reply, /agent_action_dispatch/i);
  assert.equal(auditCalls[0]?.action, 'dm.tools.send_dm_relay.permission_denied');
});

test('AgentToolService can complete a task by title reference', async () => {
  const { client, markCompletedCalls, service } = createService({
    capabilityAllowed: false,
    getActionByIdResult: null,
    plan: {
      toolCalls: [
        {
          name: 'complete_task',
          result: 'Done',
          taskReference: 'release notes'
        }
      ]
    }
  });

  const result = await service.maybeHandleDmQuery(
    'Mark the release notes task done.',
    {
      id: 'requester-1',
      username: 'erick'
    } as never,
    client as never,
    'dm-channel-1'
  );

  assert.ok(result);
  assert.equal(markCompletedCalls.length, 1);
  assert.match(result.reply, /Marked task `task-1` complete/i);
});
