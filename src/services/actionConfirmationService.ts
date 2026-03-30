import {
  ActionRowBuilder,
  ButtonBuilder,
  ButtonStyle,
  MessageFlags,
  type ButtonInteraction,
  type Client,
  type Guild,
  type GuildMember,
  type User
} from 'discord.js';

import type { Env } from '../config/env.js';
import type { Logger } from '../lib/logger.js';
import type { AuditLogService } from './auditLogService.js';
import {
  AGENT_ACTION_STATUSES,
  AGENT_ACTION_TYPES,
  type AgentActionRecord,
  type AgentActionService
} from './agentActionService.js';
import type { MessageHistoryService } from './messageHistoryService.js';
import { CAPABILITIES, type RolePolicyService } from './rolePolicyService.js';

const CONFIRM_PREFIX = 'agent-action-confirm';
const CANCEL_PREFIX = 'agent-action-cancel';
const CONFIRMATION_TTL_MS = 15 * 60 * 1000;

export interface RelayConfirmationPrompt {
  action: AgentActionRecord;
  components: ActionRowBuilder<ButtonBuilder>[];
  reply: string;
}

export interface PendingConfirmationResult {
  reply: string;
}

export interface RequestRelayConfirmationInput {
  channelId: string | null;
  context?: string | null;
  guildId: string | null;
  message: string;
  metadata?: Record<string, unknown>;
  recipientUserId: string;
  recipientUsername: string;
  requesterUserId: string;
  requesterUsername: string;
}

export class ActionConfirmationService {
  constructor(
    private readonly env: Env,
    private readonly agentActions: AgentActionService,
    private readonly auditLogs: AuditLogService,
    private readonly messageHistory: MessageHistoryService,
    private readonly rolePolicies: RolePolicyService,
    private readonly logger: Logger
  ) {}

  matches(interaction: ButtonInteraction): boolean {
    return interaction.customId.startsWith(`${CONFIRM_PREFIX}:`)
      || interaction.customId.startsWith(`${CANCEL_PREFIX}:`);
  }

  async requestRelayConfirmation(input: RequestRelayConfirmationInput): Promise<RelayConfirmationPrompt> {
    const confirmationRequestedAt = new Date();
    const confirmationExpiresAt = new Date(confirmationRequestedAt.getTime() + CONFIRMATION_TTL_MS);
    const action = await this.agentActions.createDirectMessageRelay({
      guildId: input.guildId,
      channelId: input.channelId,
      requesterUserId: input.requesterUserId,
      requesterUsername: input.requesterUsername,
      recipientUserId: input.recipientUserId,
      recipientUsername: input.recipientUsername,
      message: input.message,
      context: input.context ?? null,
      initialStatus: AGENT_ACTION_STATUSES.awaitingConfirmation,
      confirmationRequestedAt: confirmationRequestedAt.toISOString(),
      confirmationExpiresAt: confirmationExpiresAt.toISOString(),
      metadata: {
        ...(input.metadata ?? {}),
        confirmationSurface: 'discord'
      }
    });

    await this.auditLogs.record({
      guildId: input.guildId,
      actorUserId: input.requesterUserId,
      action: 'agent_action.confirmation_requested',
      targetType: 'agent_action',
      targetId: action.id,
      metadata: {
        actionType: action.action_type,
        confirmationExpiresAt: confirmationExpiresAt.toISOString(),
        recipientUserId: input.recipientUserId,
        recipientUsername: input.recipientUsername
      }
    });

    return {
      action,
      components: buildConfirmationComponents(action.id),
      reply: [
        `I’m ready to DM ${input.recipientUsername} through Gigi.`,
        `Confirm within 15 minutes and I’ll send: "${input.message.trim()}"`,
        input.context?.trim() ? `Context: ${input.context.trim()}` : null
      ]
        .filter((line): line is string => Boolean(line))
        .join('\n')
    };
  }

