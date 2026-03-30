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
    const targetMember = await guild.members.fetch(targetUser.id).catch(() => null);

    if (!targetMember) {
      await context.services.auditLogs.record({
        guildId: guild.id,
        actorUserId: interaction.user.id,
        action: 'relay.dm.recipient_permission_denied',
        targetType: 'agent_action',
        targetId: null,
        metadata: {
          reason: 'recipient_not_in_guild',
          recipientUserId: targetUser.id,
          recipientUsername: targetUser.username
        }
      });

      await interaction.reply({
        content: 'I can only DM users through Gigi if they are in this server and have relay-receive permission.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const recipientAllowed = await context.services.rolePolicies.memberHasCapability(
      guild,
      targetMember,
      CAPABILITIES.agentActionReceive
    );

    if (!recipientAllowed) {
      await context.services.auditLogs.record({
        guildId: guild.id,
        actorUserId: interaction.user.id,
        action: 'relay.dm.recipient_permission_denied',
        targetType: 'agent_action',
        targetId: null,
        metadata: {
          reason: 'recipient_missing_capability',
          recipientUserId: targetUser.id,
          recipientUsername: targetUser.username
        }
      });

      await interaction.reply({
        content: 'I can only DM users through Gigi if that user has `agent_action_receive` permission.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    if (relayMessage.length === 0) {
      await interaction.reply({
        content: 'The relay message cannot be empty.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const confirmation = await context.services.actionConfirmations.requestRelayConfirmation({
      channelId: interaction.channelId,
      context: relayContext,
      guildId: guild.id,
      message: relayMessage,
      metadata: {
        createdFrom: 'slash_command',
        guildName: guild.name,
        relayChannelId: interaction.channelId
      },
      recipientUserId: targetUser.id,
      recipientUsername: targetUser.username,
      requesterUserId: interaction.user.id,
      requesterUsername: member.displayName
    });

    await interaction.reply({
      content: confirmation.reply,
      components: confirmation.components,
      flags: MessageFlags.Ephemeral
    });
  }
};
