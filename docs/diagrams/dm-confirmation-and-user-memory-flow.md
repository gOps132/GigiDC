---
title: DM Confirmation and User Memory Flow
description: Confirmation lifecycle and requester-centric memory in the DM path.
---

# DM Confirmation And User Memory Flow

This diagram captures the two new architectural seams added in this slice: persisted confirmation for cross-user relays, and bounded requester-centric memory snapshots for richer DM continuity.

```mermaid
flowchart LR
  Requester["DM Requester"]
  DmMessage["Discord DM"]
  Router["DmIntentRouter"]
  Planner["AgentToolService"]
  Confirm["ActionConfirmationService"]
  Actions["agent_actions"]
  Profiles["user_profiles"]
  Snapshots["user_memory_snapshots"]
  Reply["Gigi DM reply"]
  Recipient["Relay Recipient"]
  Retrieval["RetrievalService"]

  Requester --> DmMessage
  DmMessage --> Router
  Router -->|"relay request"| Planner
  Planner --> Confirm
  Confirm --> Actions
  Confirm -->|"confirm / cancel"| Reply
  Confirm --> Recipient
  Requester -->|"later question"| Retrieval
  Retrieval --> Actions
  Retrieval --> Profiles
  Retrieval --> Snapshots
  Retrieval --> Reply
```

## Reading Guide

- Cross-user relay requests do not execute immediately anymore. They become `agent_actions` rows that wait for explicit confirmation.
- `ActionConfirmationService` is the canonical place where confirm, cancel, expiry, permission re-checks, and final relay delivery happen.
- `user_profiles` stores stable requester identity facts for the primary guild, while `user_memory_snapshots` stores bounded derived summaries such as `identity_summary`, `working_context`, and `preferences`.
- Retrieval can now answer self-oriented DM questions from raw history plus participant-visible actions plus bounded user-memory snapshots.
- The snapshot layer is intentionally derived and expiring. It improves continuity, but it is not a substitute for source messages or action records.
