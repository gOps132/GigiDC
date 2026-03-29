# DM Tool Execution Flow

This diagram captures the new step-3 runtime path: an explicit DM request can be planned into multiple internal tool calls, executed against the shared task/action substrate, and answered back in the same DM turn.

```mermaid
flowchart LR
  Requester["DM User"]
  DmMessage["Discord DM"]
  Conversation["DmConversationService"]
  Planner["AgentToolService"]
  Model["ToolPlanningClient<br/>OpenAI structured output"]
  TaskAction["AgentActionService"]
  Audit["AuditLogService"]
  History["MessageHistoryService"]
  ActionTable["agent_actions"]
  AuditTable["audit_logs"]
  MessageTable["messages"]
  Reply["Deterministic DM reply"]

  Requester --> DmMessage
  DmMessage --> Conversation
  Conversation --> Planner
  Planner --> Model
  Planner --> TaskAction
  Planner --> Audit
  Planner --> History
  TaskAction --> ActionTable
  Audit --> AuditTable
  History --> MessageTable
  Planner --> Reply
```

## Reading Guide

- `DmConversationService` now checks explicit tool-style requests before it falls back to retrieval.
- `AgentToolService` can execute up to three internal tool calls in one DM turn.
- The planner is bounded: it only targets internal task and relay tools, not arbitrary browser, shell, or external-provider actions.
- Task and relay execution still writes through `agent_actions`, so follow-up retrieval can recall what Gigi actually did.
- Audit and canonical message history are updated in the same tool path, which keeps permission denials, relay outcomes, and DM-visible replies traceable.
- This is still synchronous in-process orchestration. It is useful for short actions, but it is not yet a durable background worker system.
