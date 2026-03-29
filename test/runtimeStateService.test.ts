import test from 'node:test';
import assert from 'node:assert/strict';

import { RuntimeStateService } from '../src/services/runtimeStateService.js';

function createIndexingStatus() {
  return {
    activeMessageId: null,
    indexedCount: 0,
    lastError: null,
    lastErrorAt: null,
    lastSuccessAt: null,
    pendingJobs: 0,
    processing: false,
    startedAt: new Date().toISOString()
  };
}

test('RuntimeStateService is ready only after Discord and command registration are healthy', () => {
  const service = new RuntimeStateService();

  assert.equal(service.getSnapshot(createIndexingStatus()).ready, false);

  service.markCommandRegistrationReady();
  assert.equal(service.getSnapshot(createIndexingStatus()).ready, false);

  service.markDiscordReady('bot-user-id');
  assert.equal(service.getSnapshot(createIndexingStatus()).ready, true);
});

test('RuntimeStateService treats skipped command registration as ready when Discord is ready', () => {
  const service = new RuntimeStateService();

  service.markCommandRegistrationSkipped();
  service.markDiscordReady('bot-user-id');

  const snapshot = service.getSnapshot(createIndexingStatus());
  assert.equal(snapshot.ready, true);
  assert.equal(snapshot.checks.commandRegistration.status, 'skipped');
});

test('RuntimeStateService exposes failed command registration as not ready', () => {
  const service = new RuntimeStateService();

  service.markDiscordReady('bot-user-id');
  service.markCommandRegistrationFailed('boom');

  const snapshot = service.getSnapshot(createIndexingStatus());
  assert.equal(snapshot.ready, false);
  assert.equal(snapshot.checks.commandRegistration.error, 'boom');
});
