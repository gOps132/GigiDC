import { loadEnv } from './config/env.js';
import {
  OpenAIEmbeddingClient,
  OpenAIResponseClient,
  OpenAIToolPlanningClient
} from './adapters/openaiClients.js';
import {
  SupabaseAgentActionStore,
  SupabaseAssignmentStore,
  SupabaseAuditLogStore,
  SupabaseChannelIngestionPolicyStore,
  SupabaseRolePolicyStore
} from './adapters/supabaseControlPlane.js';
import {
  SupabaseMessageHistoryRepository,
  SupabasePendingDmScopeSelectionStore
} from './adapters/supabaseHistory.js';
import { createDiscordClient } from './discord/client.js';
import { registerApplicationCommands } from './discord/registerCommands.js';
import { Logger } from './lib/logger.js';
import { createOpenAIClient } from './lib/openai.js';
import { createSupabaseAdminClient } from './lib/supabase.js';
import { AgentActionService } from './services/agentActionService.js';
import { AgentToolService } from './services/agentToolService.js';
import { AssignmentService } from './services/assignmentService.js';
import { AuditLogService } from './services/auditLogService.js';
import { ChannelIngestionPolicyService } from './services/channelIngestionPolicyService.js';
import { DmConversationService } from './services/dmConversationService.js';
import { MessageHistoryService } from './services/messageHistoryService.js';
import { MessageIndexingService } from './services/messageIndexingService.js';
import { RetrievalService } from './services/retrievalService.js';
import { RolePolicyService } from './services/rolePolicyService.js';
import { RuntimeStateService } from './services/runtimeStateService.js';
import { startWebhookServer } from './web/server.js';

async function main(): Promise<void> {
  const env = loadEnv();
  const logger = new Logger(env.LOG_LEVEL);
  const supabase = createSupabaseAdminClient(env);
  const openai = createOpenAIClient(env);
  const runtime = new RuntimeStateService();
  const embeddings = new OpenAIEmbeddingClient(openai);
  const responses = new OpenAIResponseClient(openai);
  const toolPlanner = new OpenAIToolPlanningClient(openai);
  const historyRepository = new SupabaseMessageHistoryRepository(supabase);
  const pendingDmScopeSelections = new SupabasePendingDmScopeSelectionStore(supabase);
  const rolePolicyStore = new SupabaseRolePolicyStore(supabase);
  const channelIngestionPolicyStore = new SupabaseChannelIngestionPolicyStore(supabase);
  const assignmentStore = new SupabaseAssignmentStore(supabase);
  const auditLogStore = new SupabaseAuditLogStore(supabase);
  const agentActionStore = new SupabaseAgentActionStore(supabase);
  const rolePolicies = new RolePolicyService(rolePolicyStore);
  const agentActions = new AgentActionService(agentActionStore);
  const channelIngestionPolicies = new ChannelIngestionPolicyService(channelIngestionPolicyStore);
  const assignments = new AssignmentService(assignmentStore);
  const auditLogs = new AuditLogService(auditLogStore);
  const messageIndexing = new MessageIndexingService(env, embeddings, historyRepository, logger);
  const messageHistory = new MessageHistoryService(
    env,
    historyRepository,
    embeddings,
    channelIngestionPolicies,
    messageIndexing,
    rolePolicies,
    logger
  );
  const retrieval = new RetrievalService(env, responses, messageHistory, agentActions);
  const agentTools = new AgentToolService(
    env,
    toolPlanner,
    agentActions,
    auditLogs,
    messageHistory,
    rolePolicies,
    logger
  );

  const context = {
    env,
    logger,
    runtime,
    services: {
      agentActions,
      agentTools,
      assignments,
      auditLogs,
      channelIngestionPolicies,
      dmConversation: null as unknown as DmConversationService,
      messageHistory,
      messageIndexing,
      retrieval,
      rolePolicies
    }
  };

  const dmConversation = new DmConversationService(context, pendingDmScopeSelections);
  context.services.dmConversation = dmConversation;

  if (env.REGISTER_COMMANDS_ON_STARTUP) {
    void registerApplicationCommands()
      .then(() => {
        runtime.markCommandRegistrationReady();
        logger.info('Registered Discord application commands at startup');
      })
      .catch((error) => {
        const message = error instanceof Error ? error.message : 'Unknown command registration error';
        runtime.markCommandRegistrationFailed(message);
        logger.error('Failed to register Discord application commands at startup', {
          error: message
        });
      });
  } else {
    runtime.markCommandRegistrationSkipped();
    logger.info('Skipped Discord application command registration at startup');
  }

  const client = createDiscordClient(context);
  startWebhookServer(context);
  await client.login(env.DISCORD_TOKEN);
}

main().catch((error) => {
  const message = error instanceof Error ? error.stack ?? error.message : String(error);
  console.error(message);
  process.exitCode = 1;
});
