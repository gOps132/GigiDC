import test from 'node:test';
import assert from 'node:assert/strict';

import type { BotContext } from '../src/discord/types.js';
import type {
  PendingDmScopeSelection,
  PendingDmScopeSelectionStore
} from '../src/ports/conversation.js';
import { DmConversationService } from '../src/services/dmConversationService.js';

class InMemoryPendingDmScopeSelectionStore implements PendingDmScopeSelectionStore {
  readonly deletedIds: string[] = [];
  readonly saved: PendingDmScopeSelection[] = [];
  private readonly selections = new Map<string, PendingDmScopeSelection>();

  async delete(selectionId: string): Promise<void> {
    this.deletedIds.push(selectionId);
    this.selections.delete(selectionId);
  }

  async deleteExpired(_now: Date): Promise<void> {
    return;
  }

  async get(selectionId: string): Promise<PendingDmScopeSelection | null> {
    return this.selections.get(selectionId) ?? null;
  }

  async save(selection: PendingDmScopeSelection, _expiresAt: Date): Promise<void> {
    this.saved.push(selection);
    this.selections.set(selection.id, selection);
  }
}

function createContext(overrides?: {
  confirmationReply?: string | null;
  allowGuildWideHistory?: boolean;
  botReplyId?: string;
  guildName?: string;
  outboundStoreError?: Error | null;
  primaryGuildId?: string;
  retrievalAnswer?: string;
  toolReply?: string | null;
}) {
  const answerCalls: Array<{
    botUserId: string;
    query: string;
    requesterUserId: string;
    scope: unknown;
  }> = [];
  const toolCalls: Array<{ channelId: string; query: string; requesterUserId: string }> = [];
  const replyCalls: Array<{ components?: unknown[]; content: string }> = [];
  const storedBotMessages: string[] = [];
  const updateCalls: Array<{ components?: unknown[]; content: string }> = [];
  const followUpCalls: Array<{ content: string }> = [];

  const guildId = overrides?.primaryGuildId ?? 'guild-1';
  const guild = {
    id: guildId,
    name: overrides?.guildName ?? 'Gigi HQ',
    members: {
      fetch: async (_userId: string) => ({})
    }
  };

  const context = {
    env: {
      DISCORD_GUILD_ID: overrides?.primaryGuildId,
      PRIMARY_GUILD_ID: overrides?.primaryGuildId
    },
    logger: {
      debug() {},
      error() {},
      info() {},
      warn() {}
    },
    runtime: {},
    services: {
      actionConfirmations: {
        async handleButton() {
          return;
        },
        matches() {
          return false;
        },
        async maybeHandleTextConfirmation() {
          if (!overrides?.confirmationReply) {
            return null;
          }

          return {
            reply: overrides.confirmationReply
          };
        }
      },
      agentActions: {},
      agentTools: {
        async maybeHandleDmQuery(query: string, requester: { id: string }, _client: unknown, channelId: string) {
          toolCalls.push({
            channelId,
            query,
            requesterUserId: requester.id
          });

          if (!overrides?.toolReply) {
            return null;
          }

          return {
            executedToolNames: ['list_open_tasks'],
            reply: overrides.toolReply
          };
        }
      },
      assignments: {},
      auditLogs: {},
      channelIngestionPolicies: {},
      dmConversation: {},
      messageHistory: {
        async storeBotAuthoredMessage(message: { id: string }) {
          if (overrides?.outboundStoreError) {
            throw overrides.outboundStoreError;
          }

          storedBotMessages.push(message.id);
          return {
            reason: 'stored' as const,
            stored: true
          };
        }
      },
      messageIndexing: {},
      permissionAdmin: {},
      retrieval: {
        async answerQuestion(query: string, scope: unknown, requesterUserId: string, botUserId: string) {
          answerCalls.push({
            botUserId,
            query,
            requesterUserId,
            scope
          });

          return {
            answer: overrides?.retrievalAnswer ?? 'retrieved answer',
            source: 'semantic' as const
          };
        }
      },
      rolePolicies: {
        async memberHasCapability() {
          return overrides?.allowGuildWideHistory ?? false;
        }
      },
      sensitiveData: {
        async maybeHandleDmQuery() {
          return null;
        }
      },
      userMemory: {
        async syncProfile() {
          return;
        }
      }
    }
  } as unknown as BotContext;

  const client = {
    guilds: {
      cache: new Map([[guildId, guild]]),
      async fetch() {
        return guild;
      }
    },
    user: {
      id: 'bot-user'
    }
  };

  const message = {
    author: {
      bot: false,
      id: 'user-1'
    },
    channel: {
      isDMBased: () => true
    },
    content: 'What did we talk about yesterday?',
    async reply(payload: { components?: unknown[]; content: string }) {
      replyCalls.push(payload);
      return {
        author: {
          bot: true
        },
        id: overrides?.botReplyId ?? `bot-reply-${replyCalls.length}`
      };
    }
  };

  const interaction = {
    customId: '',
    user: {
      id: 'user-1'
    },
    values: ['guild:guild-1'],
    async followUp(payload: { content: string }) {
      followUpCalls.push(payload);
      return {
        author: {
          bot: true
        },
        id: `bot-follow-up-${followUpCalls.length}`
      };
    },
    async reply(payload: { content: string }) {
      followUpCalls.push(payload);
    },
    async update(payload: { components?: unknown[]; content: string }) {
      updateCalls.push(payload);
    }
  };

  return {
    answerCalls,
    client,
    context,
    followUpCalls,
    interaction,
    message,
    replyCalls,
    storedBotMessages,
    toolCalls,
    updateCalls
  };
}

