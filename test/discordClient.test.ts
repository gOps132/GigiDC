import test from 'node:test';
import assert from 'node:assert/strict';

import { handleIncomingDiscordMessage } from '../src/discord/client.js';

function createHarness(overrides?: {
  dmConversationError?: Error | null;
  guildMention?: boolean;
  guildMentionError?: Error | null;
  dmConversationError?: Error | null;
  sensitiveWrite?: boolean;
  storageError?: Error | null;
  stored?: boolean;
  storeReason?: 'stored' | 'skipped_by_ingestion_policy' | 'unsupported_scope' | 'system_message';
}) {
  const handleCalls: string[] = [];
  const guildMentionCalls: string[] = [];
  const replies: string[] = [];
  const storageCalls: string[] = [];

  const context = {
    logger: {
      debug() {},
      error() {},
      info() {},
      warn() {}
    },
    services: {
      dmConversation: {
        async handleMessage(message: { content: string }) {
          handleCalls.push(message.content);
          if (overrides?.dmConversationError) {
            throw overrides.dmConversationError;
          }
        },
        async handleGuildMention(message: { content: string }) {
          guildMentionCalls.push(message.content);
          if (overrides?.guildMentionError) {
            throw overrides.guildMentionError;
          }
        },
        shouldHandleGuildMention() {
          return overrides?.guildMention ?? false;
        }
      },
      messageHistory: {
        async storeDiscordMessage() {
          storageCalls.push('store');
          if (overrides?.storageError) {
            throw overrides.storageError;
          }

          return {
            reason: overrides?.storeReason ?? 'stored',
            stored: overrides?.stored ?? true
          };
        }
      },
      sensitiveData: {
        shouldBypassHistoryStorage() {
          return overrides?.sensitiveWrite ?? false;
        }
      }
    }
  } as never;

  const message = {
    author: {
      bot: false
    },
    channel: {
      isDMBased() {
        return !overrides?.guildMention;
      }
    },
    channelId: 'dm-channel-1',
    content: 'hi',
    id: 'msg-1',
    inGuild() {
      return overrides?.guildMention ?? false;
    },
    async reply(payload: { content: string }) {
      replies.push(payload.content);
      return {} as never;
    }
  };

  return {
    context,
    guildMentionCalls,
    handleCalls,
    message,
    replies
    ,
    storageCalls
  };
}

test('handleIncomingDiscordMessage still processes a DM when message storage fails', async () => {
  const { context, handleCalls, message, replies } = createHarness({
    storageError: new Error('db unavailable')
  });

  await handleIncomingDiscordMessage(message as never, {} as never, context);

  assert.deepEqual(handleCalls, ['hi']);
  assert.deepEqual(replies, []);
});

test('handleIncomingDiscordMessage skips normal history storage for sensitive write attempts in DM', async () => {
  const { context, handleCalls, message, replies, storageCalls } = createHarness({
    sensitiveWrite: true
  });

  await handleIncomingDiscordMessage(message as never, {} as never, context);

  assert.deepEqual(storageCalls, []);
  assert.deepEqual(handleCalls, ['hi']);
  assert.deepEqual(replies, []);
});

test('handleIncomingDiscordMessage sends a fallback DM reply when DM handling fails', async () => {
  const { context, handleCalls, message, replies } = createHarness({
    dmConversationError: new Error('openai unavailable')
  });

  await handleIncomingDiscordMessage(message as never, {} as never, context);

  assert.deepEqual(handleCalls, ['hi']);
  assert.equal(replies[0], 'I hit an internal error while handling that DM. Try again in a moment.');
});

test('handleIncomingDiscordMessage processes guild mentions even when ingestion is disabled for the channel', async () => {
  const { context, guildMentionCalls, handleCalls, message, replies } = createHarness({
    guildMention: true,
    storeReason: 'skipped_by_ingestion_policy',
    stored: false
  });

  await handleIncomingDiscordMessage(message as never, { user: { id: 'bot-user' } } as never, context);

  assert.deepEqual(handleCalls, []);
  assert.deepEqual(guildMentionCalls, ['hi']);
  assert.deepEqual(replies, []);
});