  async maybeHandleTextConfirmation(
    query: string,
    requester: User,
    client: Client
  ): Promise<PendingConfirmationResult | null> {
    const actionMatch = query.match(/\baction\s+(?<id>[0-9a-f-]{8,})\b/i);
    const pending = await this.agentActions.listPendingConfirmationsForRequester(requester.id, 5);

    if (pending.length === 0) {
      return {
        reply: 'You have nothing pending to confirm or cancel right now.'
      };
    }

    const requestedActionId = actionMatch?.groups?.id ?? null;
    const requestedAction = requestedActionId
      ? pending.find((action) => action.id === requestedActionId)
      : null;
    const target = requestedAction ?? (pending.length === 1 ? pending[0] : null);

    if (!target) {
      return {
        reply: 'You have multiple pending Gigi actions. Use the confirm or cancel buttons on the specific action you want to resolve.'
      };
    }

    if (query.trim().match(/^(cancel|cancel it|never mind|nevermind|stop|don.?t send it)[.!?]*$/i)) {
      return this.cancelPendingAction(target, requester.id);
    }

    return this.confirmPendingAction(target, requester.id, client);
  }

  async handleButton(interaction: ButtonInteraction, client: Client): Promise<void> {
    const isConfirm = interaction.customId.startsWith(`${CONFIRM_PREFIX}:`);
    const actionId = interaction.customId
      .replace(`${CONFIRM_PREFIX}:`, '')
      .replace(`${CANCEL_PREFIX}:`, '');
    const action = await this.agentActions.getActionById(actionId);

    if (!action) {
      await interaction.reply({
        content: 'That pending Gigi action no longer exists.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    if (action.requester_user_id !== interaction.user.id) {
      await interaction.reply({
        content: 'That confirmation belongs to another user.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const result = isConfirm
      ? await this.confirmPendingAction(action, interaction.user.id, client)
      : await this.cancelPendingAction(action, interaction.user.id);

    await interaction.update({
      content: result.reply,
      components: []
    });
  }

  private async confirmPendingAction(
    action: AgentActionRecord,
    requesterUserId: string,
    client: Client
  ): Promise<PendingConfirmationResult> {
    const pending = await this.agentActions.getActionById(action.id);
    if (!pending) {
      return {
        reply: 'That pending Gigi action no longer exists.'
      };
    }

    if (pending.requester_user_id !== requesterUserId) {
      return {
        reply: 'That confirmation belongs to another user.'
      };
    }

    if (pending.status !== AGENT_ACTION_STATUSES.awaitingConfirmation) {
      return {
        reply: `That action is already ${humanizeStatus(pending.status)}.`
      };
    }

    if (isExpired(pending)) {
      await this.agentActions.markCancelled(pending, {
        metadata: {
          cancelledReason: 'confirmation_expired'
        },
        resultSummary: 'Relay confirmation expired before execution.'
      });
      await this.auditLogs.record({
        guildId: pending.guild_id,
        actorUserId: requesterUserId,
        action: 'agent_action.confirmation_expired',
        targetType: 'agent_action',
        targetId: pending.id,
        metadata: {
          actionType: pending.action_type
        }
      });

      return {
        reply: 'That pending DM relay expired. Ask me again if you still want me to send it.'
      };
    }

    if (pending.action_type !== AGENT_ACTION_TYPES.dmRelay) {
      return {
        reply: 'I can only confirm DM relay actions right now.'
      };
    }

    const guild = await this.resolvePrimaryGuild(client, pending.guild_id);
    const requesterMember = guild
      ? await guild.members.fetch(requesterUserId).catch(() => null)
      : null;
    const recipientMember = guild && pending.recipient_user_id
      ? await guild.members.fetch(pending.recipient_user_id).catch(() => null)
      : null;

    if (!(await this.hasDispatchPermission(guild, requesterMember))) {
      await this.auditLogs.record({
        guildId: pending.guild_id,
        actorUserId: requesterUserId,
        action: 'agent_action.confirmation_permission_denied',
        targetType: 'agent_action',
        targetId: pending.id,
        metadata: {
          reason: 'requester_missing_dispatch'
        }
      });

      await this.agentActions.markFailed(pending, 'Requester no longer has agent_action_dispatch permission.', {
        failedReason: 'requester_missing_dispatch'
      });

      return {
        reply: 'You no longer have permission to dispatch shared Gigi actions.'
      };
    }

    if (!(await this.hasReceivePermission(guild, recipientMember))) {
      await this.auditLogs.record({
        guildId: pending.guild_id,
        actorUserId: requesterUserId,
        action: 'agent_action.confirmation_permission_denied',
        targetType: 'agent_action',
        targetId: pending.id,
        metadata: {
          reason: recipientMember ? 'recipient_missing_capability' : 'recipient_not_in_guild',
          recipientUserId: pending.recipient_user_id
        }
      });

      await this.agentActions.markFailed(pending, 'Recipient no longer has agent_action_receive permission.', {
        failedReason: recipientMember ? 'recipient_missing_capability' : 'recipient_not_in_guild'
      });

      return {
        reply: 'I can only DM users through Gigi if that user is still in the primary server and still has `agent_action_receive` permission.'
      };
    }

    const inProgress = await this.agentActions.markInProgress(pending, {
      confirmedByUserId: requesterUserId,
      metadata: {
        confirmedFrom: 'discord'
      }
    });

    await this.auditLogs.record({
      guildId: pending.guild_id,
      actorUserId: requesterUserId,
      action: 'agent_action.confirmation_confirmed',
      targetType: 'agent_action',
      targetId: pending.id,
      metadata: {
        actionType: pending.action_type
      }
    });

    return this.executeRelay(inProgress, client);
  }

  private async cancelPendingAction(
    action: AgentActionRecord,
    requesterUserId: string
  ): Promise<PendingConfirmationResult> {
    if (action.requester_user_id !== requesterUserId) {
      return {
        reply: 'That confirmation belongs to another user.'
      };
    }

    if (action.status !== AGENT_ACTION_STATUSES.awaitingConfirmation) {
      return {
        reply: `That action is already ${humanizeStatus(action.status)}.`
      };
    }

    await this.agentActions.markCancelled(action, {
      metadata: {
        cancelledReason: 'requester_cancelled'
      },
      resultSummary: 'Requester cancelled the pending relay confirmation.'
    });

    await this.auditLogs.record({
      guildId: action.guild_id,
      actorUserId: requesterUserId,
      action: 'agent_action.confirmation_cancelled',
      targetType: 'agent_action',
      targetId: action.id,
      metadata: {
        actionType: action.action_type
      }
    });

    return {
      reply: 'Cancelled that pending DM relay.'
    };
  }

  private async executeRelay(
    action: AgentActionRecord,
    client: Client
  ): Promise<PendingConfirmationResult> {
    if (!action.recipient_user_id) {
      await this.agentActions.markFailed(action, 'Pending relay is missing a recipient user id.');
      return {
        reply: 'I could not send that relay because the recipient was missing.'
      };
    }

    const recipient = await client.users.fetch(action.recipient_user_id).catch(() => null);
    if (!recipient) {
      await this.agentActions.markFailed(action, 'Failed to resolve the relay recipient from Discord.');
      return {
        reply: 'I could not resolve that relay recipient anymore.'
      };
    }

    try {
      const sentMessage = await recipient.send({
        content: buildRelayContent(
          action.requester_username,
          action.instructions.trim(),
          typeof action.metadata.context === 'string' ? action.metadata.context : null
        )
      });

      let historyStored = false;
      try {
        await this.messageHistory.storeBotAuthoredMessage(sentMessage);
        historyStored = true;
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Unknown relay history persistence error';
        this.logger.error('Failed to persist outbound confirmed DM relay in canonical history', {
          actionId: action.id,
          error: message,
          recipientUserId: action.recipient_user_id,
          sentMessageId: sentMessage.id
        });
      }

      await this.agentActions.markCompleted(action, {
        metadata: {
          ...(action.metadata ?? {}),
          deliveredChannelId: sentMessage.channelId,
          deliveredMessageId: sentMessage.id,
          historyStored
        },
        resultSummary: `Delivered DM relay to ${action.recipient_username ?? recipient.username}`
      });

      await this.auditLogs.record({
        guildId: action.guild_id,
        actorUserId: action.requester_user_id,
        action: 'agent_action.relay.sent',
        targetType: 'agent_action',
        targetId: action.id,
        metadata: {
          recipientUserId: action.recipient_user_id,
          recipientUsername: action.recipient_username ?? recipient.username
        }
      });

      return {
        reply: `Confirmed and sent a DM relay to ${action.recipient_username ?? recipient.username}.`
      };
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unknown DM relay send error';

      await this.agentActions.markFailed(action, message, {
        failedReason: 'discord_send_failed'
      });
      await this.auditLogs.record({
        guildId: action.guild_id,
        actorUserId: action.requester_user_id,
        action: 'agent_action.relay.failed',
        targetType: 'agent_action',
        targetId: action.id,
        metadata: {
          error: message,
          recipientUserId: action.recipient_user_id,
          recipientUsername: action.recipient_username
        }
      });

      return {
        reply: `I confirmed that relay, but I could not DM ${action.recipient_username ?? 'that user'}. They may have direct messages disabled.`
      };
    }
  }

  private async resolvePrimaryGuild(client: Client, actionGuildId: string | null): Promise<Guild | null> {
    const guildId = actionGuildId ?? this.env.PRIMARY_GUILD_ID ?? this.env.DISCORD_GUILD_ID;
    if (!guildId) {
      return null;
    }

    return client.guilds.cache.get(guildId)
      ?? (await client.guilds.fetch(guildId).catch(() => null));
  }

  private async hasDispatchPermission(
    guild: Guild | null,
    requesterMember: GuildMember | null
  ): Promise<boolean> {
    if (!guild || !requesterMember) {
      return false;
    }

    return this.rolePolicies.memberHasCapability(
      guild,
      requesterMember,
      CAPABILITIES.agentActionDispatch
    );
  }

  private async hasReceivePermission(
    guild: Guild | null,
    recipientMember: GuildMember | null
  ): Promise<boolean> {
    if (!guild || !recipientMember) {
      return false;
    }

    return this.rolePolicies.memberHasCapability(
      guild,
      recipientMember,
      CAPABILITIES.agentActionReceive
    );
  }
}

function buildConfirmationComponents(actionId: string): ActionRowBuilder<ButtonBuilder>[] {
  return [
    new ActionRowBuilder<ButtonBuilder>().addComponents(
      new ButtonBuilder()
        .setCustomId(`${CONFIRM_PREFIX}:${actionId}`)
        .setLabel('Confirm DM')
        .setStyle(ButtonStyle.Success),
      new ButtonBuilder()
        .setCustomId(`${CANCEL_PREFIX}:${actionId}`)
        .setLabel('Cancel')
        .setStyle(ButtonStyle.Secondary)
    )
  ];
}

function buildRelayContent(requesterLabel: string, message: string, context: string | null): string {
  const lines = [
    `${requesterLabel} asked me to pass this along:`,
    '',
    message
  ];

  if (context && context.length > 0) {
    lines.push('', `Context: ${context}`);
  }

  lines.push('', 'You can ask me follow-up questions about this relay here in DM.');
  return lines.join('\n');
}

function humanizeStatus(status: AgentActionRecord['status']): string {
  return status.replaceAll('_', ' ');
}

function isExpired(action: AgentActionRecord): boolean {
  if (!action.confirmation_expires_at) {
    return false;
  }

  return Date.now() > Date.parse(action.confirmation_expires_at);
}
