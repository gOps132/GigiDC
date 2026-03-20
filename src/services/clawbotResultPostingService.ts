import {
  EmbedBuilder,
  type Client,
  type MessageCreateOptions
} from 'discord.js';

import type { Logger } from '../lib/logger.js';
import type { ClawbotJobRecord } from './clawbotJobService.js';

export class ClawbotResultPostingService {
  constructor(
    private readonly client: Client,
    private readonly logger: Logger
  ) {}

  async postResult(job: ClawbotJobRecord): Promise<string | null> {
    const targetId = job.thread_id ?? job.channel_id;
    const channel = await this.client.channels.fetch(targetId);

    if (!channel || !channel.isTextBased() || !('send' in channel)) {
      this.logger.warn('Unable to resolve Discord target for Clawbot result', {
        localJobId: job.id,
        targetId
      });
      return null;
    }

    const message = await channel.send(buildResultMessage(job));
    return message.id;
  }
}

function buildResultMessage(job: ClawbotJobRecord): MessageCreateOptions {
  const color = job.status === 'completed' ? 0x2ecc71 : 0xe74c3c;
  const title = job.status === 'completed' ? 'Clawbot job completed' : 'Clawbot job failed';
  const embed = new EmbedBuilder()
    .setTitle(title)
    .setColor(color)
    .addFields(
      { name: 'Command', value: job.command_name, inline: true },
      { name: 'Task', value: job.task_type, inline: true },
      { name: 'Local job ID', value: job.id },
      { name: 'Clawbot job ID', value: job.clawbot_job_id ?? 'Not provided' }
    )
    .setTimestamp(new Date(job.updated_at));

  if (job.result_summary) {
    embed.setDescription(job.result_summary);
  }

  if (job.error_message) {
    embed.addFields({
      name: 'Error',
      value: limitFieldValue(job.error_message)
    });
  }

  if (job.artifact_links.length > 0) {
    embed.addFields({
      name: 'Artifacts',
      value: limitFieldValue(job.artifact_links.join('\n'))
    });
  }

  return {
    content: `<@${job.requester_user_id}>`,
    embeds: [embed]
  };
}

function limitFieldValue(value: string): string {
  return value.length > 1024 ? `${value.slice(0, 1021)}...` : value;
}
