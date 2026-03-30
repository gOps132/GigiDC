import type {
  ActionRowBuilder,
  ButtonBuilder,
  Client,
  Guild,
  GuildMember,
  User
} from 'discord.js';

import type { Env } from '../config/env.js';
import type { Logger } from '../lib/logger.js';
import type { PlannedToolCall, ToolPlanningClient } from '../ports/ai.js';
import type { ActionConfirmationService } from './actionConfirmationService.js';
import type { AuditLogService } from './auditLogService.js';
import {
  canUserAccessAction,
  isTaskAction,
  type AgentActionRecord,
  type AgentActionService
} from './agentActionService.js';
import type { GuildAdminActionService } from './guildAdminActionService.js';
import { looksLikeToolRequest } from './dmIntentRouter.js';
import type { PermissionAdminService } from './permissionAdminService.js';
import { CAPABILITIES, type RolePolicyService } from './rolePolicyService.js';

const MAX_TOOL_CALLS_PER_TURN = 3;

export interface DmToolHandlingResult {
  components?: ActionRowBuilder<ButtonBuilder>[];
  executedToolNames: string[];
  reply: string;
}

interface ExecutionContext {
  client: Client;
  currentChannelId: string;
  guild: Guild | null;
  requester: User;
  requesterLabel: string;
  requesterMember: GuildMember | null;
}

interface ToolExecutionResult {
  components?: ActionRowBuilder<ButtonBuilder>[];
  handled: boolean;
  summary: string;
  toolName: PlannedToolCall['name'];
}

export class AgentToolService {
  constructor(
    private readonly env: Env,
    private readonly planner: ToolPlanningClient,
    private readonly actionConfirmations: ActionConfirmationService,
    private readonly agentActions: AgentActionService,
    private readonly auditLogs: AuditLogService,
    private readonly guildAdminActions: GuildAdminActionService,
    private readonly permissionAdmin: PermissionAdminService,
    private readonly rolePolicies: RolePolicyService,
    private readonly logger: Logger
  ) {}