test('DmConversationService persists scope selection prompts instead of keeping them in memory', async () => {
  const store = new InMemoryPendingDmScopeSelectionStore();
  const { client, context, message, replyCalls, storedBotMessages } = createContext({
    allowGuildWideHistory: true,
    primaryGuildId: 'guild-1'
  });
  const service = new DmConversationService(context, store);

  await service.handleMessage(message as never, client as never);

  assert.equal(store.saved.length, 1);
  assert.equal(store.saved[0]?.userId, 'user-1');
  assert.equal(store.saved[0]?.scopeOptions.length, 2);
  assert.equal(replyCalls.length, 1);
  assert.deepEqual(storedBotMessages, ['bot-reply-1']);
  assert.match(replyCalls[0]?.content ?? '', /pick which chat history/i);
});

test('DmConversationService reads persisted scope selections when the user chooses a scope', async () => {
  const store = new InMemoryPendingDmScopeSelectionStore();
  const selection: PendingDmScopeSelection = {
    createdAt: Date.now(),
    id: 'selection-1',
    query: 'What did we talk about yesterday?',
    scopeOptions: [
      {
        label: 'This DM',
        scope: {
          dmUserId: 'user-1',
          kind: 'dm'
        },
        value: 'dm'
      },
      {
        label: 'Gigi HQ server',
        scope: {
          guildId: 'guild-1',
          kind: 'guild'
        },
        value: 'guild:guild-1'
      }
    ],
    userId: 'user-1'
  };
  await store.save(selection, new Date(Date.now() + 60_000));

  const { answerCalls, client, context, followUpCalls, interaction, storedBotMessages, updateCalls } = createContext({
    allowGuildWideHistory: true,
    primaryGuildId: 'guild-1',
    retrievalAnswer: 'Here is the stored answer'
  });
  interaction.customId = 'dm-scope:selection-1';

  const service = new DmConversationService(context, store);
  await service.handleSelection(interaction as never, client as never);

  assert.deepEqual(store.deletedIds, ['selection-1']);
  assert.equal(answerCalls.length, 1);
  assert.equal(answerCalls[0]?.query, selection.query);
  assert.deepEqual(answerCalls[0]?.scope, selection.scopeOptions[1]?.scope);
  assert.match(updateCalls[0]?.content ?? '', /using gigi hq server/i);
  assert.equal(followUpCalls[0]?.content, 'Here is the stored answer');
  assert.deepEqual(storedBotMessages, ['bot-follow-up-1']);
});

test('DmConversationService persists direct bot-authored DM replies in canonical history', async () => {
  const store = new InMemoryPendingDmScopeSelectionStore();
  const { answerCalls, client, context, message, replyCalls, storedBotMessages } = createContext({
    allowGuildWideHistory: false,
    retrievalAnswer: 'Here is the direct answer'
  });
  const service = new DmConversationService(context, store);

  await service.handleMessage(message as never, client as never);

  assert.equal(answerCalls.length, 1);
  assert.equal(replyCalls[0]?.content, 'Here is the direct answer');
  assert.deepEqual(storedBotMessages, ['bot-reply-1']);
});

