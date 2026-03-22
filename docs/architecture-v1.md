# V1 Architecture

## Summary

V1 is a reduced architecture built for:

- DM-first agentic interaction
- slash-command assignment notices
- exact + semantic retrieval over raw Discord history
- role-gated guild-wide history access

V1 intentionally excludes:

- smart digestion
- memory promotion
- OCR / vision
- browser workers
- code sandbox workers

## Runtime Shape

```text
Discord
  -> Bot Service (discord.js)
     -> Supabase / Postgres
     -> OpenAI
```

Detailed layout:

```text
DMs / Slash Commands
   -> Bot Service
      -> Permission checks
      -> Assignment notifier handlers
      -> DM conversation router
      -> Retrieval service
         -> SQL/text search
         -> embeddings search
      -> Supabase/Postgres
      -> OpenAI
```

## Interaction Modes

### DMs

DMs are the primary agent interface.

Use them for:

- flexible questions
- semantic history lookup
- exact analytics like phrase counts
- scoped follow-up questions

If multiple scopes are possible and allowed, the bot asks the user to choose via a Discord select menu.

### Slash Commands

Slash commands are for structured workflows:

- `/ping`
- `/assignment create`
- `/assignment publish`
- `/assignment list`

## Data Split

### Control-plane tables

- guilds
- role_policies
- assignments
- audit_logs

### Retrieval tables

- messages
- message_embeddings
- message_attachments

## Scope Rules

V1 supports:

- `This DM`
- `Guild-wide` in the configured primary guild

Guild-wide history access is role-gated by `history_guild_wide`.

## Assignment Notifier

Assignment support in V1 is a notice workflow, not a full assignment management system.

Each notice stores:

- title
- description
- due date
- target channel
- affected role IDs
- creator
- publication status

Publishing posts a formatted notice to the target channel and mentions the affected roles.