  async maybeHandleDmQuery(
    query: string,
    requester: User,
    client: Client,
    currentChannelId: string
  ): Promise<DmToolHandlingResult | null> {
    if (!looksLikeToolRequest(query)) {
      return null;
    }

    let plan;
    try {
      plan = await this.planner.planDmTools({
        model: this.env.OPENAI_RESPONSE_MODEL,
        instructions: buildPlannerInstructions(),
        text: query
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unknown DM tool planning error';
      this.logger.error('Failed to plan DM tool calls', {
        error: message,
        requesterUserId: requester.id
      });
      return looksLikeRelayIntent(query)
        ? {
            executedToolNames: [],
            reply: 'I could not turn that into a real DM relay request. Mention the user explicitly, use `me`, or use their exact Discord name so I can create a real pending action.',
            components: undefined
          }
        : null;
    }

    if (plan.toolCalls.length === 0) {
      return looksLikeRelayIntent(query)
        ? {
            executedToolNames: [],
            reply: 'I could not turn that into a real DM relay request. Mention the user explicitly, use `me`, or use their exact Discord name so I can create a real pending action.',
            components: undefined
          }
        : null;
    }

    const guild = await this.resolvePrimaryGuild(client);
    const requesterMember = guild
      ? await guild.members.fetch(requester.id).catch(() => null)
      : null;
    const requesterLabel = requesterMember?.displayName ?? requester.username;
    const executionContext: ExecutionContext = {
      client,
      currentChannelId,
      guild,
      requester,
      requesterLabel,
      requesterMember
    };

    const results: ToolExecutionResult[] = [];
    for (const toolCall of plan.toolCalls.slice(0, MAX_TOOL_CALLS_PER_TURN)) {
      results.push(await this.executeToolCall(toolCall, executionContext));
    }

    if (results.length === 0) {
      return looksLikeRelayIntent(query)
        ? {
            executedToolNames: [],
            reply: 'I could not turn that into a real DM relay request. Mention the user explicitly, use `me`, or use their exact Discord name so I can create a real pending action.',
            components: undefined
          }
        : null;
    }

    return {
      components: results.find((result) => result.components)?.components,
      executedToolNames: results.map((result) => result.toolName),
      reply: results.map((result) => result.summary).join('\n\n')
    };
  }

  private async executeToolCall(
    toolCall: PlannedToolCall,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    if (toolCall.name === 'create_follow_up_task') {
      return this.executeCreateFollowUpTask(toolCall, context);
    }

    if (toolCall.name === 'list_open_tasks') {
      return this.executeListOpenTasks(toolCall, context);
    }

    if (toolCall.name === 'complete_task') {
      return this.executeCompleteTask(toolCall, context);
    }

    if (toolCall.name === 'get_ingestion_status') {
      return this.executeGetIngestionStatus(toolCall, context);
    }

    if (toolCall.name === 'set_ingestion_policy') {
      return this.executeSetIngestionPolicy(toolCall, context);
    }

    if (toolCall.name === 'create_assignment') {
      return this.executeCreateAssignment(toolCall, context);
    }

    if (toolCall.name === 'list_assignments') {
      return this.executeListAssignments(toolCall, context);
    }

    if (toolCall.name === 'publish_assignment') {
      return this.executePublishAssignment(toolCall, context);
    }

    if (toolCall.name === 'grant_permission') {
      return this.executeGrantPermission(toolCall, context);
    }

    if (toolCall.name === 'revoke_permission') {
      return this.executeRevokePermission(toolCall, context);
    }

    if (toolCall.name === 'list_permissions') {
      return this.executeListPermissions(toolCall, context);
    }

    return this.executeSendDmRelay(toolCall, context);
  }

  private async executeCreateFollowUpTask(
    toolCall: Extract<PlannedToolCall, { name: 'create_follow_up_task' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    const canDispatch = await this.canDispatchSharedActions(context);
    if (!canDispatch) {
      await this.recordAudit(context, 'dm.tools.create_follow_up_task.permission_denied', null, {
        assigneeReference: toolCall.assigneeReference
      });

      return {
        handled: true,
        summary: 'I can only create shared Gigi tasks for users with `agent_action_dispatch` access.',
        toolName: toolCall.name
      };
    }

    const assignee = await this.resolveUserReference(
      toolCall.assigneeReference,
      context
    );
    if (!assignee) {
      return {
        handled: true,
        summary: unresolvedUserSummary(toolCall.assigneeReference, 'task assignee'),
        toolName: toolCall.name
      };
    }

    const dueAt = normalizeDueAt(toolCall.dueAt);
    if (toolCall.dueAt && !dueAt) {
      return {
        handled: true,
        summary: 'I could not create that task because the due date was not a full ISO-8601 timestamp.',
        toolName: toolCall.name
      };
    }

    const task = await this.agentActions.createFollowUpTask({
      guildId: context.guild?.id ?? null,
      channelId: context.currentChannelId,
      requesterUserId: context.requester.id,
      requesterUsername: context.requesterLabel,
      assigneeUserId: assignee.id,
      assigneeUsername: assignee.username,
      title: toolCall.title.trim(),
      instructions: toolCall.details.trim(),
      dueAt: dueAt?.toISOString() ?? null,
      metadata: {
        createdFrom: 'dm_tool',
        originalQuery: truncateForMetadata(toolCall.details),
        plannerSurface: 'dm'
      }
    });

    await this.recordAudit(context, 'dm.tools.create_follow_up_task.created', task.id, {
      assigneeUserId: assignee.id,
      assigneeUsername: assignee.username,
      dueAt: task.due_at
    });

    const dueText = task.due_at ? ` Due ${new Date(task.due_at).toISOString()}.` : '';
    return {
      handled: true,
      summary: `Created task \`${task.id}\` for ${assignee.username}: ${task.title}.${dueText}`,
      toolName: toolCall.name
    };
  }

  private async executeListOpenTasks(
    toolCall: Extract<PlannedToolCall, { name: 'list_open_tasks' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    const targetUser = await this.resolveUserReference(toolCall.userReference, context);
    if (!targetUser) {
      return {
        handled: true,
        summary: unresolvedUserSummary(toolCall.userReference, 'task owner'),
        toolName: toolCall.name
      };
    }

    const canDispatch = await this.canDispatchSharedActions(context);
    if (targetUser.id !== context.requester.id && !canDispatch) {
      await this.recordAudit(context, 'dm.tools.list_open_tasks.permission_denied', null, {
        requestedUserId: targetUser.id,
        requestedUsername: targetUser.username
      });

      return {
        handled: true,
        summary: 'I can only list another user\'s tasks for users with `agent_action_dispatch` access.',
        toolName: toolCall.name
      };
    }

    const tasks = await this.agentActions.listOpenTasksForUser(targetUser.id, 10);
    await this.recordAudit(context, 'dm.tools.list_open_tasks.executed', null, {
      listedUserId: targetUser.id,
      listedUsername: targetUser.username,
      openTaskCount: tasks.length
    });

    if (tasks.length === 0) {
      return {
        handled: true,
        summary: targetUser.id === context.requester.id
          ? 'You have no open Gigi tasks.'
          : `${targetUser.username} has no open Gigi tasks.`,
        toolName: toolCall.name
      };
    }

    return {
      handled: true,
      summary: [
        targetUser.id === context.requester.id
          ? 'Your open Gigi tasks:'
          : `${targetUser.username}'s open Gigi tasks:`,
        ...tasks.map((task) => formatTaskSummary(task))
      ].join('\n'),
      toolName: toolCall.name
    };
  }

  private async executeCompleteTask(
    toolCall: Extract<PlannedToolCall, { name: 'complete_task' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    const task = await this.resolveTaskReference(toolCall.taskReference, context.requester.id);
    if (!task || !isTaskAction(task)) {
      return {
        handled: true,
        summary: `I could not find an open task matching "${toolCall.taskReference}".`,
        toolName: toolCall.name
      };
    }

    const canDispatch = await this.canDispatchSharedActions(context);
    if (!canUserAccessAction(task, context.requester.id) && !canDispatch) {
      await this.recordAudit(context, 'dm.tools.complete_task.permission_denied', task.id, {
        taskReference: toolCall.taskReference
      });

      return {
        handled: true,
        summary: 'You do not have permission to complete that task.',
        toolName: toolCall.name
      };
    }

    const completed = await this.agentActions.markCompleted(task, {
      metadata: {
        completedFrom: 'dm_tool'
      },
      resultSummary: toolCall.result?.trim() ?? null
    });

    await this.recordAudit(context, 'dm.tools.complete_task.completed', completed.id, {
      resultSummary: completed.result_summary
    });

    return {
      handled: true,
      summary: `Marked task \`${completed.id}\` complete: ${completed.title}.`,
      toolName: toolCall.name
    };
  }

  private async executeSendDmRelay(
    toolCall: Extract<PlannedToolCall, { name: 'send_dm_relay' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    const canDispatch = await this.canDispatchSharedActions(context);
    if (!canDispatch) {
      await this.recordAudit(context, 'dm.tools.send_dm_relay.permission_denied', null, {
        recipientReference: toolCall.recipientReference
      });

      return {
        handled: true,
        summary: 'I can only send shared Gigi relays for users with `agent_action_dispatch` access.',
        toolName: toolCall.name
      };
    }

    const recipient = await this.resolveUserReference(toolCall.recipientReference, context);
    if (!recipient) {
      return {
        handled: true,
        summary: unresolvedUserSummary(toolCall.recipientReference, 'relay recipient'),
        toolName: toolCall.name
      };
    }

    const recipientAllowed = await this.canReceiveSharedActions(recipient.id, context);
    if (!recipientAllowed) {
      await this.recordAudit(context, 'dm.tools.send_dm_relay.recipient_permission_denied', null, {
        recipientUserId: recipient.id,
        recipientUsername: recipient.username
      });

      return {
        handled: true,
        summary: 'I can only DM users through Gigi if that user has `agent_action_receive` permission in the primary server.',
        toolName: toolCall.name
      };
    }

    const confirmation = await this.actionConfirmations.requestRelayConfirmation({
      channelId: context.currentChannelId,
      context: toolCall.context?.trim() ?? null,
      guildId: context.guild?.id ?? null,
      message: toolCall.message.trim(),
      metadata: {
        createdFrom: 'dm_tool',
        plannerSurface: 'dm'
      },
      recipientUserId: recipient.id,
      recipientUsername: recipient.username,
      requesterUserId: context.requester.id,
      requesterUsername: context.requesterLabel
    });

    return {
      handled: true,
      components: confirmation.components,
      summary: confirmation.reply,
      toolName: toolCall.name
    };
  }

  private async executeGetIngestionStatus(
    toolCall: Extract<PlannedToolCall, { name: 'get_ingestion_status' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    return {
      handled: true,
      summary: await this.guildAdminActions.getIngestionStatus({
        channelReference: toolCall.channelReference,
        client: context.client,
        requester: context.requester
      }),
      toolName: toolCall.name
    };
  }

  private async executeSetIngestionPolicy(
    toolCall: Extract<PlannedToolCall, { name: 'set_ingestion_policy' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    return {
      handled: true,
      summary: await this.guildAdminActions.setIngestionPolicy({
        channelReference: toolCall.channelReference,
        client: context.client,
        enabled: toolCall.enabled,
        requester: context.requester
      }),
      toolName: toolCall.name
    };
  }

  private async executeCreateAssignment(
    toolCall: Extract<PlannedToolCall, { name: 'create_assignment' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    return {
      handled: true,
      summary: await this.guildAdminActions.createAssignment({
        affectedRoleReferences: toolCall.affectedRoleReferences,
        channelReference: toolCall.channelReference,
        client: context.client,
        description: toolCall.description,
        dueAt: toolCall.dueAt,
        requester: context.requester,
        title: toolCall.title
      }),
      toolName: toolCall.name
    };
  }

  private async executeListAssignments(
    toolCall: Extract<PlannedToolCall, { name: 'list_assignments' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    return {
      handled: true,
      summary: await this.guildAdminActions.listAssignments({
        client: context.client,
        requester: context.requester
      }),
      toolName: toolCall.name
    };
  }

  private async executePublishAssignment(
    toolCall: Extract<PlannedToolCall, { name: 'publish_assignment' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    return {
      handled: true,
      summary: await this.guildAdminActions.publishAssignment({
        assignmentReference: toolCall.assignmentReference,
        channelReference: toolCall.channelReference,
        client: context.client,
        requester: context.requester
      }),
      toolName: toolCall.name
    };
  }

  private async executeGrantPermission(
    toolCall: Extract<PlannedToolCall, { name: 'grant_permission' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    const targetUser = await this.resolveUserReference(toolCall.userReference, context);
    if (!targetUser) {
      return {
        handled: true,
        summary: unresolvedUserSummary(toolCall.userReference, 'permission target'),
        toolName: toolCall.name
      };
    }

    return {
      handled: true,
      summary: await this.permissionAdmin.grantUserPermission({
        capability: toolCall.capability,
        client: context.client,
        requester: context.requester,
        targetUser
      }),
      toolName: toolCall.name
    };
  }

  private async executeRevokePermission(
    toolCall: Extract<PlannedToolCall, { name: 'revoke_permission' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    const targetUser = await this.resolveUserReference(toolCall.userReference, context);
    if (!targetUser) {
      return {
        handled: true,
        summary: unresolvedUserSummary(toolCall.userReference, 'permission target'),
        toolName: toolCall.name
      };
    }

    return {
      handled: true,
      summary: await this.permissionAdmin.revokeUserPermission({
        capability: toolCall.capability,
        client: context.client,
        requester: context.requester,
        targetUser
      }),
      toolName: toolCall.name
    };
  }

  private async executeListPermissions(
    toolCall: Extract<PlannedToolCall, { name: 'list_permissions' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    const targetUser = await this.resolveUserReference(toolCall.userReference, context);
    if (!targetUser) {
      return {
        handled: true,
        summary: unresolvedUserSummary(toolCall.userReference, 'permission target'),
        toolName: toolCall.name
      };
    }

    return {
      handled: true,
      summary: await this.permissionAdmin.listUserPermissions({
        client: context.client,
        requester: context.requester,
        targetUser
      }),
      toolName: toolCall.name
    };
  }

  private async canDispatchSharedActions(context: ExecutionContext): Promise<boolean> {
    if (!context.guild || !context.requesterMember) {
      return false;
    }

    return this.rolePolicies.memberHasCapability(
      context.guild,
      context.requesterMember,
      CAPABILITIES.agentActionDispatch
    );
  }

  private async canReceiveSharedActions(
    recipientUserId: string,
    context: ExecutionContext
  ): Promise<boolean> {
    if (!context.guild) {
      return false;
    }

    const recipientMember = await context.guild.members.fetch(recipientUserId).catch(() => null);
    if (!recipientMember) {
      return false;
    }

    return this.rolePolicies.memberHasCapability(
      context.guild,
      recipientMember,
      CAPABILITIES.agentActionReceive
    );
  }

  private async resolvePrimaryGuild(client: Client): Promise<Guild | null> {
    const primaryGuildId = this.env.PRIMARY_GUILD_ID ?? this.env.DISCORD_GUILD_ID;
    if (!primaryGuildId) {
      return null;
    }

    return client.guilds.cache.get(primaryGuildId)
      ?? (await client.guilds.fetch(primaryGuildId).catch(() => null));
  }

  private async resolveUserReference(
    userReference: string | null,
    context: ExecutionContext
  ): Promise<User | null> {
    const normalized = normalizeReference(userReference);
    if (!normalized || normalized === 'me' || normalized === 'myself' || normalized === 'self') {
      return context.requester;
    }

    const mentionedUserId = parseMentionedUserId(normalized);
    if (mentionedUserId) {
      return context.client.users.fetch(mentionedUserId).catch(() => null);
    }

    if (!context.guild) {
      return null;
    }

    const requesterNames = [
      context.requester.username,
      context.requester.globalName,
      context.requesterMember?.displayName
    ].map((value) => normalizeReference(value));

    if (requesterNames.includes(normalized)) {
      return context.requester;
    }

    const memberMatches = await context.guild.members.search({
      query: normalized,
      limit: 5
    }).catch(() => null);

    if (!memberMatches || memberMatches.size === 0) {
      return null;
    }

    const exactMatch = [...memberMatches.values()].find((member) =>
      [
        member.user.username,
        member.user.globalName,
        member.displayName
      ]
        .map((value) => normalizeReference(value))
        .includes(normalized)
    );

    if (exactMatch) {
      return exactMatch.user;
    }

    return memberMatches.size === 1
      ? [...memberMatches.values()][0]?.user ?? null
      : null;
  }

  private async resolveTaskReference(
    taskReference: string,
    requesterUserId: string
  ): Promise<AgentActionRecord | null> {
    const trimmedReference = taskReference.trim();
    const byId = await this.agentActions.getActionById(trimmedReference);
    if (byId && isTaskAction(byId)) {
      return byId;
    }

    const openTasks = await this.agentActions.listOpenTasksForUser(requesterUserId, 12);
    const normalizedReference = normalizeReference(trimmedReference);
    if (!normalizedReference) {
      return null;
    }

    const exactMatch = openTasks.find((task) =>
      normalizeReference(task.title) === normalizedReference
    );
    if (exactMatch) {
      return exactMatch;
    }

    const containsMatch = openTasks.find((task) =>
      normalizeReference(task.title)?.includes(normalizedReference)
      || normalizeReference(task.instructions)?.includes(normalizedReference)
    );

    return containsMatch ?? null;
  }

  private async recordAudit(
    context: ExecutionContext,
    action: string,
    targetId: string | null,
    metadata?: Record<string, unknown>
  ): Promise<void> {
    await this.auditLogs.record({
      guildId: context.guild?.id ?? null,
      actorUserId: context.requester.id,
      action,
      targetType: 'agent_action',
      targetId,
      metadata
    });
  }
}

function buildPlannerInstructions(): string {
  return [
    'You are the internal DM tool planner for GigiDC.',
    'Decide whether the user is asking Gigi to execute internal tools instead of answering from retrieval.',
    'Only use tools for explicit action requests such as creating tasks, listing tasks, completing tasks, sending a DM relay, checking or changing ingestion status, listing, creating, or publishing assignments, or listing, granting, or revoking direct user permissions.',
    'Return no tool calls for pure history questions, freeform chat, or questions that should be handled by retrieval.',
    'Keep calls in the same order the user expects them to happen.',
    'Never invent user IDs or task IDs.',
    'Use the raw user reference from the request for assigneeReference, userReference, recipientReference, or taskReference.',
    'Use the raw channel or role references from the request for channelReference and affectedRoleReferences.',
    'Use at most three tool calls.'
  ].join(' ');
}

function normalizeDueAt(value: string | null): Date | null {
  if (!value) {
    return null;
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.valueOf())) {
    return null;
  }

  return parsed;
}

function normalizeReference(value: string | null | undefined): string | null {
  const normalized = value?.trim().replace(/^@/, '').toLowerCase() ?? '';
  return normalized.length > 0 ? normalized : null;
}

function parseMentionedUserId(value: string): string | null {
  const match = value.match(/^<@!?(?<id>\d+)>$/);
  return match?.groups?.id ?? null;
}

function unresolvedUserSummary(reference: string | null, targetLabel: string): string {
  if (!reference) {
    return `I could not resolve the ${targetLabel}.`;
  }

  return `I could not resolve "${reference}" as the ${targetLabel}. Mention the user explicitly or use their exact Discord name.`;
}

function looksLikeRelayIntent(query: string): boolean {
  const normalized = query.trim().toLowerCase();
  return normalized.includes('can you dm')
    || normalized.includes('send a dm')
    || normalized.includes('send me a dm')
    || normalized.includes('send them a dm')
    || normalized.includes('send him a dm')
    || normalized.includes('send her a dm')
    || normalized.includes('dm me')
    || normalized.includes('dm @')
    || normalized.includes('message @')
    || normalized.includes('relay')
    || (normalized.includes(' dm ') && normalized.includes('"'))
    || (normalized.includes(' dm ') && normalized.includes('“'))
    || (normalized.includes('message ') && normalized.includes('"'))
    || (normalized.includes('message ') && normalized.includes('“'));
}

function formatTaskSummary(task: AgentActionRecord): string {
  const dueText = task.due_at ? ` (due ${new Date(task.due_at).toISOString()})` : '';
  return `- \`${task.id}\` ${task.title}${dueText}`;
}

function truncateForMetadata(value: string): string {
  return value.slice(0, 200);
}
