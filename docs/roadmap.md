---
title: Roadmap
description: Planned evolution of GigiDC beyond the current V1 architecture.
---

# Roadmap

## V1

Ship the reduced architecture:

- DM-first agent interaction
- slash-command assignment notices
- participant-visible shared action memory for Gigi-mediated relays
- bounded internal DM tool execution for tasks and relays
- raw message history storage
- exact SQL/text retrieval
- semantic retrieval with embeddings
- metadata-only attachment support

Not in V1:

- digests
- durable memory facts beyond explicit Gigi actions
- OCR / vision
- browser tools
- sandbox execution
- durable worker-backed tool orchestration

## V2

Add digest artifacts without turning them into the source of truth.

Planned additions:

- per-thread and per-channel/day summaries
- richer shared-action and task tracking beyond simple follow-up tasks
- richer DM tool execution than the current bounded task/relay set
- topic tags
- phrase stats
- lightweight activity summaries
- manual or selective digest generation for active windows

Rules:

- raw history remains canonical
- explicit Gigi actions can be durable before broader memory promotion
- digests are derived and replaceable
- no automatic memory promotion yet

Implications driving V2:

- raw history alone will eventually produce context rot as DM, relay, and bot-authored message volume grows
- retrieval quality will need better ranking, narrower context selection, and derived summaries instead of larger windows
- storage and embedding cost will keep rising unless the project introduces retention, summarization, or selective embedding rules

## V3

Add durable memory and richer tooling only after V1 and V2 prove useful.

Planned additions:

- durable memory facts
- traceable task memory that can span channels, DMs, and bot-authored actions
- inside-joke tracking and cooldowns
- optional OCR / image-aware retrieval
- optional browser worker
- optional code/test assistance worker
- richer orchestration for long-running tasks and tool execution beyond the current synchronous DM planner

Rules:

- durable memory must stay traceable to source history
- expensive enrichment must stay permission-gated
- no broad autonomous behavior without auditability
- new tool surfaces should attach to durable task/action records instead of living only in transient chat turns

Implications driving V3:

- a larger shared-memory surface without explicit task/fact modeling would increase leakage risk and retrieval ambiguity
- richer multi-tool orchestration should keep sitting on top of durable task/action records, not raw chat history alone
- synchronous DM tool execution is useful now, but it will eventually need durable job handling, retries, and better user resolution
