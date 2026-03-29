import {
  EmbedBuilder,
  MessageFlags,
  SlashCommandBuilder
} from 'discord.js';

import type { BotContext, SlashCommand } from '../discord/types.js';
import {
  AGENT_ACTION_STATUSES,
  canUserAccessAction,
  isTaskAction,
  type AgentActionRecord
} from '../services/agentActionService.js';
import { CAPABILITIES } from '../services/rolePolicyService.js';

const taskCommandData = new SlashCommandBuilder()
  .setName('task')
  .setDescription('Create and manage shared Gigi tasks.')
  .addSubcommand((subcommand) =>
    subcommand
      .setName('create')
      .setDescription('Create a follow-up task in Gigi memory.')
      .addStringOption((option) =>
        option
          .setName('title')
          .setDescription('Short task title')
          .setRequired(true)
      )
      .addStringOption((option) =>
        option
          .setName('details')
          .setDescription('Task details or instructions')
          .setRequired(true)
      )
      .addUserOption((option) =>
        option
          .setName('user')
          .setDescription('Optional assignee. Defaults to you.')
      )
      .addStringOption((option) =>
        option
          .setName('due_at')
          .setDescription('Optional ISO-8601 due date, for example 2026-04-01T09:00:00Z')
      )
  )
  .addSubcommand((subcommand) =>
    subcommand
      .setName('list')
      .setDescription('List open shared Gigi tasks.')
      .addUserOption((option) =>
        option
          .setName('user')
          .setDescription('Whose open tasks to view. Defaults to you.')
      )
  )
  .addSubcommand((subcommand) =>
    subcommand
      .setName('complete')
      .setDescription('Mark a task as completed.')
      .addStringOption((option) =>
        option
          .setName('task_id')
          .setDescription('The task action ID')
          .setRequired(true)
      )
      .addStringOption((option) =>
        option
          .setName('result')
          .setDescription('Optional completion note')
      )
  );

export const taskCommand: SlashCommand = {
  data: taskCommandData,
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
    const canDispatch = await context.services.rolePolicies.memberHasCapability(
      guild,
      member,
      CAPABILITIES.agentActionDispatch
    );

    const subcommand = interaction.options.getSubcommand();

    if (subcommand === 'create') {
      await handleCreate(interaction, context, guild.id, guild.name, member.displayName, canDispatch);
      return;
    }

    if (subcommand === 'list') {
      await handleList(interaction, context, guild.id, canDispatch);
      return;
    }

    await handleComplete(interaction, context, guild.id, canDispatch);
  }
};

