import {
  ChannelType,
  EmbedBuilder,
  MessageFlags,
  SlashCommandBuilder,
  type ChatInputCommandInteraction,
  type GuildMember,
  type TextBasedChannel
} from 'discord.js';

import type { AssignmentRecord } from '../services/assignmentService.js';
import { CAPABILITIES } from '../services/rolePolicyService.js';
import type { BotContext, SlashCommand } from '../discord/types.js';

const assignmentCommandData = new SlashCommandBuilder()
  .setName('assignment')
  .setDescription('Create, publish, and list assignments.')
  .addSubcommand((subcommand) =>
    subcommand
      .setName('create')
      .setDescription('Create a draft assignment.')
      .addStringOption((option) =>
        option
          .setName('title')
          .setDescription('Short assignment title')
          .setRequired(true)
      )
      .addStringOption((option) =>
        option
          .setName('description')
          .setDescription('Assignment details')
          .setRequired(true)
      )
      .addStringOption((option) =>
        option
          .setName('affected_roles')
          .setDescription('Role mentions or role IDs separated by spaces or commas')
      )
      .addStringOption((option) =>
        option
          .setName('due_at')
          .setDescription('ISO-8601 timestamp, for example 2026-03-20T17:00:00Z')
      )
      .addChannelOption((option) =>
        option
          .setName('channel')
          .setDescription('Target announcement channel')
          .addChannelTypes(ChannelType.GuildText, ChannelType.GuildAnnouncement)
      )
  )
  .addSubcommand((subcommand) =>
    subcommand
      .setName('publish')
      .setDescription('Publish an existing draft assignment.')
      .addStringOption((option) =>
        option
          .setName('assignment_id')
          .setDescription('Assignment record ID')
          .setRequired(true)
      )
  )
  .addSubcommand((subcommand) =>
    subcommand
      .setName('list')
      .setDescription('List the most recent assignments.')
  );

export const assignmentCommand: SlashCommand = {
  data: assignmentCommandData,
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
      CAPABILITIES.assignmentAdmin
    );

    if (!allowed) {
      await interaction.reply({
        content: 'You do not have permission to manage assignments.',
        flags: MessageFlags.Ephemeral
      });
      return;
    }

    const subcommand = interaction.options.getSubcommand();

    if (subcommand === 'create') {
      await handleCreate(interaction, context);
      return;
    }

    if (subcommand === 'publish') {
      await handlePublish(interaction, context, member);
      return;
    }

    await handleList(interaction, context);
  }
};

