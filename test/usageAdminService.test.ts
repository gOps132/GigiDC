import test from 'node:test';
import assert from 'node:assert/strict';

import { UsageAdminService } from '../src/services/usageAdminService.js';
import { CAPABILITIES } from '../src/services/rolePolicyService.js';

function createService(overrides?: {
  allowed?: boolean;
  dailyRows?: Array<Record<string, unknown>>;
  requesterRows?: Array<Record<string, unknown>>;
}) {
  const auditCalls: Array<Record<string, unknown>> = [];
  const dailySummaryCalls: Array<Record<string, unknown>> = [];
  const requesterSummaryCalls: Array<Record<string, unknown>> = [];

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

  const service = new UsageAdminService(
    {
      DISCORD_GUILD_ID: 'guild-1'
    } as never,
    {
      async record(input: Record<string, unknown>) {
        auditCalls.push(input);
      }
    } as never,
    {
      async listDailySummary(input: Record<string, unknown>) {
        dailySummaryCalls.push(input);
        return (overrides?.dailyRows as never) ?? [];
      },
      async listRequesterDailySummary(input: Record<string, unknown>) {
        requesterSummaryCalls.push(input);
        return (overrides?.requesterRows as never) ?? [];
      }
    } as never,
    {
      async memberHasCapability() {
        return overrides?.allowed ?? false;
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
    dailySummaryCalls,
    requesterSummaryCalls,
    service
  };
}

test('UsageAdminService denies summary access without usage_admin', async () => {
  const { auditCalls, client, dailySummaryCalls, service } = createService({
    allowed: false
  });

  const reply = await service.getUsageSummary({
    client: client as never,
    days: 7,
    requester: {
      id: 'requester-1',
      username: 'erick'
    } as never
  });

  assert.equal(reply, 'You do not have permission to inspect Gigi usage summaries.');
  assert.equal(dailySummaryCalls.length, 0);
  assert.equal(auditCalls[0]?.action, 'usage.summary.permission_denied');
  const metadata = auditCalls[0]?.metadata as { capability?: string } | undefined;
  assert.equal(metadata?.capability, CAPABILITIES.usageAdmin);
});

test('UsageAdminService returns formatted summary for recent server usage', async () => {
  const { auditCalls, client, dailySummaryCalls, service } = createService({
    allowed: true,
    dailyRows: [
      {
        estimatedCostUsd: 0.012345,
        eventCount: 3,
        inputTokens: 1000,
        model: 'gpt-4.1-mini',
        operation: 'retrieval_response',
        outputTokens: 500,
        provider: 'openai',
        surface: 'dm',
        totalTokens: 1500,
        usageDay: '2026-03-30T00:00:00.000Z'
      }
    ]
  });

  const reply = await service.getUsageSummary({
    client: client as never,
    days: 7,
    requester: {
      id: 'requester-1',
      username: 'erick'
    } as never
  });

  assert.equal(dailySummaryCalls[0]?.guildId, 'guild-1');
  assert.match(reply, /Server model usage/i);
  assert.match(reply, /\$0\.012345/);
  assert.match(reply, /retrieval_response/);
  assert.equal(auditCalls[0]?.action, 'usage.summary.executed');
});

test('UsageAdminService returns formatted summary for one user', async () => {
  const { auditCalls, client, requesterSummaryCalls, service } = createService({
    allowed: true,
    requesterRows: [
      {
        estimatedCostUsd: 0.000321,
        eventCount: 2,
        inputTokens: 200,
        operation: 'dm_tool_planning',
        outputTokens: 80,
        provider: 'openai',
        requesterUserId: 'target-1',
        surface: 'dm',
        totalTokens: 280,
        usageDay: '2026-03-30T00:00:00.000Z'
      }
    ]
  });

  const reply = await service.getUserUsageSummary({
    client: client as never,
    days: 3,
    requester: {
      id: 'requester-1',
      username: 'erick'
    } as never,
    targetUser: {
      id: 'target-1',
      username: 'mina'
    } as never
  });

  assert.equal(requesterSummaryCalls[0]?.requesterUserId, 'target-1');
  assert.match(reply, /Usage for <@target-1>/);
  assert.match(reply, /dm_tool_planning/);
  assert.equal(auditCalls[0]?.action, 'usage.user.executed');
});
