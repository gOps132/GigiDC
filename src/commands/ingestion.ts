import {
  ChannelType,
  EmbedBuilder,
  MessageFlags,
  SlashCommandBuilder,
  type ChatInputCommandInteraction,
  type GuildMember
} from 'discord.js';

import type { BotContext, SlashCommand } from '../discord/types.js';
import type { ChannelIngestionPolicyRecord } from '../services/channelIngestionPolicyService.js';
import { CAPABILITIES } from '../services/rolePolicyService.js';

const ingestionCommandData = new SlashCommandBuilder()
  .setName('ingestion')
  .setDescription('Manage guild-channel history ingestion.')
  .addSubcommand((subcommand) =>
    subcommand
      .setName('enable')
      .setDescription('Enable message-history ingestion for a channel.')
      .addChannelOption((option) =>
        option
          .setName('channel')
          .setDescription('Channel or thread to enable. Defaults to the current channel.')
          .addChannelTypes(
            ChannelType.GuildText,
            ChannelType.GuildAnnouncement,
            ChannelType.PublicThread,
            ChannelType.PrivateThread,
            ChannelType.AnnouncementThread
          )
      )
  )
  .addSubcommand((subcommand) =>
    subcommand
      .setName('disable')
      .setDescription('Disable message-history ingestion for a channel.')
      .addChannelOption((option) =>
        option
          .setName('channel')
          .setDescription('Channel or thread to disable. Defaults to the current channel.')
          .addChannelTypes(
            ChannelType.GuildText,
            ChannelType.GuildAnnouncement,
            ChannelType.PublicThread,
            ChannelType.PrivateThread,
            ChannelType.AnnouncementThread
          )
      )
  )
  .addSubcommand((subcommand) =>
    subcommand
      .setName('status')
      .setDescription('Show whether message-history ingestion is enabled for a channel.')
      .addChannelOption((option) =>
        option
          .setName('channel')
          .setDescription('Channel or thread to inspect. Defaults to the current channel.')
          .addChannelTypes(
            ChannelType.GuildText,
            ChannelType.GuildAnnouncement,
            ChannelType.PublicThread,
            ChannelType.PrivateThread,
            ChannelType.AnnouncementThread
          )
      )
  );

export const ingestionCommand: SlashCommand = {
  data: ingestionCommandData,
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

    const targetChannel = resolveTargetChannel(interaction);
    if (!targetChannel) {
      await interaction.reply({
        content: 'Choose a text channel or thread, or run this command from one.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const member = await guild.members.fetch(interaction.user.id);
    const allowed = await context.services.rolePolicies.memberHasCapability(
      guild,
      member,
      CAPABILITIES.ingestionAdmin
    );

    if (!allowed) {
      await context.services.auditLogs.record({
        guildId: guild.id,
        actorUserId: interaction.user.id,
        action: 'ingestion.permission_denied',
        targetType: 'channel_ingestion_policy',
        targetId: targetChannel.id,
        metadata: {
          channelId: targetChannel.id,
          channelName: targetChannel.name
        }
      });

      await interaction.reply({
        content: 'You do not have permission to manage ingestion policies.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const subcommand = interaction.options.getSubcommand();
    if (subcommand === 'status') {
      await handleStatus(interaction, context, targetChannel.id, targetChannel.name);
      return;
    }

    await handleSetState(
      interaction,
      context,
      member,
      targetChannel.id,
      targetChannel.name,
      subcommand === 'enable'
    );
  }
};

async function handleSetState(
  interaction: ChatInputCommandInteraction,
  context: BotContext,
  member: GuildMember,
  channelId: string,
  channelLabel: string,
  enabled: boolean
): Promise<void> {
  const guild = interaction.guild;
  if (!guild) {
    return;
  }

  const previous = await context.services.channelIngestionPolicies.getPolicy(guild.id, channelId);
  const updated = await context.services.channelIngestionPolicies.setChannelEnabled({
    channelId,
    enabled,
    guildId: guild.id,
    updatedByUserId: interaction.user.id
  });

  const changed = previous?.enabled !== updated.enabled;
  await context.services.auditLogs.record({
    guildId: guild.id,
    actorUserId: interaction.user.id,
    action: changed
      ? updated.enabled ? 'ingestion.enabled' : 'ingestion.disabled'
        : updated.enabled ? 'ingestion.enable_noop' : 'ingestion.disable_noop',
    targetType: 'channel_ingestion_policy',
    targetId: channelId,
    metadata: {
      channelId,
      channelLabel,
      previousEnabled: previous?.enabled ?? false,
      updatedEnabled: updated.enabled,
      updatedByMemberId: member.id
    }
  });

  await interaction.reply({
    embeds: [
      buildPolicyEmbed(
        updated,
        channelId,
        channelLabel,
        changed
          ? updated.enabled
            ? 'Ingestion enabled'
            : 'Ingestion disabled'
          : updated.enabled
            ? 'Ingestion already enabled'
            : 'Ingestion already disabled'
      )
    ],
    flags: MessageFlags.Ephemeral
  });
}

async function handleStatus(
  interaction: ChatInputCommandInteraction,
  context: BotContext,
  channelId: string,
  channelLabel: string
): Promise<void> {
  const guild = interaction.guild;
  if (!guild) {
    return;
  }

  const policy = await context.services.channelIngestionPolicies.getPolicy(guild.id, channelId);

  await interaction.reply({
    embeds: [
      buildPolicyEmbed(
        policy,
        channelId,
        channelLabel,
        policy?.enabled ? 'Ingestion enabled' : 'Ingestion disabled'
      )
    ],
    flags: MessageFlags.Ephemeral
  });
}

function buildPolicyEmbed(
  policy: ChannelIngestionPolicyRecord | null,
  channelId: string,
  channelLabel: string,
  title: string
): EmbedBuilder {
  const embed = new EmbedBuilder()
    .setTitle(title)
    .setColor(policy?.enabled ? 0x2ecc71 : 0xe67e22)
    .addFields(
      {
        name: 'Channel',
        value: `${channelLabel}\n<#${channelId}>`
      },
      {
        name: 'Enabled',
        value: policy?.enabled ? 'Yes' : 'No'
      }
    );

  if (policy?.updated_by_user_id) {
    embed.addFields({
      name: 'Last updated by',
      value: `<@${policy.updated_by_user_id}>`
    });
  }

  if (policy?.updated_at) {
    embed.setTimestamp(new Date(policy.updated_at));
  }

  return embed;
}

function resolveTargetChannel(
  interaction: ChatInputCommandInteraction
): { id: string; name: string } | null {
  const selectedChannel = interaction.options.getChannel('channel');
  if (selectedChannel) {
    return {
      id: selectedChannel.id,
      name: String((selectedChannel as { name?: string }).name ?? selectedChannel.id)
    };
  }

  const currentChannel = interaction.channel;
  if (currentChannel && 'name' in currentChannel) {
    return {
      id: currentChannel.id,
      name: currentChannel.name ?? currentChannel.id
    };
  }

  return null;
}
