import {
  EmbedBuilder,
  MessageFlags,
  SlashCommandBuilder
} from 'discord.js';

import type { BotContext, SlashCommand } from '../discord/types.js';

const usageCommandData = new SlashCommandBuilder()
  .setName('usage')
  .setDescription('Inspect Gigi token usage and estimated cost.')
  .addSubcommand((subcommand) =>
    subcommand
      .setName('summary')
      .setDescription('Show recent server-wide usage summary.')
      .addIntegerOption((option) =>
        option
          .setName('days')
          .setDescription('How many recent days to include. Defaults to 7.')
          .setMinValue(1)
          .setMaxValue(30)
      )
  )
  .addSubcommand((subcommand) =>
    subcommand
      .setName('user')
      .setDescription('Show recent usage summary for one user.')
      .addUserOption((option) =>
        option
          .setName('user')
          .setDescription('The user to inspect.')
          .setRequired(true)
      )
      .addIntegerOption((option) =>
        option
          .setName('days')
          .setDescription('How many recent days to include. Defaults to 7.')
          .setMinValue(1)
          .setMaxValue(30)
      )
  );

export const usageCommand: SlashCommand = {
  data: usageCommandData,
  async execute(interaction, context) {
    if (!interaction.inGuild()) {
      await interaction.reply({
        content: 'This command can only be used in a server.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    if (!interaction.guild) {
      await interaction.reply({
        content: 'Guild context was not available for this command.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const days = interaction.options.getInteger('days') ?? 7;
    const subcommand = interaction.options.getSubcommand();

    const summary = subcommand === 'user'
      ? await context.services.usageAdmin.getUserUsageSummary({
          client: interaction.client,
          days,
          guild: interaction.guild,
          requester: interaction.user,
          targetUser: interaction.options.getUser('user', true)
        })
      : await context.services.usageAdmin.getUsageSummary({
          client: interaction.client,
          days,
          guild: interaction.guild,
          requester: interaction.user
        });

    await interaction.reply({
      embeds: [
        new EmbedBuilder()
          .setTitle(subcommand === 'user' ? 'User usage summary' : 'Usage summary')
          .setDescription(summary)
          .setColor(0x5865f2)
      ],
      flags: MessageFlags.Ephemeral
    });
  }
};
