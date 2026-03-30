import test from 'node:test';
import assert from 'node:assert/strict';

import { ActionConfirmationService } from '../src/services/actionConfirmationService.js';
import {
  AGENT_ACTION_SCOPES,
  AGENT_ACTION_STATUSES,
  AGENT_ACTION_TYPES,
  type AgentActionRecord
} from '../src/services/agentActionService.js';

function createPendingRelayAction(): AgentActionRecord {
  return {
    id: 'action-1',
    action_scope: AGENT_ACTION_SCOPES.action,
    guild_id: 'guild-1',
    channel_id: 'dm-channel-1',
    requester_user_id: 'requester-1',
    requester_username: 'Erick',
    recipient_user_id: 'recipient-1',
    recipient_username: 'mina',
    action_type: AGENT_ACTION_TYPES.dmRelay,
    status: AGENT_ACTION_STATUSES.awaitingConfirmation,
    visibility: 'participants',
    title: 'DM relay from Erick to mina',
    instructions: 'Please review the checklist.',
    result_summary: null,
    error_message: null,
    metadata: {
      context: 'Tonight’s launch'
    },
    due_at: null,
    confirmation_requested_at: new Date(Date.now() - 1_000).toISOString(),
    confirmation_expires_at: new Date(Date.now() + 5 * 60 * 1_000).toISOString(),
    confirmed_at: null,
    confirmed_by_user_id: null,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    completed_at: null,
    cancelled_at: null
  };
}

function createService(overrides?: {
  dispatchAllowed?: boolean;
  pendingActions?: AgentActionRecord[];
  recipientAllowed?: boolean;
  sendError?: Error | null;
}) {
  const auditCalls: Array<Record<string, unknown>> = [];
  const completedCalls: Array<Record<string, unknown>> = [];
  const createdCalls: Array<Record<string, unknown>> = [];
  const failedCalls: Array<Record<string, unknown>> = [];
  const inProgressCalls: Array<Record<string, unknown>> = [];
  const cancelledCalls: Array<Record<string, unknown>> = [];
  const storedBotMessages: string[] = [];

  const pendingActions = overrides?.pendingActions ?? [createPendingRelayAction()];

  const service = new ActionConfirmationService(
    {
      DISCORD_GUILD_ID: 'guild-1',
      PRIMARY_GUILD_ID: 'guild-1'
    } as never,
    {
      async createDirectMessageRelay(input: Record<string, unknown>) {
        createdCalls.push(input);
        return {
          ...createPendingRelayAction(),
          confirmation_expires_at: input.confirmationExpiresAt ?? null,
          confirmation_requested_at: input.confirmationRequestedAt ?? null,
          metadata: input.metadata ?? {},
          recipient_user_id: input.recipientUserId,
          recipient_username: input.recipientUsername,
          requester_user_id: input.requesterUserId,
          requester_username: input.requesterUsername,
          status: input.initialStatus
        };
      },
      async getActionById(actionId: string) {
        return pendingActions.find((action) => action.id === actionId) ?? null;
      },
      async listPendingConfirmationsForRequester(userId: string) {
        return pendingActions.filter((action) => action.requester_user_id === userId);
      },
      async markCancelled(action: AgentActionRecord, input: Record<string, unknown>) {
        cancelledCalls.push({ action, input });
        return {
          ...action,
          cancelled_at: new Date().toISOString(),
          result_summary: input.resultSummary ?? null,
          status: AGENT_ACTION_STATUSES.cancelled
        };
      },
      async markCompleted(action: AgentActionRecord, input: Record<string, unknown>) {
        completedCalls.push({ action, input });
        return {
          ...action,
          completed_at: new Date().toISOString(),
          result_summary: input.resultSummary ?? null,
          status: AGENT_ACTION_STATUSES.completed
        };
      },
      async markFailed(action: AgentActionRecord, errorMessage: string) {
        failedCalls.push({ action, errorMessage });
        return {
          ...action,
          error_message: errorMessage,
          status: AGENT_ACTION_STATUSES.failed
        };
      },
      async markInProgress(action: AgentActionRecord, input: Record<string, unknown>) {
        inProgressCalls.push({ action, input });
        return {
          ...action,
          confirmed_at: new Date().toISOString(),
          confirmed_by_user_id: input.confirmedByUserId ?? null,
          status: AGENT_ACTION_STATUSES.inProgress
        };
      }
    } as never,
    {
      async record(input: Record<string, unknown>) {
        auditCalls.push(input);
      }
    } as never,
    {
      async storeBotAuthoredMessage(message: { id: string }) {
        storedBotMessages.push(message.id);
        return {
          reason: 'stored' as const,
          stored: true
        };
      }
    } as never,
    {
      async memberHasCapability(_guild: unknown, member: { id: string }, capability: string) {
        if (capability === 'agent_action_receive') {
          return member.id === 'recipient-1'
            ? overrides?.recipientAllowed ?? true
            : false;
        }

        return overrides?.dispatchAllowed ?? true;
      }
    } as never,
    {
      debug() {},
      error() {},
      info() {},
      warn() {}
    } as never
  );

  const client = {
    guilds: {
      cache: new Map([['guild-1', {
        id: 'guild-1',
        members: {
          async fetch(userId: string) {
            if (userId === 'recipient-1') {
              return {
                id: userId
              };
            }

            return {
              id: userId
            };
          }
        }
      }]]),
      async fetch() {
        return {
          id: 'guild-1',
          members: {
            async fetch(userId: string) {
              return {
                id: userId
              };
            }
          }
        };
      }
    },
    users: {
      async fetch() {
        return {
          username: 'mina',
          async send() {
            if (overrides?.sendError) {
              throw overrides.sendError;
            }

            return {
              author: {
                bot: true
              },
              channelId: 'dm-target-channel',
              id: 'sent-message-1'
            };
          }
        };
      }
    }
  };

  return {
    auditCalls,
    cancelledCalls,
    client,
    completedCalls,
    createdCalls,
    failedCalls,
    inProgressCalls,
    service,
    storedBotMessages
  };
}

