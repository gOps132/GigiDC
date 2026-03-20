import {
  ChannelType,
  EmbedBuilder,
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
        ephemeral: true
      });
      return;
    }

    const guild = interaction.guild;
    if (!guild) {
      await interaction.reply({
        content: 'Guild context was not available for this command.',
        ephemeral: true
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
        ephemeral: true
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
      ephemeral: true
    });
    return;
  }

  const title = interaction.options.getString('title', true).trim();
  const description = interaction.options.getString('description', true).trim();
  const dueAtInput = interaction.options.getString('due_at');
  const channel = interaction.options.getChannel('channel');

  const dueAt = parseDueAt(dueAtInput);

  if (dueAtInput && !dueAt) {
    await interaction.reply({
      content: 'Invalid `due_at`. Use a full ISO-8601 timestamp like `2026-03-20T17:00:00Z`.',
      ephemeral: true
    });
    return;
  }

  const assignment = await context.services.assignments.createAssignment({
    guildId: guild.id,
    title,
    description,
    dueAt: dueAt?.toISOString() ?? null,
    announcementChannelId: channel?.id ?? null,
    createdByUserId: interaction.user.id
  });

  await interaction.reply({
    embeds: [buildAssignmentEmbed(assignment, 'Assignment draft created')],
    ephemeral: true
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
      ephemeral: true
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
      ephemeral: true
    });
    return;
  }

  const targetChannel = await resolveTargetChannel(interaction, assignment);

  if (!targetChannel) {
    await interaction.reply({
      content: 'Could not resolve a text channel for this assignment.',
      ephemeral: true
    });
    return;
  }

  if (!('send' in targetChannel)) {
    await interaction.reply({
      content: 'The resolved channel cannot send messages.',
      ephemeral: true
    });
    return;
  }

  const sentMessage = await targetChannel.send({
    content: `Assignment announcement from ${member}`,
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
    ephemeral: true
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
      ephemeral: true
    });
    return;
  }

  const assignments = await context.services.assignments.listAssignments(guild.id);

  if (assignments.length === 0) {
    await interaction.reply({
      content: 'No assignments have been created yet.',
      ephemeral: true
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
    ephemeral: true
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
