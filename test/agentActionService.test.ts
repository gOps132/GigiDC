import test from 'node:test';
import assert from 'node:assert/strict';

import {
  AGENT_ACTION_SCOPES,
  AGENT_ACTION_STATUSES,
  AGENT_ACTION_TYPES,
  AgentActionService,
  type AgentActionRecord,
  type CreateAgentActionInput,
  type UpdateAgentActionStatusInput
} from '../src/services/agentActionService.js';
import type { AgentActionStore } from '../src/ports/controlPlane.js';

class InMemoryAgentActionStore implements AgentActionStore {
  readonly actions: AgentActionRecord[] = [];

  async createAction(input: CreateAgentActionInput): Promise<AgentActionRecord> {
    const record: AgentActionRecord = {
      id: `action-${this.actions.length + 1}`,
      action_scope: input.actionScope,
      guild_id: input.guildId,
      channel_id: input.channelId,
      requester_user_id: input.requesterUserId,
      requester_username: input.requesterUsername,
      recipient_user_id: input.recipientUserId,
      recipient_username: input.recipientUsername,
      action_type: input.actionType,
      visibility: input.visibility,
      title: input.title,
      instructions: input.instructions,
      result_summary: null,
      error_message: null,
      metadata: input.metadata ?? {},
      due_at: input.dueAt ?? null,
      confirmation_requested_at: input.confirmationRequestedAt ?? null,
      confirmation_expires_at: input.confirmationExpiresAt ?? null,
      confirmed_at: null,
      confirmed_by_user_id: null,
      created_at: new Date(Date.now() - this.actions.length * 1_000).toISOString(),
      updated_at: new Date().toISOString(),
      completed_at: null,
      cancelled_at: null,
      status: input.initialStatus ?? AGENT_ACTION_STATUSES.requested
    };

    this.actions.push(record);
    return record;
  }

  async getActionById(actionId: string): Promise<AgentActionRecord | null> {
    return this.actions.find((action) => action.id === actionId) ?? null;
  }

  async listVisibleRecentForUser(
    userId: string,
    _limit: number,
    options?: {
      actionScope?: AgentActionRecord['action_scope'];
      statuses?: AgentActionRecord['status'][];
    }
  ): Promise<AgentActionRecord[]> {
    return this.actions.filter((action) => {
      const visible = action.requester_user_id === userId || action.recipient_user_id === userId;
      if (!visible) {
        return false;
      }

      if (options?.actionScope && action.action_scope !== options.actionScope) {
        return false;
      }

      if (options?.statuses && !options.statuses.includes(action.status)) {
        return false;
      }

      return true;
    });
  }

  async updateActionStatus(input: UpdateAgentActionStatusInput): Promise<AgentActionRecord> {
    const action = this.actions.find((record) => record.id === input.actionId);
    if (!action) {
      throw new Error('missing action');
    }

    action.status = input.status;
    action.result_summary = input.resultSummary ?? null;
    action.error_message = input.errorMessage ?? null;
    action.metadata = input.metadata ?? {};
    action.completed_at = input.completedAt ?? null;
    action.confirmation_requested_at = input.confirmationRequestedAt ?? action.confirmation_requested_at;
    action.confirmation_expires_at = input.confirmationExpiresAt ?? action.confirmation_expires_at;
    action.confirmed_at = input.confirmedAt ?? action.confirmed_at;
    action.confirmed_by_user_id = input.confirmedByUserId ?? action.confirmed_by_user_id;
    action.cancelled_at = input.cancelledAt ?? action.cancelled_at;
    action.updated_at = new Date().toISOString();
    return action;
  }
}

test('AgentActionService creates DM relay actions with participant visibility', async () => {
  const store = new InMemoryAgentActionStore();
  const service = new AgentActionService(store);

  const action = await service.createDirectMessageRelay({
    guildId: 'guild-1',
    channelId: 'channel-1',
    requesterUserId: 'user-1',
    requesterUsername: 'Erick',
    recipientUserId: 'user-2',
    recipientUsername: 'Mina',
    message: 'Please send me the deployment notes.',
    context: 'Follow up on tonight’s release.'
  });

  assert.equal(action.action_type, AGENT_ACTION_TYPES.dmRelay);
  assert.equal(action.action_scope, AGENT_ACTION_SCOPES.action);
  assert.equal(action.visibility, 'participants');
  assert.equal(action.metadata.context, 'Follow up on tonight’s release.');
});

