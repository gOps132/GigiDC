# Service And Adapter Boundaries

This diagram captures the current architecture seam that matters most for future upgrades: Discord-facing services depend on ports, and vendor-specific adapters sit behind those ports.

```mermaid
flowchart LR
  Discord["Discord Runtime"]
  Services["Application Services<br/>DmConversationService<br/>MessageHistoryService<br/>MessageIndexingService<br/>RetrievalService<br/>AssignmentService"]
  Ports["Ports<br/>PendingDmScopeSelectionStore<br/>MessageHistoryRepository<br/>AssignmentStore<br/>AuditLogStore<br/>RolePolicyStore<br/>ChannelIngestionPolicyStore<br/>EmbeddingClient<br/>ResponseClient"]
  SupabaseAdapters["Supabase Adapters<br/>supabaseHistory.ts<br/>supabaseControlPlane.ts"]
  OpenAIAdapters["OpenAI Adapters<br/>openaiClients.ts"]
  DB["Supabase / Postgres"]
  OpenAI["OpenAI APIs"]

  Discord --> Services
  Services --> Ports
  Ports --> SupabaseAdapters
  Ports --> OpenAIAdapters
  SupabaseAdapters --> DB
  OpenAIAdapters --> OpenAI
```

## Reading Guide

- `src/discord/client.ts` remains the runtime shell, but most behavior now lives behind service contracts.
- The service layer no longer imports Supabase or OpenAI SDK clients directly for core behavior.
- `pending_dm_scope_selections` moved DM menu state out of process memory and behind a store port.
- This is not full clean architecture yet, but it is the right seam for future background workers, tests, and provider changes.