async function handleCreate(
  interaction: Parameters<SlashCommand['execute']>[0],
  context: BotContext,
  guildId: string,
  guildName: string,
  requesterLabel: string,
  canDispatch: boolean
): Promise<void> {
  if (!canDispatch) {
    await context.services.auditLogs.record({
      guildId,
      actorUserId: interaction.user.id,
      action: 'task.create.permission_denied',
      targetType: 'agent_action',
      metadata: {
        command: 'task',
        subcommand: 'create'
      }
    });

    await interaction.reply({
      content: 'You do not have permission to create shared Gigi tasks.',
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  const title = interaction.options.getString('title', true).trim();
  const details = interaction.options.getString('details', true).trim();
  const dueAtInput = interaction.options.getString('due_at');
  const dueAt = parseDueAt(dueAtInput);

  if (dueAtInput && !dueAt) {
    await interaction.reply({
      content: 'Invalid `due_at`. Use a full ISO-8601 timestamp like `2026-04-01T09:00:00Z`.',
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  const assignee = interaction.options.getUser('user') ?? interaction.user;
  const task = await context.services.agentActions.createFollowUpTask({
    guildId,
    channelId: interaction.channelId,
    requesterUserId: interaction.user.id,
    requesterUsername: requesterLabel,
    assigneeUserId: assignee.id,
    assigneeUsername: assignee.username,
    title,
    instructions: details,
    dueAt: dueAt?.toISOString() ?? null,
    metadata: {
      createdFromCommand: 'task.create',
      guildName
    }
  });

  await context.services.auditLogs.record({
    guildId,
    actorUserId: interaction.user.id,
    action: 'task.created',
    targetType: 'agent_action',
    targetId: task.id,
    metadata: {
      assigneeUserId: assignee.id,
      assigneeUsername: assignee.username,
      dueAt: task.due_at
    }
  });

  await interaction.reply({
    embeds: [buildTaskEmbed(task, 'Task created')],
    flags: MessageFlags.Ephemeral
  });
}

async function handleList(
  interaction: Parameters<SlashCommand['execute']>[0],
  context: BotContext,
  guildId: string,
  canDispatch: boolean
): Promise<void> {
  const targetUser = interaction.options.getUser('user') ?? interaction.user;
  if (targetUser.id !== interaction.user.id && !canDispatch) {
    await context.services.auditLogs.record({
      guildId,
      actorUserId: interaction.user.id,
      action: 'task.list.permission_denied',
      targetType: 'agent_action',
      metadata: {
        requestedUserId: targetUser.id
      }
    });

    await interaction.reply({
      content: 'You can only list your own tasks unless you have shared-action dispatch access.',
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  const tasks = await context.services.agentActions.listOpenTasksForUser(targetUser.id, 10);
  if (tasks.length === 0) {
    await interaction.reply({
      content: targetUser.id === interaction.user.id
        ? 'You have no open Gigi tasks.'
        : `${targetUser.username} has no open Gigi tasks.`,
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  await interaction.reply({
    embeds: [
      new EmbedBuilder()
        .setTitle(targetUser.id === interaction.user.id ? 'Your open Gigi tasks' : `${targetUser.username}'s open Gigi tasks`)
        .setColor(0x3498db)
        .setDescription(tasks.map((task) => formatTaskSummary(task)).join('\n\n'))
    ],
    flags: MessageFlags.Ephemeral
  });
}

async function handleComplete(
  interaction: Parameters<SlashCommand['execute']>[0],
  context: BotContext,
  guildId: string,
  canDispatch: boolean
): Promise<void> {
  const taskId = interaction.options.getString('task_id', true);
  const result = interaction.options.getString('result')?.trim() ?? null;
  const task = await context.services.agentActions.getActionById(taskId);

  if (!task || !isTaskAction(task)) {
    await interaction.reply({
      content: `No task found for ID \`${taskId}\`.`,
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  const canAccess = canUserAccessAction(task, interaction.user.id);
  if (!canAccess && !canDispatch) {
    await context.services.auditLogs.record({
      guildId,
      actorUserId: interaction.user.id,
      action: 'task.complete.permission_denied',
      targetType: 'agent_action',
      targetId: task.id
    });

    await interaction.reply({
      content: 'You do not have permission to complete that task.',
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  if (task.status === AGENT_ACTION_STATUSES.completed) {
    await interaction.reply({
      content: 'That task is already completed.',
      flags: MessageFlags.Ephemeral
    });
    return;
  }

  const completed = await context.services.agentActions.markCompleted(task, {
    metadata: {
      ...(task.metadata ?? {}),
      completedByUserId: interaction.user.id
    },
    resultSummary: result ?? `Completed by ${interaction.user.username}`
  });

  await context.services.auditLogs.record({
    guildId,
    actorUserId: interaction.user.id,
    action: 'task.completed',
    targetType: 'agent_action',
    targetId: completed.id,
    metadata: {
      resultSummary: completed.result_summary
    }
  });

  await interaction.reply({
    embeds: [buildTaskEmbed(completed, 'Task completed')],
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

function buildTaskEmbed(task: AgentActionRecord, title: string): EmbedBuilder {
  const embed = new EmbedBuilder()
    .setTitle(title)
    .setColor(task.status === AGENT_ACTION_STATUSES.completed ? 0x2ecc71 : 0x3498db)
    .addFields(
      { name: 'Task ID', value: task.id },
      { name: 'Title', value: task.title },
      { name: 'Assigned by', value: task.requester_username },
      { name: 'Assigned to', value: task.recipient_username ?? task.requester_username },
      { name: 'Status', value: task.status }
    )
    .setDescription(task.instructions);

  if (task.due_at) {
    embed.addFields({
      name: 'Due',
      value: `<t:${Math.floor(new Date(task.due_at).getTime() / 1000)}:F>`
    });
  }

  if (task.result_summary) {
    embed.addFields({
      name: 'Result',
      value: task.result_summary
    });
  }

  return embed;
}

function formatTaskSummary(task: AgentActionRecord): string {
  const dueLabel = task.due_at
    ? `<t:${Math.floor(new Date(task.due_at).getTime() / 1000)}:F>`
    : 'No due date';

  return [
    `**${task.title}**`,
    `ID: \`${task.id}\``,
    `Assigned by: ${task.requester_username}`,
    `Assigned to: ${task.recipient_username ?? task.requester_username}`,
    `Due: ${dueLabel}`,
    `Status: \`${task.status}\``
  ].join('\n');
}
