import test from 'node:test';
import assert from 'node:assert/strict';

import { RetrievalService } from '../src/services/retrievalService.js';
import { AGENT_ACTION_STATUSES, AGENT_ACTION_TYPES, type AgentActionRecord } from '../src/services/agentActionService.js';

test('RetrievalService can answer from shared action history when message history is empty', async () => {
  const responseInputs: Array<{ instructions: string; model: string; text: string }> = [];
  const action: AgentActionRecord = {
    id: 'action-1',
    guild_id: 'guild-1',
    channel_id: 'channel-1',
    requester_user_id: 'erick-id',
    requester_username: 'Erick',
    recipient_user_id: 'user-2',
    recipient_username: 'Mina',
    action_type: AGENT_ACTION_TYPES.dmRelay,
    status: AGENT_ACTION_STATUSES.completed,
    visibility: 'participants',
    title: 'DM relay from Erick to Mina',
    instructions: 'Please review the launch checklist before 5 PM.',
    result_summary: 'Delivered DM relay to Mina',
    error_message: null,
    metadata: {
      context: 'This was about the release train.'
    },
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    completed_at: new Date().toISOString()
  };

  const service = new RetrievalService(
    {
      OPENAI_RESPONSE_MODEL: 'gpt-test'
    } as never,
    {
      async createTextResponse(input) {
        responseInputs.push(input);
        return 'Erick wanted you to review the launch checklist before 5 PM.';
      }
    },
    {
      async countPhrase() {
        return 0;
      },
      async listRecentMessages() {
        return [];
      },
      async searchSemantic() {
        return [];
      }
    } as never,
    {
      async listRelevantVisibleActionsForUser() {
        return [action];
      }
    } as never
  );

  const answer = await service.answerQuestion(
    'what did Erick want again?',
    {
      dmUserId: 'user-2',
      kind: 'dm'
    },
    'user-2',
    'bot-user'
  );

  assert.equal(answer.source, 'action');
  assert.match(answer.answer, /launch checklist/i);
  assert.match(responseInputs[0]?.text ?? '', /Recent shared actions/i);
  assert.match(responseInputs[0]?.text ?? '', /Erick/i);
});
