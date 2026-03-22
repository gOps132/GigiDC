# Discord Bot with Clawbot-Primary Personalization

> Historical plan only.
>
> The active implementation target is now documented in:
>
> - [docs/architecture-v1.md](/Users/giancedrick/dev/projects/gigi/docs/architecture-v1.md)
> - [docs/roadmap.md](/Users/giancedrick/dev/projects/gigi/docs/roadmap.md)

## Summary

This repo is the Discord control plane. Clawbot is the personalization and agent backend.

This bot owns:

- Discord slash commands and interaction UX
- Role restrictions and guild or channel policy
- Assignment workflows
- Audit logging and local job references
- Forwarding eligible Discord channel history to Clawbot

Clawbot owns:

- Raw channel history ingestion and personalization memory
- Async review, generation, and note-analysis work
- Result artifacts and long-running agent execution

Supabase remains the local store for Discord-specific control-plane data only.

## Architecture

### Local control plane

- `discord.js` bot runtime on Node 22 and TypeScript
- Supabase tables for:
  - `guilds`
  - `role_policies`
  - `assignments`
  - `channel_ingestion_policies`
  - `clawbot_jobs`
  - `audit_logs`
- Local assignment commands stay functional even if Clawbot is unavailable

### Clawbot integration

- Outgoing job dispatch over HTTP with `CLAWBOT_BASE_URL` and `CLAWBOT_API_KEY`
- Outgoing Discord message ingestion over HTTP for channels where ingestion is enabled
- Incoming webhook callback at `POST /webhooks/clawbot`
- Callback verification using `CLAWBOT_WEBHOOK_SECRET` or the same shared secret as `CLAWBOT_API_KEY`

### Command split

- Local:
  - `/ping`
  - `/assignment create`
  - `/assignment publish`
  - `/assignment list`
  - `/ingestion enable`
  - `/ingestion disable`
  - `/ingestion status`
- Clawbot-backed:
  - `/review pr`
  - `/generate tests`
  - `/generate quiz`
  - `/generate summary`
  - `/notes analyze`

## Interfaces and Data

### Outgoing Clawbot job dispatch

The bot sends:

- local job ID
- command name
- task type
- guild ID
- channel ID
- thread ID when applicable
- requester user ID
- input payload
- callback URL

### Outgoing Discord message ingestion

The bot sends:

- guild ID
- channel ID
- thread ID when applicable
- message ID
- author ID and username
- message content
- attachment metadata
- creation timestamp

### Incoming webhook callback

Clawbot must send:

- `localJobId`
- `clawbotJobId`
- `status` as `completed`, `failed`, or `cancelled`
- optional `resultSummary`
- optional `artifactLinks`
- optional `errorMessage`

The bot stores only job references, metadata, and safe summaries locally. Full raw Clawbot outputs are not persisted in Supabase by default.

## Testing and Acceptance

- Assignment commands work without Clawbot
- Role-restricted commands enforce `assignment_admin`, `ingestion_admin`, and `clawbot_dispatch`
- Enabling ingestion for a channel causes new messages there to be forwarded to Clawbot
- Disabled channels do not forward history
- Clawbot-backed commands create local job references and return immediate acknowledgement
- Valid webhook callbacks update local job status and post results back into Discord
- Invalid or duplicate callbacks do not create duplicate result posts
- Logs and stored metadata do not leak secrets or unnecessary raw private content

## Defaults and Assumptions

- Clawbot is already deployed and reachable from this bot
- This bot runs its webhook server on port `8080`
- `BOT_PUBLIC_BASE_URL` points to a public URL that forwards to this bot service
- Channel history ingestion is opt-in and managed per channel
- Discord `MESSAGE CONTENT INTENT` is enabled if ingestion is required
- External resources must be credited and sensitive data must be reviewed before sharing or deployment
