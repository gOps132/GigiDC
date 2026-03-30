import test from 'node:test';
import assert from 'node:assert/strict';

import { ChannelType } from 'discord.js';

import { Logger } from '../src/lib/logger.js';
import { GuildAdminActionService } from '../src/services/guildAdminActionService.js';
import { CAPABILITIES } from '../src/services/rolePolicyService.js';

function createService(overrides?: {
  assignmentAllowed?: boolean;
  assignmentById?: Record<string, unknown> | null;
  assignments?: Array<Record<string, unknown>>;
  channelPolicy?: {
    enabled: boolean;
    updatedAt?: string;
    updatedByUserId?: string;
  } | null;
  channelReferenceName?: string;
  createAssignmentResult?: Record<string, unknown>;
  ingestionAllowed?: boolean;
  roleName?: string;
}) {
  const auditCalls: Array<Record<string, unknown>> = [];
  const createAssignmentCalls: Array<Record<string, unknown>> = [];
  const markPublishedCalls: Array<Record<string, unknown>> = [];
  const sendCalls: Array<Record<string, unknown>> = [];
  const setIngestionCalls: Array<Record<string, unknown>> = [];

  const guildChannel = {
    id: 'channel-1',
    name: overrides?.channelReferenceName ?? 'shipping',
    type: ChannelType.GuildText,
    async send(payload: Record<string, unknown>) {
      sendCalls.push(payload);
      return {
        id: 'message-1'
      };
    }
  };

  const assignment = {
    id: 'assignment-1',
    guild_id: 'guild-1',
    title: 'Release notes',
    description: 'Draft and publish release notes.',
    due_at: '2026-04-02T09:00:00Z',
    announcement_channel_id: guildChannel.id,
    mentioned_role_ids: ['role-1'],
    created_by_user_id: 'requester-1',
    published_message_id: null,
    status: 'draft',
    created_at: '2026-03-30T00:00:00.000Z',
    updated_at: '2026-03-30T00:00:00.000Z'
  };

  const guild = {
    channels: {
      async fetch(channelId?: string) {
        if (channelId) {
          return channelId === guildChannel.id ? guildChannel : null;
        }

        return new Map([[guildChannel.id, guildChannel]]);
      }
    },
    id: 'guild-1',
    members: {
      async fetch(userId: string) {
        return {
          displayName: userId === 'requester-1' ? 'Erick' : 'Someone',
          id: userId
        };
      }
    },
    roles: {
      async fetch() {
        return new Map([[
          'role-1',
          {
            id: 'role-1',
            name: overrides?.roleName ?? 'Writers'
          }
        ]]);
      }
    }
  };

  const client = {
    guilds: {
      cache: new Map([['guild-1', guild]]),
      async fetch() {
        return guild;
      }
    }
  };

  const service = new GuildAdminActionService(
    {
      DISCORD_GUILD_ID: 'guild-1'
    } as never,
    {
      async createAssignment(input: Record<string, unknown>) {
        createAssignmentCalls.push(input);
        return (overrides?.createAssignmentResult as never) ?? {
          ...assignment,
          announcement_channel_id: input.announcementChannelId ?? null,
          created_by_user_id: input.createdByUserId,
          description: input.description,
          due_at: input.dueAt ?? null,
          mentioned_role_ids: input.mentionedRoleIds,
          title: input.title
        };
      },
      async getAssignmentById(_guildId: string, assignmentId: string) {
        if (assignmentId === assignment.id) {
          return (overrides?.assignmentById as never) ?? assignment;
        }

        return null;
      },
      async listAssignments() {
        return (overrides?.assignments as never) ?? [assignment];
      },
      async markPublished(guildId: string, assignmentId: string, messageId: string, channelId: string) {
        markPublishedCalls.push({
          assignmentId,
          channelId,
          guildId,
          messageId
        });

        return {
          ...assignment,
          announcement_channel_id: channelId,
          id: assignmentId,
          published_message_id: messageId,
          status: 'published'
        };
      }
    } as never,
    {
      async record(input: Record<string, unknown>) {
        auditCalls.push(input);
      }
    } as never,
    {
      async getPolicy(guildId: string, channelId: string) {
        if (!overrides?.channelPolicy) {
          return null;
        }

        return {
          channel_id: channelId,
          created_at: '2026-03-30T00:00:00.000Z',
          enabled: overrides.channelPolicy.enabled,
          guild_id: guildId,
          id: `${guildId}:${channelId}`,
          updated_at: overrides.channelPolicy.updatedAt ?? '2026-03-30T00:00:00.000Z',
          updated_by_user_id: overrides.channelPolicy.updatedByUserId ?? 'requester-1'
        };
      },
      async setChannelEnabled(input: Record<string, unknown>) {
        setIngestionCalls.push(input);
        return {
          channel_id: input.channelId,
          created_at: '2026-03-30T00:00:00.000Z',
          enabled: input.enabled,
          guild_id: input.guildId,
          id: `${input.guildId}:${input.channelId}`,
          updated_at: '2026-03-30T00:00:00.000Z',
          updated_by_user_id: input.updatedByUserId
        };
      }
    } as never,
    {
      async memberHasCapability(_guild: unknown, _member: unknown, capability: string) {
        if (capability === CAPABILITIES.ingestionAdmin) {
          return overrides?.ingestionAllowed ?? false;
        }

        if (capability === CAPABILITIES.assignmentAdmin) {
          return overrides?.assignmentAllowed ?? false;
        }

        return false;
      }
    } as never,
    new Logger('error')
  );

  return {
    auditCalls,
    client,
    createAssignmentCalls,
    markPublishedCalls,
    sendCalls,
    service,
    setIngestionCalls
  };
}

