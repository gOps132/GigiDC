import { SlashCommandBuilder } from 'discord.js';

import type { BotContext, SlashCommand } from '../discord/types.js';
import { CAPABILITIES } from '../services/rolePolicyService.js';

export const reviewCommand: SlashCommand = {
  data: new SlashCommandBuilder()
    .setName('review')
    .setDescription('Send code review work to Clawbot.')
    .addSubcommand((subcommand) =>
      subcommand
        .setName('pr')
        .setDescription('Review a pull request or diff via Clawbot.')
        .addStringOption((option) =>
          option
            .setName('reference')
            .setDescription('Pull request URL, number, or diff reference')
            .setRequired(true)
        )
        .addStringOption((option) =>
          option
            .setName('instructions')
            .setDescription('Optional instructions for Clawbot')
        )
    ),
  async execute(interaction, context) {
    if (!interaction.inGuild()) {
      await interaction.reply({ content: 'This command can only be used in a server.', ephemeral: true });
      return;
    }

    const guild = interaction.guild;
    if (!guild) {
      await interaction.reply({ content: 'Guild context was not available.', ephemeral: true });
      return;
    }

    const member = await guild.members.fetch(interaction.user.id);
    const allowed = await context.services.rolePolicies.memberHasCapability(
      guild,
      member,
      CAPABILITIES.clawbotDispatch
    );

    if (!allowed) {
      await interaction.reply({
        content: 'You do not have permission to dispatch Clawbot review jobs.',
        ephemeral: true
      });
      return;
    }

    await interaction.deferReply({ ephemeral: true });

    const job = await context.services.clawbotDispatch.dispatch({
      guildId: guild.id,
      channelId: interaction.channelId,
      threadId: interaction.channel?.isThread() ? interaction.channel.id : null,
      requesterUserId: interaction.user.id,
      commandName: 'review',
      taskType: 'review_pr',
      input: {
        reference: interaction.options.getString('reference', true),
        instructions: interaction.options.getString('instructions')
      }
    });

    await interaction.editReply({
      content: `Queued Clawbot review job \`${job.id}\`. Results will be posted back here when the callback arrives.`
    });
  }
};