test('ActionConfirmationService creates awaiting-confirmation relay actions with buttons', async () => {
  const { auditCalls, createdCalls, service } = createService();

  const result = await service.requestRelayConfirmation({
    channelId: 'dm-channel-1',
    context: 'Tonight’s launch',
    guildId: 'guild-1',
    message: 'Please review the checklist.',
    recipientUserId: 'recipient-1',
    recipientUsername: 'mina',
    requesterUserId: 'requester-1',
    requesterUsername: 'Erick'
  });

  assert.equal(createdCalls.length, 1);
  assert.equal(createdCalls[0]?.initialStatus, AGENT_ACTION_STATUSES.awaitingConfirmation);
  assert.ok(createdCalls[0]?.confirmationRequestedAt);
  assert.ok(createdCalls[0]?.confirmationExpiresAt);
  assert.match(result.reply, /confirm within 15 minutes/i);
  assert.equal(result.components.length, 1);
  assert.equal(auditCalls[0]?.action, 'agent_action.confirmation_requested');
});

test('ActionConfirmationService confirms and sends a pending relay from free text', async () => {
  const { auditCalls, client, completedCalls, inProgressCalls, service, storedBotMessages } = createService();

  const result = await service.maybeHandleTextConfirmation(
    'confirm!',
    {
      id: 'requester-1'
    } as never,
    client as never
  );

  assert.ok(result);
  assert.match(result.reply, /confirmed and sent a dm relay/i);
  assert.equal(inProgressCalls.length, 1);
  assert.equal(completedCalls.length, 1);
  assert.deepEqual(storedBotMessages, ['sent-message-1']);
  assert.equal(auditCalls[auditCalls.length - 1]?.action, 'agent_action.relay.sent');
});

test('ActionConfirmationService returns a clear no-op when nothing is pending', async () => {
  const { client, service } = createService({
    pendingActions: []
  });

  const result = await service.maybeHandleTextConfirmation(
    'confirm!',
    {
      id: 'requester-1'
    } as never,
    client as never
  );

  assert.ok(result);
  assert.match(result.reply, /nothing pending/i);
});