test('GuildAdminActionService denies ingestion updates without capability', async () => {
  const { auditCalls, client, service, setIngestionCalls } = createService({
    ingestionAllowed: false
  });

  const reply = await service.setIngestionPolicy({
    channelReference: 'shipping',
    client: client as never,
    enabled: true,
    requester: {
      id: 'requester-1',
      username: 'erick'
    } as never
  });

  assert.equal(reply, 'You do not have permission to manage ingestion policies.');
  assert.equal(setIngestionCalls.length, 0);
  assert.equal(auditCalls[0]?.action, 'dm.tools.ingestion.permission_denied');
});

test('GuildAdminActionService reports ingestion status for a named guild channel', async () => {
  const { auditCalls, client, service } = createService({
    channelPolicy: {
      enabled: true,
      updatedAt: '2026-03-30T01:00:00.000Z',
      updatedByUserId: 'requester-2'
    },
    ingestionAllowed: true
  });

  const reply = await service.getIngestionStatus({
    channelReference: 'shipping',
    client: client as never,
    requester: {
      id: 'requester-1',
      username: 'erick'
    } as never
  });

  assert.match(reply, /Ingestion for <#channel-1> is enabled/i);
  assert.match(reply, /Last updated by <@requester-2>/i);
  assert.equal(auditCalls[0]?.action, 'dm.tools.ingestion.status.executed');
});

test('GuildAdminActionService creates assignments from DM-resolved channel and roles', async () => {
  const { auditCalls, client, createAssignmentCalls, service } = createService({
    assignmentAllowed: true,
    channelReferenceName: 'homework',
    roleName: 'Students'
  });

  const reply = await service.createAssignment({
    affectedRoleReferences: ['Students'],
    channelReference: 'homework',
    client: client as never,
    description: 'Finish chapter 5 problems.',
    dueAt: '2026-04-02T09:00:00Z',
    requester: {
      id: 'requester-1',
      username: 'erick'
    } as never,
    title: 'Chapter 5 homework'
  });

  assert.equal(createAssignmentCalls.length, 1);
  assert.equal(createAssignmentCalls[0]?.announcementChannelId, 'channel-1');
  assert.deepEqual(createAssignmentCalls[0]?.mentionedRoleIds, ['role-1']);
  assert.match(reply, /Created assignment `assignment-1`: Chapter 5 homework/i);
  assert.equal(auditCalls[0]?.action, 'dm.tools.assignment.create.created');
});

test('GuildAdminActionService publishes assignments with the shared announcement embed', async () => {
  const { auditCalls, client, markPublishedCalls, sendCalls, service } = createService({
    assignmentAllowed: true
  });

  const reply = await service.publishAssignment({
    assignmentReference: 'assignment-1',
    channelReference: null,
    client: client as never,
    requester: {
      id: 'requester-1',
      username: 'erick'
    } as never
  });

  assert.equal(sendCalls.length, 1);
  assert.match((sendCalls[0]?.content as string) ?? '', /New assignment notice from Erick/);
  assert.equal((sendCalls[0]?.embeds as unknown[])?.length, 1);
  assert.equal(markPublishedCalls.length, 1);
  assert.equal(markPublishedCalls[0]?.channelId, 'channel-1');
  assert.match(reply, /Published assignment `assignment-1` to <#channel-1>/i);
  assert.equal(auditCalls[0]?.action, 'dm.tools.assignment.publish.published');
});