async function handleCreate(
  interaction: ChatInputCommandInteraction,
  context: BotContext
): Promise<void> {
  if (!interaction.inGuild()) {
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

  const title = interaction.options.getString('title', true).trim();
  const description = interaction.options.getString('description', true).trim();
  const affectedRolesInput = interaction.options.getString('affected_roles');
  const dueAtInput = interaction.options.getString('due_at');
  const channel = interaction.options.getChannel('channel');

  const dueAt = parseDueAt(dueAtInput);
  const mentionedRoleIds = parseMentionedRoleIds(affectedRolesInput);

  if (dueAtInput && !dueAt) {
    await interaction.reply({
        content: 'Invalid `due_at`. Use a full ISO-8601 timestamp like `2026-03-20T17:00:00Z`.',
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  if (affectedRolesInput && mentionedRoleIds.length === 0) {
    await interaction.reply({
        content: 'No valid role IDs were found in `affected_roles`. Use role mentions like `<@&123>` or raw role IDs.',
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  const assignment = await context.services.assignments.createAssignment({
    guildId: guild.id,
    title,
    description,
    dueAt: dueAt?.toISOString() ?? null,
    announcementChannelId: channel?.id ?? null,
    mentionedRoleIds,
    createdByUserId: interaction.user.id
  });

  await interaction.reply({
    embeds: [buildAssignmentEmbed(assignment, 'Assignment draft created')],
    flags: MessageFlags.Ephemeral
  });
}

async function handlePublish(
  interaction: ChatInputCommandInteraction,
  context: BotContext,
  member: GuildMember
): Promise<void> {
  if (!interaction.inGuild()) {
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

  const assignmentId = interaction.options.getString('assignment_id', true);
  const assignment = await context.services.assignments.getAssignmentById(
    guild.id,
    assignmentId
  );

  if (!assignment) {
    await interaction.reply({
        content: `No assignment found for ID \`${assignmentId}\`.`,
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  const targetChannel = await resolveTargetChannel(interaction, assignment);

  if (!targetChannel) {
    await interaction.reply({
        content: 'Could not resolve a text channel for this assignment.',
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  if (!('send' in targetChannel)) {
    await interaction.reply({
        content: 'The resolved channel cannot send messages.',
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  const roleMentions = assignment.mentioned_role_ids.map((roleId) => `<@&${roleId}>`).join(' ');
  const announcementIntro = roleMentions.length > 0
    ? `${roleMentions}\nNew assignment notice from ${member}`
    : `New assignment notice from ${member}`;

  const sentMessage = await targetChannel.send({
    content: announcementIntro,
    embeds: [buildAssignmentEmbed(assignment, 'New assignment')]
  });

  const published = await context.services.assignments.markPublished(
    guild.id,
    assignment.id,
    sentMessage.id,
    targetChannel.id
  );

  await interaction.reply({
    embeds: [buildAssignmentEmbed(published, 'Assignment published')],
    flags: MessageFlags.Ephemeral
  });
}

async function handleList(
  interaction: ChatInputCommandInteraction,
  context: BotContext
): Promise<void> {
  if (!interaction.inGuild()) {
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

  const assignments = await context.services.assignments.listAssignments(guild.id);

  if (assignments.length === 0) {
    await interaction.reply({
        content: 'No assignments have been created yet.',
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  const lines = assignments.map((assignment) => {
    const dueLabel = assignment.due_at
      ? `<t:${Math.floor(new Date(assignment.due_at).getTime() / 1000)}:F>`
      : 'No due date';

    return [
      `**${assignment.title}**`,
      `ID: \`${assignment.id}\``,
      `Status: \`${assignment.status}\``,
      `Due: ${dueLabel}`
    ].join('\n');
  });

  await interaction.reply({
    embeds: [
      new EmbedBuilder()
        .setTitle('Recent assignments')
        .setDescription(lines.join('\n\n'))
        .setColor(0x5865f2)
    ],
    flags: MessageFlags.Ephemeral
  });
}

function parseDueAt(value: string | null): Date | null {
  if (!value) {
    return null;
  }

  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
}

function buildAssignmentEmbed(
  assignment: AssignmentRecord,
  title: string
): EmbedBuilder {
  const embed = new EmbedBuilder()
    .setTitle(title)
    .setColor(0x5865f2)
    .addFields(
      { name: 'Assignment ID', value: assignment.id },
      { name: 'Title', value: assignment.title },
      { name: 'Status', value: assignment.status }
    )
    .setDescription(assignment.description)
    .setTimestamp(new Date(assignment.updated_at));

  if (assignment.due_at) {
    embed.addFields({
      name: 'Due',
      value: `<t:${Math.floor(new Date(assignment.due_at).getTime() / 1000)}:F>`
    });
  }

  if (assignment.announcement_channel_id) {
    embed.addFields({
      name: 'Announcement channel',
      value: `<#${assignment.announcement_channel_id}>`
    });
  }

  if (assignment.mentioned_role_ids.length > 0) {
    embed.addFields({
      name: 'Affected roles',
      value: assignment.mentioned_role_ids.map((roleId) => `<@&${roleId}>`).join(' ')
    });
  }

  return embed;
}

async function resolveTargetChannel(
  interaction: ChatInputCommandInteraction,
  assignment: AssignmentRecord
): Promise<TextBasedChannel | null> {
  const currentChannel = interaction.channel;

  if (assignment.announcement_channel_id) {
    const resolved = await interaction.guild?.channels.fetch(assignment.announcement_channel_id);
    if (resolved?.isTextBased()) {
      return resolved;
    }
  }

  if (currentChannel?.isTextBased()) {
    return currentChannel;
  }

  return null;
}

function parseMentionedRoleIds(value: string | null): string[] {
  if (!value) {
    return [];
  }

  const matches = value.match(/\d{16,20}/g) ?? [];
  return [...new Set(matches)];
}
