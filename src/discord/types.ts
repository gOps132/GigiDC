import type {
  ChatInputCommandInteraction,
  StringSelectMenuInteraction,
  SlashCommandBuilder,
  SlashCommandSubcommandsOnlyBuilder
} from 'discord.js';

import type { Env } from '../config/env.js';
import type { Logger } from '../lib/logger.js';
import type { AssignmentService } from '../services/assignmentService.js';
import type { AuditLogService } from '../services/auditLogService.js';
import type { DmConversationService } from '../services/dmConversationService.js';
import type { MessageHistoryService } from '../services/messageHistoryService.js';
import type { RetrievalService } from '../services/retrievalService.js';
import type { RolePolicyService } from '../services/rolePolicyService.js';

export interface BotContext {
  env: Env;
  logger: Logger;
  services: {
    assignments: AssignmentService;
    auditLogs: AuditLogService;
    dmConversation: DmConversationService;
    messageHistory: MessageHistoryService;
    retrieval: RetrievalService;
    rolePolicies: RolePolicyService;
  };
}

export interface SlashCommand {
  data: SlashCommandBuilder | SlashCommandSubcommandsOnlyBuilder;
  execute(interaction: ChatInputCommandInteraction, context: BotContext): Promise<void>;
}

export interface SelectMenuHandler {
  matches(interaction: StringSelectMenuInteraction): boolean;
  execute(interaction: StringSelectMenuInteraction, context: BotContext): Promise<void>;
}
