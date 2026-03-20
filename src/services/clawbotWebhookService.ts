import type { Client } from 'discord.js';
import { z } from 'zod';

import type { BotContext } from '../discord/types.js';
import type { ClawbotResultPostingService } from './clawbotResultPostingService.js';

const clawbotWebhookSchema = z.object({
  localJobId: z.string().min(1),
  clawbotJobId: z.string().min(1).nullable().optional(),
  status: z.enum(['completed', 'failed', 'cancelled']),
  resultSummary: z.string().nullable().optional(),
  artifactLinks: z.array(z.string()).default([]),
  errorMessage: z.string().nullable().optional()
});

export type ClawbotWebhookPayload = z.infer<typeof clawbotWebhookSchema>;

export class ClawbotWebhookService {
  constructor(
    private readonly context: BotContext,
    private readonly _client: Client,
    private readonly resultPoster: ClawbotResultPostingService
  ) {}

  parsePayload(value: unknown): ClawbotWebhookPayload {
    return clawbotWebhookSchema.parse(value);
  }

  async handleCompletion(payload: ClawbotWebhookPayload): Promise<void> {
    const job = await this.context.services.clawbotJobs.findByLocalId(payload.localJobId);
    if (!job) {
      throw new Error(`Unknown local job ID: ${payload.localJobId}`);
    }

    if (job.result_posted_message_id) {
      return;
    }

    const completed = await this.context.services.clawbotJobs.markComplete({
      localJobId: payload.localJobId,
      clawbotJobId: payload.clawbotJobId ?? null,
      status: payload.status,
      resultSummary: payload.resultSummary ?? null,
      artifactLinks: payload.artifactLinks,
      errorMessage: payload.errorMessage ?? null
    });

    const postedMessageId = await this.resultPoster.postResult(completed);
    if (postedMessageId) {
      await this.context.services.clawbotJobs.markResultPosted(completed.id, postedMessageId);
    }

    await this.context.services.auditLogs.record({
      guildId: completed.guild_id,
      actorUserId: null,
      action: 'clawbot_job_callback_processed',
      targetType: 'clawbot_job',
      targetId: completed.id,
      metadata: {
        status: completed.status,
        clawbotJobId: completed.clawbot_job_id,
        postedMessageId
      }
    });
  }
}
