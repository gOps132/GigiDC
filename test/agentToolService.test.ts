import test from 'node:test';
import assert from 'node:assert/strict';

import { AgentToolService } from '../src/services/agentToolService.js';
import { AGENT_ACTION_SCOPES, AGENT_ACTION_STATUSES, AGENT_ACTION_TYPES } from '../src/services/agentActionService.js';

function createService(overrides?: {
  dispatchAllowed?: boolean;
  getActionByIdResult?: Record<string, unknown> | null;
  listOpenTasksResult?: Array<Record<string, unknown>>;
  plan?: {
    toolCalls: Array<Record<string, unknown>>;
  };
  relayPrompt?: string;
  recipientAllowed?: boolean;
}) {
  const auditCalls: Array<Record<string, unknown>> = [];
  const confirmationCalls: Array<Record<string, unknown>> = [];
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
    confirmation_requested_at: null,
    confirmation_expires_at: null,
    confirmed_at: null,
    confirmed_by_user_id: null,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    completed_at: null,
    cancelled_at: null
  };

  const guild = {
    id: 'guild-1',
    members: {
      async fetch(_userId: string) {
        return {
          displayName: 'Erick'
        };
      },
      async search({ query }: { query: string }) {
        const normalized = query.toLowerCase();
        if (normalized === 'mina') {
          return new Map([
            ['recipient-1', {
              displayName: 'Mina',
              user: {
                id: 'recipient-1',
                username: 'mina'
              }
            }]
          ]);
        }

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
      async requestRelayConfirmation(input: Record<string, unknown>) {
        confirmationCalls.push(input);
        return {
          action: {
            id: 'action-2'
          },
          components: [{ type: 1 }],
          reply: overrides?.relayPrompt ?? 'Confirm within 15 minutes and I will send that DM relay.'
        };
      }
    } as never,
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
      async createAssignment() {
        return 'Created assignment `assignment-1`.';
      },
      async getIngestionStatus() {
        return 'Ingestion for <#channel-1> is enabled.';
      },
      async listAssignments() {
        return 'Recent assignments:\n- `assignment-1` Release notes [draft] no due date';
      },
      async publishAssignment() {
        return 'Published assignment `assignment-1` to <#channel-1>.';
      },
      async setIngestionPolicy() {
        return 'Enabled ingestion for <#channel-1>.';
      }
    } as never,
    {
      async grantUserPermission() {
        return 'Granted `permission_admin` directly to <@recipient-1>.';
      },
      async listUserPermissions() {
        return 'Permissions for <@requester-1>:\nEffective capabilities: `agent_action_dispatch`';
      },
      async revokeUserPermission() {
        return 'Revoked direct grant `permission_admin` from <@recipient-1>.';
      }
    } as never,
    {
      async memberHasCapability(_guild: unknown, _member: unknown, capability: string) {
        if (capability === 'agent_action_receive') {
          return overrides?.recipientAllowed ?? false;
        }

        return overrides?.dispatchAllowed ?? false;
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
    confirmationCalls,
    createTaskCalls,
    listOpenTaskCalls,
    markCompletedCalls,
    service
  };
}

test('AgentToolService can execute multiple planned tool calls in one DM turn', async () => {
  const { client, createTaskCalls, listOpenTaskCalls, service } = createService({
    dispatchAllowed: true,
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
    dispatchAllowed: false,
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

test('AgentToolService turns DM relay execution into a persisted confirmation prompt', async () => {
  const { client, confirmationCalls, service } = createService({
    dispatchAllowed: true,
    plan: {
      toolCalls: [
        {
          context: null,
          message: 'Please review the checklist.',
          name: 'send_dm_relay',
          recipientReference: 'mina'
        }
      ]
    },
    recipientAllowed: true,
    relayPrompt: 'Confirm within 15 minutes and I will send that DM relay.'
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
  assert.equal(confirmationCalls.length, 1);
  assert.equal(confirmationCalls[0]?.recipientUsername, 'mina');
  assert.match(result.reply, /confirm within 15 minutes/i);
  assert.ok(result.components);
});

test('AgentToolService routes ingestion status requests through guild admin actions', async () => {
  const { client, service } = createService({
    plan: {
      toolCalls: [
        {
          channelReference: 'shipping',
          name: 'get_ingestion_status'
        }
      ]
    }
  });

  const result = await service.maybeHandleDmQuery(
    'Show ingestion status for shipping.',
    {
      id: 'requester-1',
      username: 'erick'
    } as never,
    client as never,
    'dm-channel-1'
  );

  assert.ok(result);
  assert.deepEqual(result.executedToolNames, ['get_ingestion_status']);
  assert.match(result.reply, /Ingestion for <#channel-1> is enabled/i);
});

test('AgentToolService routes permission inspection requests through permission admin actions', async () => {
  const { client, service } = createService({
    plan: {
      toolCalls: [
        {
          name: 'list_permissions',
          userReference: 'me'
        }
      ]
    }
  });

  const result = await service.maybeHandleDmQuery(
    'What permissions do I have?',
    {
      id: 'requester-1',
      username: 'erick'
    } as never,
    client as never,
    'dm-channel-1'
  );

  assert.ok(result);
  assert.deepEqual(result.executedToolNames, ['list_permissions']);
  assert.match(result.reply, /Effective capabilities/i);
});

test('AgentToolService can complete a task by title reference', async () => {
  const { client, markCompletedCalls, service } = createService({
    dispatchAllowed: false,
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

test('AgentToolService denies DM relay execution when the recipient lacks receive permission', async () => {
  const { auditCalls, client, service } = createService({
    dispatchAllowed: true,
    recipientAllowed: false,
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
  assert.match(result.reply, /agent_action_receive/i);
  assert.equal(auditCalls[0]?.action, 'dm.tools.send_dm_relay.recipient_permission_denied');
});
