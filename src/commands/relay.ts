import {
  MessageFlags,
  SlashCommandBuilder,
  type ChatInputCommandInteraction
} from 'discord.js';

import type { BotContext, SlashCommand } from '../discord/types.js';
import { CAPABILITIES } from '../services/rolePolicyService.js';

const relayCommandData = new SlashCommandBuilder()
  .setName('relay')
  .setDescription('Ask Gigi to relay a message across surfaces.')
  .addSubcommand((subcommand) =>
    subcommand
      .setName('dm')
      .setDescription('Send a direct message to a user through Gigi.')
      .addUserOption((option) =>
        option
          .setName('user')
          .setDescription('The user who should receive the DM')
          .setRequired(true)
      )
      .addStringOption((option) =>
        option
          .setName('message')
          .setDescription('The message Gigi should send')
          .setRequired(true)
      )
      .addStringOption((option) =>
        option
          .setName('context')
          .setDescription('Optional context Gigi should remember with the relay')
      )
  );

export const relayCommand: SlashCommand = {
  data: relayCommandData,
  async execute(interaction, context) {
    if (!interaction.inGuild()) {
      await interaction.reply({
        content: 'This command can only be used in a server.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const guild = interaction.guild;
    if (!guild) {
      await interaction.reply({
        content: 'Guild context was not available for this command.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const member = await guild.members.fetch(interaction.user.id);
    const allowed = await context.services.rolePolicies.memberHasCapability(
      guild,
      member,
      CAPABILITIES.agentActionDispatch
    );

    if (!allowed) {
      await context.services.auditLogs.record({
        guildId: guild.id,
        actorUserId: interaction.user.id,
        action: 'relay.dm.permission_denied',
        targetType: 'agent_action',
        targetId: null,
        metadata: {
          command: 'relay',
          subcommand: interaction.options.getSubcommand()
        }
      });

      await interaction.reply({
        content: 'You do not have permission to dispatch shared Gigi actions.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const subcommand = interaction.options.getSubcommand();
    if (subcommand !== 'dm') {
      await interaction.reply({
        content: 'Unknown relay subcommand.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const targetUser = interaction.options.getUser('user', true);
    const relayMessage = interaction.options.getString('message', true).trim();
    const relayContext = interaction.options.getString('context')?.trim() ?? null;

    if (relayMessage.length === 0) {
      await interaction.reply({
        content: 'The relay message cannot be empty.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const action = await context.services.agentActions.createDirectMessageRelay({
      guildId: guild.id,
      channelId: interaction.channelId,
      requesterUserId: interaction.user.id,
      requesterUsername: member.displayName,
      recipientUserId: targetUser.id,
      recipientUsername: targetUser.username,
      message: relayMessage,
      context: relayContext,
      metadata: {
        guildName: guild.name,
        relayChannelId: interaction.channelId
      }
    });

    try {
      const sentMessage = await targetUser.send({
        content: buildRelayContent(member.displayName, relayMessage, relayContext)
      });

      let historyStored = false;
      try {
        await context.services.messageHistory.storeBotAuthoredMessage(sentMessage);
        historyStored = true;
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Unknown relay history persistence error';
        context.logger.error('Failed to persist outbound relay DM in canonical history', {
          actionId: action.id,
          error: message,
          recipientUserId: targetUser.id,
          sentMessageId: sentMessage.id
        });
      }

      await context.services.agentActions.markCompleted(action, {
        metadata: {
          deliveredChannelId: sentMessage.channelId,
          deliveredMessageId: sentMessage.id,
          historyStored
        },
        resultSummary: `Delivered DM relay to ${targetUser.username}`
      });

      await context.services.auditLogs.record({
        guildId: guild.id,
        actorUserId: interaction.user.id,
        action: 'relay.dm.sent',
        targetType: 'agent_action',
        targetId: action.id,
        metadata: {
          recipientUserId: targetUser.id,
          recipientUsername: targetUser.username
        }
      });

      await interaction.reply({
        content: `Sent a DM to ${targetUser}.`,
        flags: MessageFlags.Ephemeral
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unknown DM relay error';

      await context.services.agentActions.markFailed(action, message);
      await context.services.auditLogs.record({
        guildId: guild.id,
        actorUserId: interaction.user.id,
        action: 'relay.dm.failed',
        targetType: 'agent_action',
        targetId: action.id,
        metadata: {
          error: message,
          recipientUserId: targetUser.id,
          recipientUsername: targetUser.username
        }
      });

      await interaction.reply({
        content: `I could not DM ${targetUser}. They may have direct messages disabled.`,
        flags: MessageFlags.Ephemeral
      });
    }
  }
};

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