test('DmConversationService routes tool-style DM requests through the tool service before retrieval', async () => {
  const store = new InMemoryPendingDmScopeSelectionStore();
  const { answerCalls, client, context, message, replyCalls, storedBotMessages, toolCalls } = createContext({
    retrievalAnswer: 'retrieval fallback',
    toolReply: 'Created task `task-1` and listed your open tasks.'
  });
  message.content = 'Create a task for me to review the launch notes and show me my tasks.';

  const service = new DmConversationService(context, store);
  await service.handleMessage(message as never, client as never);

  assert.equal(toolCalls.length, 1);
  assert.equal(answerCalls.length, 0);
  assert.equal(replyCalls[0]?.content, 'Created task `task-1` and listed your open tasks.');
  assert.deepEqual(storedBotMessages, ['bot-reply-1']);
});

test('DmConversationService handles free-text confirmations through the confirmation service before retrieval', async () => {
  const store = new InMemoryPendingDmScopeSelectionStore();
  const { answerCalls, client, context, message, replyCalls, storedBotMessages, toolCalls } = createContext({
    confirmationReply: 'Confirmed and sent a DM relay to mina.'
  });
  message.content = 'confirm!';

  const service = new DmConversationService(context, store);
  await service.handleMessage(message as never, client as never);

  assert.equal(toolCalls.length, 0);
  assert.equal(answerCalls.length, 0);
  assert.equal(replyCalls[0]?.content, 'Confirmed and sent a DM relay to mina.');
  assert.deepEqual(storedBotMessages, ['bot-reply-1']);
});

test('DmConversationService answers deterministic capability questions before retrieval', async () => {
  const store = new InMemoryPendingDmScopeSelectionStore();
  const { answerCalls, client, context, message, replyCalls } = createContext();
  message.content = 'what tools can you call?';

  const service = new DmConversationService(context, store);
  await service.handleMessage(message as never, client as never);

  assert.equal(answerCalls.length, 0);
  assert.match(replyCalls[0]?.content ?? '', /I cannot browse the web/i);
  assert.match(replyCalls[0]?.content ?? '', /request and confirm permission-gated Gigi-mediated DMs/i);
});

test('DmConversationService answers guild mentions from the current channel scope without user memory or task memory', async () => {
  const store = new InMemoryPendingDmScopeSelectionStore();
  const { answerCalls, client, context, message, replyCalls, storedBotMessages, toolCalls } = createContext({
    retrievalAnswer: 'Channel answer'
  });
  const service = new DmConversationService(context, store);

  const guildMessage = {
    ...message,
    author: {
      ...message.author,
      globalName: 'Gops'
    },
    channel: {
      isDMBased: () => false
    },
    channelId: 'channel-9',
    content: '<@bot-user> what were we talking about?',
    guildId: 'guild-1',
    inGuild: () => true,
    member: {
      displayName: 'Gops'
    },
    mentions: {
      users: {
        has(userId: string) {
          return userId === 'bot-user';
        }
      }
    }
  };

  await service.handleGuildMention(guildMessage as never, client as never);

  assert.equal(toolCalls.length, 0);
  assert.equal(answerCalls.length, 1);
  assert.deepEqual(answerCalls[0]?.scope, {
    kind: 'guild',
    guildId: 'guild-1',
    channelId: 'channel-9'
  });
  assert.equal(replyCalls[0]?.content, 'Channel answer');
  assert.deepEqual(storedBotMessages, ['bot-reply-1']);
});

test('DmConversationService redirects guild mention tool requests back to DM or slash commands', async () => {
  const store = new InMemoryPendingDmScopeSelectionStore();
  const { answerCalls, client, context, message, replyCalls, toolCalls } = createContext();
  const service = new DmConversationService(context, store);

  const guildMessage = {
    ...message,
    channel: {
      isDMBased: () => false
    },
    channelId: 'channel-9',
    content: '<@bot-user> create a task for me',
    guildId: 'guild-1',
    inGuild: () => true,
    member: {
      displayName: 'Gops'
    },
    mentions: {
      users: {
        has(userId: string) {
          return userId === 'bot-user';
        }
      }
    }
  };

  await service.handleGuildMention(guildMessage as never, client as never);

  assert.equal(toolCalls.length, 0);
  assert.equal(answerCalls.length, 0);
  assert.match(replyCalls[0]?.content ?? '', /DM me or use the matching slash command/i);
});
