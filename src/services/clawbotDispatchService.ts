import type { BotContext } from '../discord/types.js';
import type { ClawbotClient } from './clawbotClient.js';

export interface DispatchClawbotJobInput {
  guildId: string;
  channelId: string;
  threadId: string | null;
  requesterUserId: string;
  commandName: string;
  taskType: string;
  input: Record<string, unknown>;
}

export class ClawbotDispatchService {
  constructor(
    private readonly context: BotContext,
    private readonly clawbotClient: ClawbotClient
  ) {}

  async dispatch(input: DispatchClawbotJobInput) {
    const localJob = await this.context.services.clawbotJobs.createJob({
      guildId: input.guildId,
      channelId: input.channelId,
      threadId: input.threadId,
      requesterUserId: input.requesterUserId,
      commandName: input.commandName,
      taskType: input.taskType,
      requestPayload: input.input
    });

    try {
      const response = await this.clawbotClient.dispatchJob({
        localJobId: localJob.id,
        commandName: input.commandName,
        taskType: input.taskType,
        guildId: input.guildId,
        channelId: input.channelId,
        threadId: input.threadId,
        requesterUserId: input.requesterUserId,
        input: input.input,
        callbackUrl: `${this.context.env.BOT_PUBLIC_BASE_URL}/webhooks/clawbot`
      });

      const submitted = await this.context.services.clawbotJobs.markSubmitted({
        localJobId: localJob.id,
        clawbotJobId: response.jobId,
        status: response.status
      });

      await this.context.services.auditLogs.record({
        guildId: input.guildId,
        actorUserId: input.requesterUserId,
        action: 'clawbot_job_submitted',
        targetType: 'clawbot_job',
        targetId: submitted.id,
        metadata: {
          commandName: input.commandName,
          taskType: input.taskType,
          clawbotJobId: response.jobId
        }
      });

      return submitted;
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Unknown dispatch error';

      await this.context.services.clawbotJobs.markComplete({
        localJobId: localJob.id,
        clawbotJobId: null,
        status: 'failed',
        resultSummary: null,
        artifactLinks: [],
        errorMessage: message
      });

      await this.context.services.auditLogs.record({
        guildId: input.guildId,
        actorUserId: input.requesterUserId,
        action: 'clawbot_job_failed_before_submission',
        targetType: 'clawbot_job',
        targetId: localJob.id,
        metadata: {
          commandName: input.commandName,
          taskType: input.taskType,
          error: message
        }
      });

      throw error;
    }
  }
}
