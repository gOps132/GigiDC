import { SlashCommandBuilder } from 'discord.js';

import type { SlashCommand } from '../discord/types.js';
import { CAPABILITIES } from '../services/rolePolicyService.js';

export const generateCommand: SlashCommand = {
  data: new SlashCommandBuilder()
    .setName('generate')
    .setDescription('Send content generation work to Clawbot.')
    .addSubcommand((subcommand) =>
      subcommand
        .setName('tests')
        .setDescription('Generate tests for a given subject or diff.')
        .addStringOption((option) =>
          option.setName('subject').setDescription('What Clawbot should generate tests for').setRequired(true)
        )
    )
    .addSubcommand((subcommand) =>
      subcommand
        .setName('quiz')
        .setDescription('Generate a quiz from a topic or notes.')
        .addStringOption((option) =>
          option.setName('topic').setDescription('Topic or context for the quiz').setRequired(true)
        )
    )
    .addSubcommand((subcommand) =>
      subcommand
        .setName('summary')
        .setDescription('Generate a summary of a topic or prompt.')
        .addStringOption((option) =>
          option.setName('topic').setDescription('Topic or prompt to summarize').setRequired(true)
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
        content: 'You do not have permission to dispatch Clawbot generation jobs.',
        ephemeral: true
      });
      return;
    }

    const subcommand = interaction.options.getSubcommand();
    const content =
      interaction.options.getString('subject') ??
      interaction.options.getString('topic');

    await interaction.deferReply({ ephemeral: true });

    const job = await context.services.clawbotDispatch.dispatch({
      guildId: guild.id,
      channelId: interaction.channelId,
      threadId: interaction.channel?.isThread() ? interaction.channel.id : null,
      requesterUserId: interaction.user.id,
      commandName: 'generate',
      taskType: `generate_${subcommand}`,
      input: {
        subcommand,
        content
      }
    });

    await interaction.editReply({
      content: `Queued Clawbot generation job \`${job.id}\`. Results will be posted back here when ready.`
    });
  }
};
