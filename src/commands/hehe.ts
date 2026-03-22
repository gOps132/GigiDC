import { MessageFlags, SlashCommandBuilder } from 'discord.js';

import type { SlashCommand } from '../discord/types.js';

export const heheCommand: SlashCommand = {
  data: new SlashCommandBuilder()
    .setName('hehe')
    .setDescription('Reply with hoho.'),
  async execute(interaction) {
    await interaction.reply({
      content: 'hoho',
      flags: MessageFlags.Ephemeral
    });
  }
};
