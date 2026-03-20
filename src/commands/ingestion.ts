import { ChannelType, SlashCommandBuilder } from 'discord.js';

import type { BotContext, SlashCommand } from '../discord/types.js';
import { CAPABILITIES } from '../services/rolePolicyService.js';

const ingestionCommandData = new SlashCommandBuilder()
  .setName('ingestion')
  .setDescription('Manage whether channel history is forwarded to Clawbot.')
  .addSubcommand((subcommand) =>
    subcommand
      .setName('enable')
      .setDescription('Enable Clawbot history ingestion for a channel.')
      .addChannelOption((option) =>
        option
          .setName('channel')
          .setDescription('Channel to enable')
          .setRequired(true)
          .addChannelTypes(ChannelType.GuildText, ChannelType.GuildAnnouncement)
      )
  )
  .addSubcommand((subcommand) =>
    subcommand
      .setName('disable')
      .setDescription('Disable Clawbot history ingestion for a channel.')
      .addChannelOption((option) =>
        option
          .setName('channel')
          .setDescription('Channel to disable')
          .setRequired(true)
          .addChannelTypes(ChannelType.GuildText, ChannelType.GuildAnnouncement)
      )
  )
  .addSubcommand((subcommand) =>
    subcommand
      .setName('status')
      .setDescription('Show ingestion status for a channel.')
      .addChannelOption((option) =>
        option
          .setName('channel')
          .setDescription('Channel to inspect')
          .setRequired(true)
          .addChannelTypes(ChannelType.GuildText, ChannelType.GuildAnnouncement)
      )
  );

export const ingestionCommand: SlashCommand = {
  data: ingestionCommandData,
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
    const subcommand = interaction.options.getSubcommand();
    const channel = interaction.options.getChannel('channel', true);

    if (subcommand === 'status') {
      const enabled = await context.services.channelIngestionPolicies.isEnabled(guild.id, channel.id);
      await interaction.reply({
        content: `Clawbot history ingestion is currently **${enabled ? 'enabled' : 'disabled'}** for <#${channel.id}>.`,
        ephemeral: true
      });
      return;
    }

    const allowed = await context.services.rolePolicies.memberHasCapability(
      guild,
      member,
      CAPABILITIES.ingestionAdmin
    );

    if (!allowed) {
      await interaction.reply({
        content: 'You do not have permission to manage Clawbot ingestion.',
        ephemeral: true
      });
      return;
    }

    const enabled = subcommand === 'enable';
    await context.services.channelIngestionPolicies.setPolicy(
      guild.id,
      channel.id,
      enabled,
      interaction.user.id
    );

    await context.services.auditLogs.record({
      guildId: guild.id,
      actorUserId: interaction.user.id,
      action: enabled ? 'channel_ingestion_enabled' : 'channel_ingestion_disabled',
      targetType: 'discord_channel',
      targetId: channel.id
    });

    await interaction.reply({
      content: `Clawbot history ingestion ${enabled ? 'enabled' : 'disabled'} for <#${channel.id}>.`,
      ephemeral: true
    });
  }
};
