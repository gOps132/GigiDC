import test from 'node:test';
import assert from 'node:assert/strict';

import {
  USER_MEMORY_SNAPSHOT_KINDS,
  type UserMemorySnapshotRecord,
  type UserProfileRecord
} from '../src/ports/identity.js';
import { AGENT_ACTION_SCOPES, AGENT_ACTION_STATUSES, AGENT_ACTION_TYPES } from '../src/services/agentActionService.js';
import { UserMemoryService } from '../src/services/userMemoryService.js';

class InMemoryUserProfileStore {
  profiles = new Map<string, UserProfileRecord>();

  async getProfile(guildId: string, userId: string): Promise<UserProfileRecord | null> {
    return this.profiles.get(`${guildId}:${userId}`) ?? null;
  }

  async upsertProfile(input: {
    displayName?: string | null;
    globalName?: string | null;
    guildId: string;
    observedAt: string;
    userId: string;
    username: string;
  }): Promise<UserProfileRecord> {
    const key = `${input.guildId}:${input.userId}`;
    const existing = this.profiles.get(key);
    const record: UserProfileRecord = {
      created_at: existing?.created_at ?? input.observedAt,
      display_name: input.displayName ?? null,
      first_seen_at: existing?.first_seen_at ?? input.observedAt,
      global_name: input.globalName ?? null,
      guild_id: input.guildId,
      last_seen_at: input.observedAt,
      updated_at: input.observedAt,
      user_id: input.userId,
      username: input.username
    };
    this.profiles.set(key, record);
    return record;
  }
}

class InMemoryUserMemorySnapshotStore {
  snapshots = new Map<string, UserMemorySnapshotRecord>();

  async listSnapshotsForUser(guildId: string, userId: string): Promise<UserMemorySnapshotRecord[]> {
    return [...this.snapshots.values()].filter((snapshot) =>
      snapshot.guild_id === guildId && snapshot.user_id === userId
    );
  }

  async upsertSnapshot(input: {
    expiresAt?: string | null;
    generatedAt: string;
    guildId: string;
    snapshotKind: UserMemorySnapshotRecord['snapshot_kind'];
    sourceActionIds: string[];
    sourceMessageIds: string[];
    summaryText: string;
    userId: string;
  }): Promise<UserMemorySnapshotRecord> {
    const key = `${input.guildId}:${input.userId}:${input.snapshotKind}`;
    const record: UserMemorySnapshotRecord = {
      created_at: input.generatedAt,
      expires_at: input.expiresAt ?? null,
      generated_at: input.generatedAt,
      guild_id: input.guildId,
      id: key,
      snapshot_kind: input.snapshotKind,
      source_action_ids: input.sourceActionIds,
      source_message_ids: input.sourceMessageIds,
      summary_text: input.summaryText,
      updated_at: input.generatedAt,
      user_id: input.userId
    };
    this.snapshots.set(key, record);
    return record;
  }
}

test('UserMemoryService syncs requester profiles and generates bounded snapshots with traceable sources', async () => {
  const profiles = new InMemoryUserProfileStore();
  const snapshots = new InMemoryUserMemorySnapshotStore();
  const service = new UserMemoryService(
    profiles as never,
    snapshots as never,
    {
      async listRecentMessages() {
        return [
          {
            id: 'message-1',
            guild_id: null,
            channel_id: 'dm-1',
            thread_id: null,
            dm_user_id: 'user-1',
            author_user_id: 'user-1',
            author_username: 'erick',
            author_is_bot: false,
            content: 'I prefer short standups and I want to finish release notes today.',
            created_at: new Date().toISOString()
          },
          {
            id: 'message-2',
            guild_id: null,
            channel_id: 'dm-1',
            thread_id: null,
            dm_user_id: 'user-1',
            author_user_id: 'bot-1',
            author_username: 'Gigi',
            author_is_bot: true,
            content: 'Noted.',
            created_at: new Date().toISOString()
          }
        ];
      }
    } as never,
    {
      async listRelevantVisibleActionsForUser() {
        return [
          {
            id: 'action-1',
            action_scope: AGENT_ACTION_SCOPES.action,
            guild_id: 'guild-1',
            channel_id: 'channel-1',
            requester_user_id: 'manager-1',
            requester_username: 'Manager',
            recipient_user_id: 'user-1',
            recipient_username: 'erick',
            action_type: AGENT_ACTION_TYPES.dmRelay,
            status: AGENT_ACTION_STATUSES.completed,
            visibility: 'participants',
            title: 'DM relay from Manager to erick',
            instructions: 'Please review the checklist.',
            result_summary: 'Delivered DM relay to erick',
            error_message: null,
            metadata: {},
            due_at: null,
            confirmation_requested_at: null,
            confirmation_expires_at: null,
            confirmed_at: null,
            confirmed_by_user_id: null,
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
            completed_at: new Date().toISOString(),
            cancelled_at: null
          }
        ];
      },
      async listOpenTasksForUser() {
        return [
          {
            id: 'task-1',
            action_scope: AGENT_ACTION_SCOPES.task,
            guild_id: 'guild-1',
            channel_id: 'channel-1',
            requester_user_id: 'manager-1',
            requester_username: 'Manager',
            recipient_user_id: 'user-1',
            recipient_username: 'erick',
            action_type: AGENT_ACTION_TYPES.followUpTask,
            status: AGENT_ACTION_STATUSES.requested,
            visibility: 'participants',
            title: 'Prepare release notes',
            instructions: 'Finish the release notes today.',
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
          }
        ];
      }
    } as never,
    {
      debug() {},
      error() {},
      info() {},
      warn() {}
    } as never
  );

  await service.syncProfile({
    displayName: 'Erick',
    guildId: 'guild-1',
    user: {
      globalName: 'Erick G.',
      id: 'user-1',
      username: 'erick'
    } as never
  });

  const context = await service.buildContext('user-1', 'guild-1');
  const storedSnapshots = await snapshots.listSnapshotsForUser('guild-1', 'user-1');

  assert.equal(context.length, 4);
  assert.match(context[0] ?? '', /display_name=Erick/);
  assert.match(context.join('\n'), /Open work: Prepare release notes/i);
  assert.match(context.join('\n'), /Observed preference signals/i);
  assert.deepEqual(
    storedSnapshots.map((snapshot) => snapshot.snapshot_kind).sort(),
    [
      USER_MEMORY_SNAPSHOT_KINDS.identitySummary,
      USER_MEMORY_SNAPSHOT_KINDS.preferences,
      USER_MEMORY_SNAPSHOT_KINDS.workingContext
    ].sort()
  );
  assert.deepEqual(
    storedSnapshots.find((snapshot) => snapshot.snapshot_kind === USER_MEMORY_SNAPSHOT_KINDS.preferences)?.source_message_ids,
    ['message-1']
  );
  assert.deepEqual(
    storedSnapshots.find((snapshot) => snapshot.snapshot_kind === USER_MEMORY_SNAPSHOT_KINDS.workingContext)?.source_action_ids,
    ['task-1', 'action-1']
  );
});
