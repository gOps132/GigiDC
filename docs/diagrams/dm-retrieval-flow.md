# DM Retrieval Flow

This diagram captures the current DM runtime path after the indexing, persistence, and shared-identity upgrades: DM ingestion, durable scope selection, participant-visible action lookup, retrieval, and background embedding generation.

```mermaid
flowchart TD
  User["Discord User"]
  Discord["Discord Gateway"]
  Client["src/discord/client.ts"]
  History["MessageHistoryService"]
  Queue["MessageIndexingService"]
  Scope["DmConversationService"]
  Retrieval["RetrievalService"]
  Actions["agent_actions"]
  Responses["OpenAI Responses API"]
  Embeddings["OpenAI Embeddings API"]
  Pending["pending_dm_scope_selections"]
  Messages["messages"]
  Attachments["message_attachments"]
  Vectors["message_embeddings"]
  RPC["count_message_phrase / match_message_embeddings"]
  Reply["Discord DM Reply"]

  User --> Discord
  Discord --> Client

  Client -->|"every message"| History
  History --> Messages
  History --> Attachments
  History -->|"enqueue content"| Queue
  Queue --> Embeddings
  Embeddings --> Vectors

  Client -->|"DM questions"| Scope
  Scope -->|"persist active select menu"| Pending
  Scope -->|"bot-authored DM replies"| History
  Scope -->|"This DM or guild-wide"| Retrieval

  Retrieval -->|"exact phrase count"| RPC
  Retrieval -->|"recent context"| Messages
  Retrieval -->|"participant-visible shared actions"| Actions
  Retrieval -->|"query embedding"| Embeddings
  Embeddings --> RPC
  RPC --> Retrieval
  Messages --> Retrieval

  Retrieval --> Responses
  Responses --> Reply
```

## Reading Guide

- DM messages are stored first, and embeddings are generated asynchronously through `MessageIndexingService` instead of inline on the gateway path.
- `DmConversationService` persists active select-menu state in `pending_dm_scope_selections`, so restarts no longer invalidate in-flight scope picks.
- DM handling and retrieval are separate concerns: `DmConversationService` decides scope, persists its own bot-authored replies immediately, and `RetrievalService` decides answer strategy and can merge participant-visible action memory with raw message history.
- Retrieval mixes exact and semantic paths. Phrase-count questions use RPC/database logic, while broader questions assemble recent and semantically matched context before asking OpenAI for the final response.
- Shared Gigi actions in `agent_actions` are a separate control-plane memory seam. They let requesters and recipients ask follow-up questions about relays even when no guild-wide raw history should be exposed.
- Guild-wide history is only available after `RolePolicyService` approves the requester capability.
