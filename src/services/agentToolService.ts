import { randomUUID } from 'node:crypto';

import {
  ActionRowBuilder,
  StringSelectMenuBuilder,
  type ButtonBuilder,
  type Client,
  type Guild,
  type GuildMember,
  type StringSelectMenuInteraction,
  type User
} from 'discord.js';

import type { Env } from '../config/env.js';
import type { Logger } from '../lib/logger.js';
import type { PlannedToolCall, ToolPlanningClient } from '../ports/ai.js';
import type {
  PendingDmRelayRecipientOption,
  PendingDmRelayRecipientSelectionStore
} from '../ports/conversation.js';
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
import type { ModelUsageService } from './modelUsageService.js';
import type { PermissionAdminService } from './permissionAdminService.js';
import { CAPABILITIES, type RolePolicyService } from './rolePolicyService.js';
import type { UsageAdminService } from './usageAdminService.js';

const MAX_TOOL_CALLS_PER_TURN = 3;
const RECIPIENT_SELECT_PREFIX = 'dm-recipient';
const RECIPIENT_SELECTION_TTL_MS = 15 * 60 * 1000;

export interface DmToolHandlingResult {
  components?: ActionRowBuilder<any>[];
  executedToolNames: string[];
  reply: string;
}

export interface DmToolPlanningHints {
  mentionedUsers?: User[];
}

interface ExecutionContext {
  client: Client;
  currentChannelId: string;
  guild: Guild | null;
  mentionedUsers: User[];
  requester: User;
  requesterLabel: string;
  requesterMember: GuildMember | null;
}

