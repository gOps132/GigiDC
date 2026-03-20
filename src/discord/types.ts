import type {
  ChatInputCommandInteraction,
  SlashCommandBuilder,
  SlashCommandSubcommandsOnlyBuilder
} from 'discord.js';

import type { Env } from '../config/env.js';
import type { Logger } from '../lib/logger.js';
import type { AssignmentService } from '../services/assignmentService.js';
import type { AuditLogService } from '../services/auditLogService.js';
import type { ChannelIngestionPolicyService } from '../services/channelIngestionPolicyService.js';
import type { ClawbotDispatchService } from '../services/clawbotDispatchService.js';
import type { ClawbotJobService } from '../services/clawbotJobService.js';
import type { RolePolicyService } from '../services/rolePolicyService.js';

export interface BotContext {
  env: Env;
  logger: Logger;
  services: {
    assignments: AssignmentService;
    auditLogs: AuditLogService;
    channelIngestionPolicies: ChannelIngestionPolicyService;
    clawbotDispatch: ClawbotDispatchService;
    clawbotJobs: ClawbotJobService;
    rolePolicies: RolePolicyService;
  };
}

export interface SlashCommand {
  data: SlashCommandBuilder | SlashCommandSubcommandsOnlyBuilder;
  execute(interaction: ChatInputCommandInteraction, context: BotContext): Promise<void>;
}
