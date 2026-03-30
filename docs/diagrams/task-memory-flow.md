---
title: Task Memory Flow
description: Shared task/action memory path for follow-up work and retrieval.
---

# Task Memory Flow

This diagram captures the new step-2 shared-identity layer: Gigi can now track open follow-up work in `agent_actions`, expose it through slash commands, and use it during DM retrieval.

```mermaid
flowchart LR
  Requester["Guild User"]
  Command["/task create"]
  Policy["RolePolicyService<br/>agent_action_dispatch"]
  TaskCommand["task.ts"]
  Actions["AgentActionService"]
  ActionTable["agent_actions<br/>action_scope=task"]
  Audit["AuditLogService"]
  List["/task list"]
  Complete["/task complete"]
  DmQuestion["DM question<br/>what tasks do I still have?"]
  Retrieval["RetrievalService"]
  Response["Gigi DM answer"]

  Requester --> Command
  Command --> Policy
  Policy --> TaskCommand
  TaskCommand --> Actions
  Actions --> ActionTable
  TaskCommand --> Audit
  Requester --> List
  Requester --> Complete
  List --> Actions
  Complete --> Actions
  Requester --> DmQuestion
  DmQuestion --> Retrieval
  Retrieval --> ActionTable
  Retrieval --> Response
```

## Reading Guide

- `agent_actions` is now a mixed task/action substrate instead of a relay-only log.
- Tasks use the same durable visibility model as relays, so assignees and requesters can recall them later without opening guild-wide history.
- Retrieval can assemble task context separately from chat history, which is the first move away from treating all memory as raw messages.
- This is still not a full orchestration system. It is a durable work-tracking seam that future tool execution can attach to.
