import type { User } from 'discord.js';

import type { Logger } from '../lib/logger.js';
import {
  USER_MEMORY_SNAPSHOT_KINDS,
  type UserMemorySnapshotKind,
  type UserMemorySnapshotRecord,
  type UserMemorySnapshotStore,
  type UserProfileStore
} from '../ports/identity.js';
import { AGENT_ACTION_TYPES, type AgentActionRecord, type AgentActionService } from './agentActionService.js';
import type { HistoryMessageRecord, MessageHistoryService } from './messageHistoryService.js';

const SNAPSHOT_TTL_MS = 6 * 60 * 60 * 1000;
const SNAPSHOT_KIND_ORDER: UserMemorySnapshotKind[] = [
  USER_MEMORY_SNAPSHOT_KINDS.identitySummary,
  USER_MEMORY_SNAPSHOT_KINDS.workingContext,
  USER_MEMORY_SNAPSHOT_KINDS.preferences
];

export class UserMemoryService {
  constructor(
    private readonly profiles: UserProfileStore,
    private readonly snapshots: UserMemorySnapshotStore,
    private readonly messageHistory: MessageHistoryService,
    private readonly agentActions: AgentActionService,
    private readonly logger: Logger
  ) {}

  async syncProfile(input: {
    displayName?: string | null;
    guildId: string | null;
    user: User;
  }): Promise<void> {
    if (!input.guildId) {
      return;
    }

    await this.profiles.upsertProfile({
      displayName: input.displayName ?? null,
      globalName: input.user.globalName ?? null,
      guildId: input.guildId,
      observedAt: new Date().toISOString(),
      userId: input.user.id,
      username: input.user.username
    });
  }

  async buildContext(userId: string, guildId: string | null): Promise<string[]> {
    if (!guildId) {
      return [];
    }

    try {
      const profile = await this.profiles.getProfile(guildId, userId);
      const snapshots = await this.ensureSnapshots(guildId, userId);
      const lines: string[] = [];

      if (profile) {
        const profileParts = [
          `username=${profile.username}`,
          profile.display_name ? `display_name=${profile.display_name}` : null,
          profile.global_name ? `global_name=${profile.global_name}` : null,
          `first_seen=${new Date(profile.first_seen_at).toISOString()}`,
          `last_seen=${new Date(profile.last_seen_at).toISOString()}`
        ].filter((part): part is string => Boolean(part));

        lines.push(`User profile: ${profileParts.join(' | ')}`);
      }

      for (const kind of SNAPSHOT_KIND_ORDER) {
        const snapshot = snapshots.find((entry) => entry.snapshot_kind === kind);
        if (!snapshot) {
          continue;
        }

        lines.push(`${formatSnapshotKind(kind)}: ${snapshot.summary_text}`);
      }

      return lines;
    } catch (error) {
      this.logger.warn('Failed to build user-memory context', {
        error: error instanceof Error ? error.message : 'Unknown user-memory error',
        guildId,
        userId
      });
      return [];
    }
  }

  private async ensureSnapshots(
    guildId: string,
    userId: string
  ): Promise<UserMemorySnapshotRecord[]> {
    const existing = await this.snapshots.listSnapshotsForUser(guildId, userId);
    const validByKind = new Map<UserMemorySnapshotKind, UserMemorySnapshotRecord>();

    for (const snapshot of existing) {
      if (!snapshot.expires_at || Date.now() < Date.parse(snapshot.expires_at)) {
        validByKind.set(snapshot.snapshot_kind, snapshot);
      }
    }

    const missingKinds = SNAPSHOT_KIND_ORDER.filter((kind) => !validByKind.has(kind));
    if (missingKinds.length === 0) {
      return SNAPSHOT_KIND_ORDER
        .map((kind) => validByKind.get(kind))
        .filter((snapshot): snapshot is UserMemorySnapshotRecord => Boolean(snapshot));
    }

    const recentMessages = await this.messageHistory.listRecentMessages(
      {
        kind: 'dm',
        dmUserId: userId
      },
      12
    );
    const visibleActions = await this.agentActions.listRelevantVisibleActionsForUser(userId, '', 6);
    const openTasks = await this.agentActions.listOpenTasksForUser(userId, 4);
    const generatedAt = new Date();
    const expiresAt = new Date(generatedAt.getTime() + SNAPSHOT_TTL_MS).toISOString();
    const freshSnapshots = await Promise.all(
      missingKinds.map(async (kind) => {
        const snapshot = buildSnapshot(kind, recentMessages, visibleActions, openTasks);
        return this.snapshots.upsertSnapshot({
          expiresAt,
          generatedAt: generatedAt.toISOString(),
          guildId,
          snapshotKind: kind,
          sourceActionIds: snapshot.sourceActionIds,
          sourceMessageIds: snapshot.sourceMessageIds,
          summaryText: snapshot.summaryText,
          userId
        });
      })
    );

    return [
      ...existing.filter((snapshot) => validByKind.has(snapshot.snapshot_kind)),
      ...freshSnapshots
    ];
  }
}

