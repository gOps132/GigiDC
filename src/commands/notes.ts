import { SlashCommandBuilder } from 'discord.js';

import type { SlashCommand } from '../discord/types.js';
import { CAPABILITIES } from '../services/rolePolicyService.js';

export const notesCommand: SlashCommand = {
  data: new SlashCommandBuilder()
    .setName('notes')
    .setDescription('Analyze notes with Clawbot.')
    .addSubcommand((subcommand) =>
      subcommand
        .setName('analyze')
        .setDescription('Analyze pasted notes or context.')
        .addStringOption((option) =>
          option.setName('content').setDescription('Notes content to analyze').setRequired(true)
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
        content: 'You do not have permission to dispatch Clawbot note analysis jobs.',
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
      commandName: 'notes',
      taskType: 'notes_analyze',
      input: {
        content: interaction.options.getString('content', true)
      }
    });

    await interaction.editReply({
      content: `Queued Clawbot notes job \`${job.id}\`. Results will be posted back here when the callback arrives.`
    });
  }
};
