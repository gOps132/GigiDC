import { loadEnv } from './config/env.js';
import { createDiscordClient } from './discord/client.js';
import { registerApplicationCommands } from './discord/registerCommands.js';
import { Logger } from './lib/logger.js';
import { createSupabaseAdminClient } from './lib/supabase.js';
import { AssignmentService } from './services/assignmentService.js';
import { AuditLogService } from './services/auditLogService.js';
import { ChannelIngestionPolicyService } from './services/channelIngestionPolicyService.js';
import { ClawbotClient } from './services/clawbotClient.js';
import { ClawbotDispatchService } from './services/clawbotDispatchService.js';
import { ClawbotJobService } from './services/clawbotJobService.js';
import { DiscordEventIngestionService } from './services/discordEventIngestionService.js';
import { RolePolicyService } from './services/rolePolicyService.js';
import { startWebhookServer } from './web/server.js';

async function main(): Promise<void> {
  const env = loadEnv();
  const logger = new Logger(env.LOG_LEVEL);
  const supabase = createSupabaseAdminClient(env);
  const clawbotClient = new ClawbotClient(env);
  const rolePolicies = new RolePolicyService(supabase);
  const assignments = new AssignmentService(supabase);
  const channelIngestionPolicies = new ChannelIngestionPolicyService(supabase);
  const clawbotJobs = new ClawbotJobService(supabase);
  const auditLogs = new AuditLogService(supabase);

  const context = {
    env,
    logger,
    services: {
      assignments,
      auditLogs,
      channelIngestionPolicies,
      clawbotDispatch: null as unknown as ClawbotDispatchService,
      clawbotJobs,
      rolePolicies
    }
  };

  const clawbotDispatch = new ClawbotDispatchService(context, clawbotClient);
  context.services.clawbotDispatch = clawbotDispatch;

  const ingestionService = new DiscordEventIngestionService(
    channelIngestionPolicies,
    rolePolicies,
    clawbotClient,
    logger
  );

  await registerApplicationCommands();

  const client = createDiscordClient(context, ingestionService);
  startWebhookServer(context, client, clawbotClient);
  await client.login(env.DISCORD_TOKEN);
}

main().catch((error) => {
  const message = error instanceof Error ? error.stack ?? error.message : String(error);
  console.error(message);
  process.exitCode = 1;
});