function buildSnapshot(
  kind: UserMemorySnapshotKind,
  recentMessages: HistoryMessageRecord[],
  visibleActions: AgentActionRecord[],
  openTasks: AgentActionRecord[]
): {
  sourceActionIds: string[];
  sourceMessageIds: string[];
  summaryText: string;
} {
  if (kind === USER_MEMORY_SNAPSHOT_KINDS.identitySummary) {
    const recentUserMessages = recentMessages
      .filter((message) => !message.author_is_bot)
      .slice(-3);

    const recentTopics = recentUserMessages
      .map((message) => truncateText(message.content))
      .filter((content) => content.length > 0);

    return {
      sourceActionIds: visibleActions.slice(0, 2).map((action) => action.id),
      sourceMessageIds: recentUserMessages.map((message) => message.id),
      summaryText: recentTopics.length > 0
        ? `Recent self-expressed context: ${recentTopics.join(' | ')}`
        : 'No recent self-expressed DM context has been captured yet.'
    };
  }

  if (kind === USER_MEMORY_SNAPSHOT_KINDS.workingContext) {
    const taskLines = openTasks.slice(0, 3).map((task) => {
      const dueText = task.due_at ? ` due ${new Date(task.due_at).toISOString()}` : '';
      return `${task.title}${dueText}`;
    });
    const relayLine = visibleActions
      .filter((action) => action.action_type === AGENT_ACTION_TYPES.dmRelay)
      .slice(0, 2)
      .map((action) => `${action.requester_username} -> ${action.recipient_username ?? 'unknown'} (${action.status})`);

    const summaryParts = [
      taskLines.length > 0
        ? `Open work: ${taskLines.join('; ')}`
        : 'Open work: none recorded.',
      relayLine.length > 0
        ? `Recent relay context: ${relayLine.join('; ')}`
        : null
    ].filter((part): part is string => Boolean(part));

    return {
      sourceActionIds: [
        ...openTasks.slice(0, 3).map((task) => task.id),
        ...visibleActions
          .filter((action) => action.action_type === AGENT_ACTION_TYPES.dmRelay)
          .slice(0, 2)
          .map((action) => action.id)
      ],
      sourceMessageIds: [],
      summaryText: summaryParts.join(' ')
    };
  }

  const preferenceMessages = recentMessages
    .filter((message) => !message.author_is_bot)
    .filter((message) => looksLikePreferenceSignal(message.content))
    .slice(-4);

  return {
    sourceActionIds: [],
    sourceMessageIds: preferenceMessages.map((message) => message.id),
    summaryText: preferenceMessages.length > 0
      ? `Observed preference signals: ${preferenceMessages.map((message) => truncateText(message.content)).join(' | ')}`
      : 'No stable preferences have been inferred from recent DM history yet.'
  };
}

function looksLikePreferenceSignal(content: string): boolean {
  return /\b(i like|i prefer|prefer|call me|i want|i don.?t want|don.?t call me|my favorite|i love|i hate)\b/i.test(
    content
  );
}

function truncateText(content: string): string {
  const normalized = content.trim().replace(/\s+/g, ' ');
  return normalized.length > 140 ? `${normalized.slice(0, 137)}...` : normalized;
}

function formatSnapshotKind(kind: UserMemorySnapshotKind): string {
  if (kind === USER_MEMORY_SNAPSHOT_KINDS.identitySummary) {
    return 'Identity summary';
  }

  if (kind === USER_MEMORY_SNAPSHOT_KINDS.workingContext) {
    return 'Working context';
  }

  return 'Preferences';
}
