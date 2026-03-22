# Roadmap

## V1

Ship the reduced architecture:

- DM-first agent interaction
- slash-command assignment notices
- raw message history storage
- exact SQL/text retrieval
- semantic retrieval with embeddings
- metadata-only attachment support

Not in V1:

- digests
- durable memory
- OCR / vision
- browser tools
- sandbox execution

## V2

Add digest artifacts without turning them into the source of truth.

Planned additions:

- per-thread and per-channel/day summaries
- topic tags
- phrase stats
- lightweight activity summaries
- manual or selective digest generation for active windows

Rules:

- raw history remains canonical
- digests are derived and replaceable
- no automatic memory promotion yet

## V3

Add durable memory and richer tooling only after V1 and V2 prove useful.

Planned additions:

- durable memory facts
- inside-joke tracking and cooldowns
- optional OCR / image-aware retrieval
- optional browser worker
- optional code/test assistance worker
- richer orchestration for long-running tasks

Rules:

- durable memory must stay traceable to source history
- expensive enrichment must stay permission-gated
- no broad autonomous behavior without auditability
