import test from 'node:test';
import assert from 'node:assert/strict';

import { RetrievalService } from '../src/services/retrievalService.js';
import { AGENT_ACTION_SCOPES, AGENT_ACTION_STATUSES, AGENT_ACTION_TYPES, type AgentActionRecord } from '../src/services/agentActionService.js';

test('RetrievalService can answer from shared action history when message history is empty', async () => {
  const responseInputs: Array<{ instructions: string; model: string; text: string }> = [];
  const action: AgentActionRecord = {
    id: 'action-1',
    action_scope: AGENT_ACTION_SCOPES.action,
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
    due_at: null,
    confirmation_requested_at: null,
    confirmation_expires_at: null,
    confirmed_at: null,
    confirmed_by_user_id: null,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    completed_at: new Date().toISOString(),
    cancelled_at: null
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
      },
      async listOpenTasksForUser() {
        return [];
      }
    } as never,
    {
      async buildContext() {
        return [];
      }
    } as never,
    {
      warn() {},
      debug() {},
      error() {},
      info() {}
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

test('RetrievalService can answer from open tasks when the question is task-oriented', async () => {
  const responseInputs: Array<{ instructions: string; model: string; text: string }> = [];
  const task: AgentActionRecord = {
    id: 'task-1',
    action_scope: AGENT_ACTION_SCOPES.task,
    guild_id: 'guild-1',
    channel_id: 'channel-1',
    requester_user_id: 'manager-id',
    requester_username: 'Manager',
    recipient_user_id: 'user-2',
    recipient_username: 'Mina',
    action_type: AGENT_ACTION_TYPES.followUpTask,
    status: AGENT_ACTION_STATUSES.requested,
    visibility: 'participants',
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

  const service = new RetrievalService(
    {
      OPENAI_RESPONSE_MODEL: 'gpt-test'
    } as never,
    {
      async createTextResponse(input) {
        responseInputs.push(input);
        return 'You need to prepare the release notes before standup.';
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
        return [];
      },
      async listOpenTasksForUser() {
        return [task];
      }
    } as never,
    {
      async buildContext() {
        return [];
      }
    } as never,
    {
      warn() {},
      debug() {},
      error() {},
      info() {}
    } as never
  );

  const answer = await service.answerQuestion(
    'what tasks do i still have?',
    {
      dmUserId: 'user-2',
      kind: 'dm'
    },
    'user-2',
    'bot-user'
  );

  assert.equal(answer.source, 'action');
  assert.match(answer.answer, /release notes/i);
  assert.match(responseInputs[0]?.text ?? '', /Open tasks/i);
  assert.match(responseInputs[0]?.text ?? '', /Prepare release notes/i);
});

test('RetrievalService falls back to direct answering when semantic search fails', async () => {
  const warnings: Array<Record<string, unknown>> = [];
  const responseInputs: Array<{ instructions: string; model: string; text: string }> = [];

  const service = new RetrievalService(
    {
      OPENAI_RESPONSE_MODEL: 'gpt-test'
    } as never,
    {
      async createTextResponse(input) {
        responseInputs.push(input);
        return 'Hello.';
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
        throw new Error('embedding provider unavailable');
      }
    } as never,
    {
      async listRelevantVisibleActionsForUser() {
        return [];
      },
      async listOpenTasksForUser() {
        return [];
      }
    } as never,
    {
      async buildContext() {
        return [];
      }
    } as never,
    {
      warn(message: string, metadata: Record<string, unknown>) {
        warnings.push({
          message,
          ...metadata
        });
      },
      debug() {},
      error() {},
      info() {}
    } as never
  );

  const answer = await service.answerQuestion(
    'hi',
    {
      dmUserId: 'user-2',
      kind: 'dm'
    },
    'user-2',
    'bot-user'
  );

  assert.equal(answer.source, 'direct');
  assert.equal(answer.answer, 'Hello.');
  assert.equal(responseInputs.length, 1);
  assert.equal(warnings[0]?.message, 'Semantic search failed during retrieval');
});

test('RetrievalService includes requester-centric user-memory snapshots in the answer context', async () => {
  const responseInputs: Array<{ instructions: string; model: string; text: string }> = [];

  const service = new RetrievalService(
    {
      DISCORD_GUILD_ID: 'guild-1',
      OPENAI_RESPONSE_MODEL: 'gpt-test'
    } as never,
    {
      async createTextResponse(input) {
        responseInputs.push(input);
        return 'You usually ask me for release planning help.';
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
        return [];
      },
      async listOpenTasksForUser() {
        return [];
      }
    } as never,
    {
      async buildContext() {
        return [
          'User profile: username=erick | display_name=Erick',
          'Working context: Open work: Prepare release notes due 2026-04-01T09:00:00Z'
        ];
      }
    } as never,
    {
      warn() {},
      debug() {},
      error() {},
      info() {}
    } as never
  );

  const answer = await service.answerQuestion(
    'what am i working on lately?',
    {
      dmUserId: 'user-2',
      kind: 'dm'
    },
    'user-2',
    'bot-user'
  );

  assert.equal(answer.source, 'direct');
  assert.match(answer.answer, /release planning help/i);
  assert.match(responseInputs[0]?.text ?? '', /User memory/i);
  assert.match(responseInputs[0]?.text ?? '', /Working context/i);
});

test('RetrievalService returns a clean fallback when response generation fails', async () => {
  const errors: Array<Record<string, unknown>> = [];

  const service = new RetrievalService(
    {
      OPENAI_RESPONSE_MODEL: 'gpt-test'
    } as never,
    {
      async createTextResponse() {
        throw new Error('invalid_api_key');
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
        return [];
      },
      async listOpenTasksForUser() {
        return [];
      }
    } as never,
    {
      async buildContext() {
        return [];
      }
    } as never,
    {
      warn() {},
      debug() {},
      error(message: string, metadata: Record<string, unknown>) {
        errors.push({
          message,
          ...metadata
        });
      },
      info() {}
    } as never
  );

  const answer = await service.answerQuestion(
    'hi',
    {
      dmUserId: 'user-2',
      kind: 'dm'
    },
    'user-2',
    'bot-user'
  );

  assert.equal(answer.answer, 'I could not reach my reasoning backend right now. Try again in a moment.');
  assert.equal(answer.source, 'direct');
  assert.equal(errors[0]?.message, 'OpenAI text response failed during retrieval');
});
