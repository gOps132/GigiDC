import test from 'node:test';
import assert from 'node:assert/strict';

import { MessageIndexingService } from '../src/services/messageIndexingService.js';

test('MessageIndexingService records model usage for background embeddings', async () => {
  const usageCalls: Array<Record<string, unknown>> = [];
  const embeddingWrites: Array<Record<string, unknown>> = [];

  const service = new MessageIndexingService(
    {
      OPENAI_EMBEDDING_MODEL: 'text-embedding-test'
    } as never,
    {
      async createEmbedding() {
        return {
          usage: {
            inputTokens: 44,
            outputTokens: null,
            totalTokens: 44
          },
          vector: [0.1, 0.2, 0.3]
        };
      }
    } as never,
    {
      async upsertMessageEmbedding(input: Record<string, unknown>) {
        embeddingWrites.push(input);
      }
    } as never,
    {
      async record(input: Record<string, unknown>) {
        usageCalls.push(input);
      }
    } as never,
    {
      debug() {},
      error() {},
      info() {},
      warn() {}
    } as never
  );

  service.enqueue({
    content: 'hello world',
    messageId: 'msg-1'
  });

  await new Promise((resolve) => setTimeout(resolve, 0));

  assert.equal(usageCalls.length, 1);
  assert.equal(usageCalls[0]?.operation, 'message_index_embedding');
  assert.equal(usageCalls[0]?.model, 'text-embedding-test');
  assert.equal(usageCalls[0]?.messageId, 'msg-1');
  assert.equal(embeddingWrites.length, 1);
});
