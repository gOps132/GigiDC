import test from 'node:test';
import assert from 'node:assert/strict';

import { PermissionAdminService } from '../src/services/permissionAdminService.js';
import { CAPABILITIES } from '../src/services/rolePolicyService.js';

function createService(overrides?: {
  allowed?: boolean;
  directCapabilities?: string[];
  effectiveCapabilities?: string[];
  revokeResult?: boolean;
}) {
  const auditCalls: Array<Record<string, unknown>> = [];
  const grantCalls: Array<Record<string, unknown>> = [];
  const revokeCalls: Array<Record<string, unknown>> = [];

  const guild = {
    id: 'guild-1',
    members: {
      async fetch(userId: string) {
        return {
          id: userId
        };
      }
    }
  };

  const service = new PermissionAdminService(
    {
      DISCORD_GUILD_ID: 'guild-1'
    } as never,
    {
      async record(input: Record<string, unknown>) {
        auditCalls.push(input);
      }
    } as never,
    {
      async grantUserCapability(input: Record<string, unknown>) {
        grantCalls.push(input);
      },
      async listDirectUserCapabilities() {
        return (overrides?.directCapabilities as never) ?? [];
      },
      async listEffectiveCapabilities() {
        return (overrides?.effectiveCapabilities as never) ?? [CAPABILITIES.permissionAdmin];
      },
      async memberHasCapability() {
        return overrides?.allowed ?? false;
      },
      async revokeUserCapability(guildId: string, userId: string, capability: string) {
        revokeCalls.push({
          capability,
          guildId,
          userId
        });
        return overrides?.revokeResult ?? true;
      }
    } as never
  );

  const client = {
    guilds: {
      cache: new Map([['guild-1', guild]]),
      async fetch() {
        return guild;
      }
    }
  };

  return {
    auditCalls,
    client,
    grantCalls,
    revokeCalls,
    service
  };
}

test('PermissionAdminService denies grant attempts without permission_admin', async () => {
  const { auditCalls, client, grantCalls, service } = createService({
    allowed: false
  });

  const reply = await service.grantUserPermission({
    capability: 'permission_admin',
    client: client as never,
    requester: {
      id: 'requester-1',
      username: 'erick'
    } as never,
    targetUser: {
      id: 'target-1',
      username: 'mina'
    } as never
  });

  assert.equal(reply, 'You do not have permission to manage Gigi direct user permissions.');
  assert.equal(grantCalls.length, 0);
  assert.equal(auditCalls[0]?.action, 'permission.grant.permission_denied');
});

test('PermissionAdminService lists direct and effective capabilities', async () => {
  const { auditCalls, client, service } = createService({
    allowed: true,
    directCapabilities: [CAPABILITIES.agentActionDispatch],
    effectiveCapabilities: [CAPABILITIES.agentActionDispatch, CAPABILITIES.permissionAdmin]
  });

  const reply = await service.listUserPermissions({
    client: client as never,
    requester: {
      id: 'requester-1',
      username: 'erick'
    } as never,
    targetUser: {
      id: 'target-1',
      username: 'mina'
    } as never
  });

  assert.match(reply, /Effective capabilities/i);
  assert.match(reply, /Direct grants/i);
  assert.equal(auditCalls[0]?.action, 'permission.list.executed');
});
