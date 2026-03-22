import { loadEnv } from './config/env.js';
import { createDiscordClient } from './discord/client.js';
import { registerApplicationCommands } from './discord/registerCommands.js';
import { Logger } from './lib/logger.js';
import { createOpenAIClient } from './lib/openai.js';
import { createSupabaseAdminClient } from './lib/supabase.js';
import { AssignmentService } from './services/assignmentService.js';
import { AuditLogService } from './services/auditLogService.js';
import { DmConversationService } from './services/dmConversationService.js';
import { MessageHistoryService } from './services/messageHistoryService.js';
import { RetrievalService } from './services/retrievalService.js';
import { RolePolicyService } from './services/rolePolicyService.js';
import { startWebhookServer } from './web/server.js';

async function main(): Promise<void> {
  const env = loadEnv();
  const logger = new Logger(env.LOG_LEVEL);
  const supabase = createSupabaseAdminClient(env);
  const openai = createOpenAIClient(env);
  const rolePolicies = new RolePolicyService(supabase);
  const assignments = new AssignmentService(supabase);
  const auditLogs = new AuditLogService(supabase);
  const messageHistory = new MessageHistoryService(env, supabase, openai, rolePolicies, logger);
  const retrieval = new RetrievalService(env, openai, messageHistory);

  const context = {
    env,
    logger,
    services: {
      assignments,
      auditLogs,
      dmConversation: null as unknown as DmConversationService,
      messageHistory,
      retrieval,
      rolePolicies
    }
  };

  const dmConversation = new DmConversationService(context);
  context.services.dmConversation = dmConversation;

  await registerApplicationCommands();

  const client = createDiscordClient(context);
  startWebhookServer(context);
  await client.login(env.DISCORD_TOKEN);
}

main().catch((error) => {
  const message = error instanceof Error ? error.stack ?? error.message : String(error);
  console.error(message);
  process.exitCode = 1;
});