interface ToolExecutionResult {
  components?: ActionRowBuilder<any>[];
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
    private readonly usageAdmin: UsageAdminService,
    private readonly pendingRecipientSelections: PendingDmRelayRecipientSelectionStore,
    private readonly modelUsage: ModelUsageService,
    private readonly logger: Logger
  ) {}

  matchesRecipientSelection(interaction: StringSelectMenuInteraction): boolean {
    return interaction.customId.startsWith(`${RECIPIENT_SELECT_PREFIX}:`);
  }

  async handleRecipientSelection(
    interaction: StringSelectMenuInteraction,
    client: Client
  ): Promise<void> {
    const selectionId = interaction.customId.replace(`${RECIPIENT_SELECT_PREFIX}:`, '');
    const pending = await this.pendingRecipientSelections.get(selectionId);

    if (!pending || Date.now() - pending.createdAt > RECIPIENT_SELECTION_TTL_MS) {
      await this.pendingRecipientSelections.delete(selectionId);
      await interaction.reply({
        content: 'That recipient selection has expired. Ask me again and I will re-run it.'
      });
      return;
    }

    if (pending.requesterUserId !== interaction.user.id) {
      await interaction.reply({
        content: 'That recipient picker belongs to another user.'
      });
      return;
    }

    const recipient = pending.recipientOptions.find((option) => option.userId === interaction.values[0]);
    if (!recipient) {
      await interaction.reply({
        content: 'That recipient option was not recognized.'
      });
      return;
    }

    await this.pendingRecipientSelections.delete(selectionId);

    const guild = await this.resolvePrimaryGuild(client);
    const requesterMember = guild
      ? await guild.members.fetch(interaction.user.id).catch(() => null)
      : null;
    const executionContext: ExecutionContext = {
      client,
      currentChannelId: pending.channelId,
      guild,
      mentionedUsers: [],
      requester: interaction.user,
      requesterLabel: pending.requesterUsername,
      requesterMember
    };

    if (!(await this.canDispatchSharedActions(executionContext))) {
      await interaction.update({
        content: 'You no longer have permission to dispatch shared Gigi actions.',
        components: []
      });
      return;
    }

    const recipientAllowed = await this.canReceiveSharedActions(recipient.userId, executionContext);
    if (!recipientAllowed) {
      await interaction.update({
        content: 'I can only DM users through Gigi if that user has `agent_action_receive` permission in the primary server.',
        components: []
      });
      return;
    }

    const confirmation = await this.actionConfirmations.requestRelayConfirmation({
      channelId: pending.channelId,
      context: pending.relayContext,
      guildId: pending.guildId,
      message: pending.relayMessage,
      metadata: {
        createdFrom: 'dm_recipient_selection',
        plannerSurface: 'dm',
        recipientSelectionId: pending.id
      },
      recipientUserId: recipient.userId,
      recipientUsername: recipient.username,
      requesterUserId: pending.requesterUserId,
      requesterUsername: pending.requesterUsername
    });

    await interaction.update({
      content: confirmation.reply,
      components: confirmation.components
    });
  }

  async maybeHandleDmQuery(
    query: string,
    requester: User,
    client: Client,
    currentChannelId: string,
    hints?: DmToolPlanningHints
  ): Promise<DmToolHandlingResult | null> {
    if (!looksLikeToolRequest(query)) {
      return null;
    }

    let plan;
    const planningModel = this.env.OPENAI_TOOL_PLANNING_MODEL ?? this.env.OPENAI_RESPONSE_MODEL;
    try {
      plan = await this.planner.planDmTools({
        model: planningModel,
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
      const deterministicRelay = this.buildDeterministicRelayFallback(query, hints?.mentionedUsers);
      if (deterministicRelay) {
        plan = {
          toolCalls: [deterministicRelay]
        };
      } else {
        return looksLikeRelayIntent(query)
          ? {
              executedToolNames: [],
              reply: 'I could not turn that into a real DM relay request. Mention the user explicitly, use `me`, or use their exact Discord name so I can create a real pending action.',
              components: undefined
            }
          : null;
      }
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
      mentionedUsers: hints?.mentionedUsers ?? [],
      requester,
      requesterLabel,
      requesterMember
    };

    if (plan.usage) {
      await this.modelUsage.record({
        channelId: currentChannelId,
        guildId: guild?.id ?? null,
        inputTokens: plan.usage.inputTokens,
        messageId: null,
        metadata: {
          queryLength: query.length,
          plannedToolCount: plan.toolCalls.length
        },
        model: planningModel,
        operation: 'dm_tool_planning',
        outputTokens: plan.usage.outputTokens,
        provider: 'openai',
        requesterUserId: requester.id,
        surface: 'dm',
        totalTokens: plan.usage.totalTokens
      });
    }

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

  private buildDeterministicRelayFallback(
    query: string,
    mentionedUsers?: User[]
  ): Extract<PlannedToolCall, { name: 'send_dm_relay' }> | null {
    if (!looksLikeRelayIntent(query)) {
      return null;
    }

    const relayMessage = extractQuotedRelayMessage(query);
    if (!relayMessage) {
      return null;
    }

    const nonBotMentions = (mentionedUsers ?? []).filter((user) => !user.bot);
    const extractedReference = extractRelayRecipientReference(query);
    if (nonBotMentions.length !== 1 && !extractedReference) {
      return null;
    }

    return {
      context: null,
      message: relayMessage,
      name: 'send_dm_relay',
      recipientReference: nonBotMentions.length === 1
        ? `<@${nonBotMentions[0]?.id}>`
        : extractedReference!
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

    if (toolCall.name === 'get_usage_summary') {
      return this.executeGetUsageSummary(toolCall, context);
    }

    if (toolCall.name === 'get_user_usage_summary') {
      return this.executeGetUserUsageSummary(toolCall, context);
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
      const recipientCandidates = await this.findRelayRecipientCandidates(
        toolCall.recipientReference,
        context
      );

      if (recipientCandidates.length === 1) {
        const [directCandidate] = recipientCandidates;
        const directRecipient = await context.client.users.fetch(directCandidate.userId).catch(() => null);
        if (directRecipient) {
          return this.executeRelayForRecipient(toolCall, context, directRecipient);
        }
      }

      if (recipientCandidates.length > 1) {
        return this.createRelayRecipientSelection(toolCall, context, recipientCandidates);
      }

      return {
        handled: true,
        summary: unresolvedUserSummary(toolCall.recipientReference, 'relay recipient'),
        toolName: toolCall.name
      };
    }

    return this.executeRelayForRecipient(toolCall, context, recipient);
  }

  private async executeRelayForRecipient(
    toolCall: Extract<PlannedToolCall, { name: 'send_dm_relay' }>,
    context: ExecutionContext,
    recipient: User
  ): Promise<ToolExecutionResult> {
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

  private async createRelayRecipientSelection(
    toolCall: Extract<PlannedToolCall, { name: 'send_dm_relay' }>,
    context: ExecutionContext,
    candidates: PendingDmRelayRecipientOption[]
  ): Promise<ToolExecutionResult> {
    const selectionId = randomUUID();
    await this.pendingRecipientSelections.deleteExpired(new Date());
    await this.pendingRecipientSelections.save(
      {
        channelId: context.currentChannelId,
        createdAt: Date.now(),
        guildId: context.guild?.id ?? null,
        id: selectionId,
        recipientOptions: candidates,
        relayContext: toolCall.context?.trim() ?? null,
        relayMessage: toolCall.message.trim(),
        requesterUserId: context.requester.id,
        requesterUsername: context.requesterLabel
      },
      new Date(Date.now() + RECIPIENT_SELECTION_TTL_MS)
    );

    return {
      handled: true,
      components: [
        new ActionRowBuilder<StringSelectMenuBuilder>().addComponents(
          new StringSelectMenuBuilder()
            .setCustomId(`${RECIPIENT_SELECT_PREFIX}:${selectionId}`)
            .setPlaceholder('Choose who Gigi should DM')
            .addOptions(
              candidates.slice(0, 25).map((candidate) => ({
                description: candidate.displayLabel === candidate.username
                  ? undefined
                  : truncateComponentText(`@${candidate.username}`),
                label: truncateComponentText(candidate.displayLabel),
                value: candidate.userId
              }))
            )
        )
      ],
      summary: `I found multiple possible users for "${toolCall.recipientReference ?? 'that recipient'}". Pick who I should DM, then I’ll create the real confirmation step.`,
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

  private async executeGetUsageSummary(
    toolCall: Extract<PlannedToolCall, { name: 'get_usage_summary' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    return {
      handled: true,
      summary: await this.usageAdmin.getUsageSummary({
        client: context.client,
        days: normalizeUsageDays(toolCall.days),
        requester: context.requester
      }),
      toolName: toolCall.name
    };
  }

  private async executeGetUserUsageSummary(
    toolCall: Extract<PlannedToolCall, { name: 'get_user_usage_summary' }>,
    context: ExecutionContext
  ): Promise<ToolExecutionResult> {
    const targetUser = await this.resolveUserReference(toolCall.userReference, context);
    if (!targetUser) {
      return {
        handled: true,
        summary: unresolvedUserSummary(toolCall.userReference, 'usage target'),
        toolName: toolCall.name
      };
    }

    return {
      handled: true,
      summary: await this.usageAdmin.getUserUsageSummary({
        client: context.client,
        days: normalizeUsageDays(toolCall.days),
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

    if (context.mentionedUsers.length === 1 && looksLikeExplicitMentionReference(userReference)) {
      return context.mentionedUsers[0] ?? null;
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

  private async findRelayRecipientCandidates(
    userReference: string | null,
    context: ExecutionContext
  ): Promise<PendingDmRelayRecipientOption[]> {
    const candidates = new Map<string, PendingDmRelayRecipientOption>();
    for (const mentionedUser of context.mentionedUsers) {
      if (mentionedUser.bot) {
        continue;
      }

      candidates.set(mentionedUser.id, {
        displayLabel: mentionedUser.globalName ?? mentionedUser.username,
        username: mentionedUser.username,
        userId: mentionedUser.id
      });
    }

    if (!context.guild) {
      return [...candidates.values()];
    }

    const searchQueries = buildRecipientSearchQueries(userReference);
    for (const query of searchQueries) {
      const memberMatches = await context.guild.members.search({
        query,
        limit: 5
      }).catch(() => null);
      if (!memberMatches) {
        continue;
      }

      for (const member of memberMatches.values()) {
        if (member.user.bot) {
          continue;
        }

        candidates.set(member.user.id, {
          displayLabel: member.displayName ?? member.user.globalName ?? member.user.username,
          username: member.user.username,
          userId: member.user.id
        });
      }
    }

    return [...candidates.values()];
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
    'Only use tools for explicit action requests such as creating tasks, listing tasks, completing tasks, sending a DM relay, checking or changing ingestion status, listing, creating, or publishing assignments, listing, granting, or revoking direct user permissions, or showing usage summaries and estimated cost.',
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

function normalizeUsageDays(value: number | null | undefined): number {
  if (!value || !Number.isFinite(value)) {
    return 7;
  }

  return Math.min(Math.max(Math.trunc(value), 1), 30);
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

function extractQuotedRelayMessage(query: string): string | null {
  const match = query.match(/["“](?<message>[^"”]+)["”]/);
  const message = match?.groups?.message?.trim() ?? null;
  return message && message.length > 0 ? message : null;
}

function extractRelayRecipientReference(query: string): string | null {
  const beforeQuote = query.search(/["“]/) >= 0
    ? query.slice(0, query.search(/["“]/))
    : query;
  const cleaned = beforeQuote.trim();
  const patterns = [
    /\b(?:can you\s+)?dm\s+(?<recipient>.+)$/i,
    /\b(?:can you\s+)?message\s+(?<recipient>.+)$/i,
    /\b(?:can you\s+)?send(?:\s+\w+)?\s+a\s+dm\s+to\s+(?<recipient>.+)$/i,
    /\b(?:can you\s+)?send\s+(?<recipient>.+?)\s+a\s+dm$/i
  ];

  for (const pattern of patterns) {
    const recipient = pattern.exec(cleaned)?.groups?.recipient?.trim();
    if (recipient) {
      return recipient;
    }
  }

  return null;
}

function looksLikeExplicitMentionReference(value: string | null | undefined): boolean {
  if (!value) {
    return false;
  }

  return value.includes('@') || value.includes('<@');
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

function buildRecipientSearchQueries(userReference: string | null): string[] {
  const normalized = normalizeReference(userReference);
  if (!normalized) {
    return [];
  }

  const tokenMatches = normalized.match(/[\p{L}\p{N}_-]{2,}/gu) ?? [];
  const searchQueries = new Set<string>([normalized]);
  for (const token of tokenMatches.sort((left, right) => right.length - left.length)) {
    searchQueries.add(token);
  }

  return [...searchQueries].slice(0, 5);
}

function truncateComponentText(value: string): string {
  return value.slice(0, 100);
}
