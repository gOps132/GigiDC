import { loadEnv } from './config/env.js';
import {
  OpenAIEmbeddingClient,
  OpenAIResponseClient,
  OpenAIToolPlanningClient
} from './adapters/openaiClients.js';
import {
  SupabaseUserMemorySnapshotStore,
  SupabaseUserProfileStore
} from './adapters/supabaseIdentity.js';
import { SupabaseSensitiveDataStore } from './adapters/supabaseSensitiveData.js';
import {
  SupabaseAgentActionStore,
  SupabaseAssignmentStore,
  SupabaseAuditLogStore,
  SupabaseChannelIngestionPolicyStore,
  SupabaseRolePolicyStore
} from './adapters/supabaseControlPlane.js';
import {
  SupabaseMessageHistoryRepository,
  SupabasePendingDmRelayRecipientSelectionStore,
  SupabasePendingDmScopeSelectionStore
} from './adapters/supabaseHistory.js';
import { createDiscordClient } from './discord/client.js';
import { registerApplicationCommands } from './discord/registerCommands.js';
import { Logger } from './lib/logger.js';
import { createOpenAIClient } from './lib/openai.js';
import { createSupabaseAdminClient } from './lib/supabase.js';
import { ActionConfirmationService } from './services/actionConfirmationService.js';
import { AgentActionService } from './services/agentActionService.js';
import { AgentToolService } from './services/agentToolService.js';
import { AssignmentService } from './services/assignmentService.js';
import { AuditLogService } from './services/auditLogService.js';
import { ChannelIngestionPolicyService } from './services/channelIngestionPolicyService.js';
import { DmConversationService } from './services/dmConversationService.js';
import { GuildAdminActionService } from './services/guildAdminActionService.js';
import { MessageHistoryService } from './services/messageHistoryService.js';
import { MessageIndexingService } from './services/messageIndexingService.js';
import { PermissionAdminService } from './services/permissionAdminService.js';
import { RetrievalService } from './services/retrievalService.js';
import { RolePolicyService } from './services/rolePolicyService.js';
import { RuntimeStateService } from './services/runtimeStateService.js';
import { SensitiveDataService } from './services/sensitiveDataService.js';
import { UserMemoryService } from './services/userMemoryService.js';
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
  const pendingDmRecipientSelections = new SupabasePendingDmRelayRecipientSelectionStore(supabase);
  const pendingDmScopeSelections = new SupabasePendingDmScopeSelectionStore(supabase);
  const userProfileStore = new SupabaseUserProfileStore(supabase);
  const userMemorySnapshotStore = new SupabaseUserMemorySnapshotStore(supabase);
  const sensitiveDataStore = new SupabaseSensitiveDataStore(supabase);
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
  const sensitiveData = new SensitiveDataService(env, sensitiveDataStore, logger);
  const permissionAdmin = new PermissionAdminService(env, auditLogs, rolePolicies);
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
  const userMemory = new UserMemoryService(
    userProfileStore,
    userMemorySnapshotStore,
    messageHistory,
    agentActions,
    logger
  );
  const retrieval = new RetrievalService(env, responses, messageHistory, agentActions, userMemory, logger);
  const actionConfirmations = new ActionConfirmationService(
    env,
    agentActions,
    auditLogs,
    messageHistory,
    rolePolicies,
    logger
  );
  const guildAdminActions = new GuildAdminActionService(
    env,
    assignments,
    auditLogs,
    channelIngestionPolicies,
    rolePolicies,
    logger
  );
  const agentTools = new AgentToolService(
    env,
    toolPlanner,
    actionConfirmations,
    agentActions,
    auditLogs,
    guildAdminActions,
    permissionAdmin,
    rolePolicies,
    pendingDmRecipientSelections,
    logger
  );

  const context = {
    env,
    logger,
    runtime,
    services: {
      actionConfirmations,
      agentActions,
      agentTools,
      assignments,
      auditLogs,
      channelIngestionPolicies,
      dmConversation: null as unknown as DmConversationService,
      messageHistory,
      messageIndexing,
      permissionAdmin,
      retrieval,
      rolePolicies,
      sensitiveData,
      userMemory
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