test('AgentActionService creates self tasks as requester-only open tasks', async () => {
  const store = new InMemoryAgentActionStore();
  const service = new AgentActionService(store);

  const task = await service.createFollowUpTask({
    guildId: 'guild-1',
    channelId: 'channel-1',
    requesterUserId: 'user-1',
    requesterUsername: 'Erick',
    assigneeUserId: 'user-1',
    assigneeUsername: 'Erick',
    title: 'Prepare release notes',
    instructions: 'Draft the release notes before standup.',
    dueAt: '2026-04-01T09:00:00Z'
  });

  assert.equal(task.action_type, AGENT_ACTION_TYPES.followUpTask);
  assert.equal(task.action_scope, AGENT_ACTION_SCOPES.task);
  assert.equal(task.visibility, 'requester_only');
  assert.equal(task.due_at, '2026-04-01T09:00:00Z');
});

test('AgentActionService ranks relevant visible actions for follow-up questions', async () => {
  const store = new InMemoryAgentActionStore();
  const service = new AgentActionService(store);

  const relevant = await store.createAction({
    actionScope: AGENT_ACTION_SCOPES.action,
    guildId: 'guild-1',
    channelId: 'channel-1',
    requesterUserId: 'erick-id',
    requesterUsername: 'Erick',
    recipientUserId: 'user-2',
    recipientUsername: 'Mina',
    actionType: AGENT_ACTION_TYPES.dmRelay,
    visibility: 'participants',
    title: 'DM relay from Erick to Mina',
    instructions: 'Please review the launch draft tonight.',
    initialStatus: AGENT_ACTION_STATUSES.requested,
    confirmationRequestedAt: null,
    confirmationExpiresAt: null,
    metadata: {}
  });
  await store.updateActionStatus({
    actionId: relevant.id,
    status: AGENT_ACTION_STATUSES.completed,
    resultSummary: 'Delivered DM relay to Mina'
  });

  await store.createAction({
    actionScope: AGENT_ACTION_SCOPES.action,
    guildId: 'guild-1',
    channelId: 'channel-2',
    requesterUserId: 'other-id',
    requesterUsername: 'Kai',
    recipientUserId: 'user-2',
    recipientUsername: 'Mina',
    actionType: AGENT_ACTION_TYPES.dmRelay,
    visibility: 'participants',
    title: 'DM relay from Kai to Mina',
    instructions: 'Morning standup moved by ten minutes.',
    initialStatus: AGENT_ACTION_STATUSES.requested,
    confirmationRequestedAt: null,
    confirmationExpiresAt: null,
    metadata: {}
  });

  const ranked = await service.listRelevantVisibleActionsForUser(
    'user-2',
    'what did Erick want again?',
    2
  );

  assert.equal(ranked.length, 2);
  assert.equal(ranked[0]?.requester_username, 'Erick');
  assert.match(ranked[0]?.instructions ?? '', /launch draft/i);
});

test('AgentActionService lists open tasks ordered by due date', async () => {
  const store = new InMemoryAgentActionStore();
  const service = new AgentActionService(store);

  await store.createAction({
    actionScope: AGENT_ACTION_SCOPES.task,
    guildId: 'guild-1',
    channelId: 'channel-1',
    requesterUserId: 'manager-id',
    requesterUsername: 'Manager',
    recipientUserId: 'user-2',
    recipientUsername: 'Mina',
    actionType: AGENT_ACTION_TYPES.followUpTask,
    visibility: 'participants',
    title: 'Later task',
    instructions: 'Handle this later.',
    dueAt: '2026-04-05T09:00:00Z',
    initialStatus: AGENT_ACTION_STATUSES.requested,
    confirmationRequestedAt: null,
    confirmationExpiresAt: null,
    metadata: {}
  });

  await store.createAction({
    actionScope: AGENT_ACTION_SCOPES.task,
    guildId: 'guild-1',
    channelId: 'channel-1',
    requesterUserId: 'manager-id',
    requesterUsername: 'Manager',
    recipientUserId: 'user-2',
    recipientUsername: 'Mina',
    actionType: AGENT_ACTION_TYPES.followUpTask,
    visibility: 'participants',
    title: 'Sooner task',
    instructions: 'Handle this first.',
    dueAt: '2026-04-01T09:00:00Z',
    initialStatus: AGENT_ACTION_STATUSES.requested,
    confirmationRequestedAt: null,
    confirmationExpiresAt: null,
    metadata: {}
  });

  const tasks = await service.listOpenTasksForUser('user-2', 5);
  assert.equal(tasks.length, 2);
  assert.equal(tasks[0]?.title, 'Sooner task');
});
