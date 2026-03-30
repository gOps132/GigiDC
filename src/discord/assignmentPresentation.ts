import { EmbedBuilder } from 'discord.js';

import type { AssignmentRecord } from '../services/assignmentService.js';

export function buildAssignmentEmbed(
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

export function buildAssignmentAnnouncementIntro(
  assignment: AssignmentRecord,
  requesterLabel: string
): string {
  const roleMentions = assignment.mentioned_role_ids.map((roleId) => `<@&${roleId}>`).join(' ');

  return roleMentions.length > 0
    ? `${roleMentions}\nNew assignment notice from ${requesterLabel}`
    : `New assignment notice from ${requesterLabel}`;
}
