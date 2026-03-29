import test from 'node:test';
import assert from 'node:assert/strict';

import {
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
      guild_id: input.guildId,
      channel_id: input.channelId,
      requester_user_id: input.requesterUserId,
      requester_username: input.requesterUsername,
      recipient_user_id: input.recipientUserId,
      recipient_username: input.recipientUsername,
      action_type: input.actionType,
      status: AGENT_ACTION_STATUSES.requested,
      visibility: input.visibility,
      title: input.title,
      instructions: input.instructions,
      result_summary: null,
      error_message: null,
      metadata: input.metadata ?? {},
      created_at: new Date(Date.now() - this.actions.length * 1_000).toISOString(),
      updated_at: new Date().toISOString(),
      completed_at: null
    };

    this.actions.push(record);
    return record;
  }

  async listVisibleRecentForUser(userId: string): Promise<AgentActionRecord[]> {
    return this.actions.filter(
      (action) => action.requester_user_id === userId || action.recipient_user_id === userId
    );
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
  assert.equal(action.visibility, 'participants');
  assert.equal(action.metadata.context, 'Follow up on tonight’s release.');
});

test('AgentActionService ranks relevant visible actions for follow-up questions', async () => {
  const store = new InMemoryAgentActionStore();
  const service = new AgentActionService(store);

  const relevant = await store.createAction({
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
    metadata: {}
  });
  await store.updateActionStatus({
    actionId: relevant.id,
    status: AGENT_ACTION_STATUSES.completed,
    resultSummary: 'Delivered DM relay to Mina'
  });

  await store.createAction({
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
