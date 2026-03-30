---
title: Setup
description: Local development setup for GigiDC.
---

# Setup Guide

## Local Development

### Prerequisites

- Node.js 22.12.0 or newer
- npm
- Docker Desktop or another Docker-compatible runtime for `supabase start`
- Supabase CLI
- A Discord application and bot
- A Supabase project
- An OpenAI API key

### Environment Setup

1. Copy `.env.example` to `.env`
2. Put your real secrets and IDs into `.env`
3. Keep `.env` uncommitted

Required variables:

- `PORT`
- `DISCORD_TOKEN`
- `DISCORD_CLIENT_ID`
- `DISCORD_GUILD_ID`
- `PRIMARY_GUILD_ID`
- `REGISTER_COMMANDS_ON_STARTUP`
- `SUPABASE_URL`
- `SUPABASE_SERVICE_ROLE_KEY`
- `OPENAI_API_KEY`
- `SENSITIVE_DATA_ENCRYPTION_KEY` if you want encrypted sensitive-data retrieval

## Discord Setup

1. Create a Discord application in the Developer Portal
2. Add a bot user
3. Enable the permissions your bot needs in your invite URL
4. Enable the `MESSAGE CONTENT INTENT` in the Discord Developer Portal
5. Invite the bot to your test server
6. Use your server ID as `DISCORD_GUILD_ID` during development so slash command updates appear quickly
7. Set `PRIMARY_GUILD_ID` if you want DM guild-wide retrieval checks in V1

## Supabase Setup

1. Create the Supabase project
2. Authenticate the CLI with `supabase login`
3. Link this repo to your remote project with `supabase link --project-ref <your-project-ref>`
4. Apply the baseline migrations with `supabase db push`
5. For local development, start the local stack with `npm run supabase:start`
6. Reset the local database from the checked-in migrations with `npm run supabase:db:reset`
7. Create `role_policies` rows when you are ready to delegate assignment and guild-wide history access beyond Discord administrators
8. If you want guild-channel history storage, add rows to `channel_ingestion_policies` for the channels that should be ingested

Current checked-in baseline migrations:

- [supabase/migrations/001_initial_schema.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/001_initial_schema.sql)
- [supabase/migrations/002_clawbot_control_plane.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/002_clawbot_control_plane.sql)
- [supabase/migrations/003_v1_retrieval.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/003_v1_retrieval.sql)
- [supabase/migrations/004_cleanup_legacy_clawbot_tables.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/004_cleanup_legacy_clawbot_tables.sql)
- [supabase/migrations/005_pending_dm_scope_selections.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/005_pending_dm_scope_selections.sql)
- [supabase/migrations/20260329160104_add_agent_actions_shared_identity.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/20260329160104_add_agent_actions_shared_identity.sql)
- [supabase/migrations/20260329162152_expand_agent_actions_task_model.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/20260329162152_expand_agent_actions_task_model.sql)
- [supabase/migrations/20260330013000_solidify_dm_confirmations_and_user_memory.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/20260330013000_solidify_dm_confirmations_and_user_memory.sql)
- [supabase/migrations/20260330023500_add_direct_permissions_and_sensitive_data.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/20260330023500_add_direct_permissions_and_sensitive_data.sql)

Use the CLI for all new migrations:

```bash
npm run supabase:migration:new -- add_feature_name
```

Do not rename or renumber the existing baseline migrations just to match the timestamp format. They are the current project history. New migrations can use the CLI-generated timestamp naming from here forward.

`004_cleanup_legacy_clawbot_tables.sql` is intentionally a no-op placeholder. Keep it in the history, but do not reintroduce destructive cleanup there.

Example role policy shape:

```sql
insert into role_policies (guild_id, capability, discord_role_id)
values
  ('your-discord-guild-id', 'agent_action_dispatch', 'your-shared-action-role-id'),
  ('your-discord-guild-id', 'agent_action_receive', 'your-gigi-dm-recipient-role-id'),
  ('your-discord-guild-id', 'assignment_admin', 'your-assignment-admin-role-id'),
  ('your-discord-guild-id', 'ingestion_admin', 'your-ingestion-admin-role-id'),
  ('your-discord-guild-id', 'history_guild_wide', 'your-history-enabled-role-id'),
  ('your-discord-guild-id', 'permission_admin', 'your-permission-admin-role-id'),
  ('your-discord-guild-id', 'usage_admin', 'your-usage-admin-role-id');
```

Direct one-off user grants are now stored in `user_capability_grants` and can be managed from Discord with `/permission`.

Example ingestion policy shape:

```sql
insert into channel_ingestion_policies (guild_id, channel_id, enabled, updated_by_user_id)
values
  ('your-discord-guild-id', 'your-channel-id', true, 'your-discord-user-id');
```

## OpenAI Setup

1. Set `OPENAI_API_KEY`
2. Optionally set `OPENAI_RESPONSE_MODEL` as the shared default fallback model
3. Optionally set `OPENAI_RETRIEVAL_MODEL` if retrieval answers should use a different model than the shared default
4. Optionally set `OPENAI_TOOL_PLANNING_MODEL` if DM tool planning should use a different model than the shared default
5. Optionally set `OPENAI_EMBEDDING_MODEL`
6. Set `SENSITIVE_DATA_ENCRYPTION_KEY` to a 32-byte base64 or hex key if you want encrypted sensitive-data retrieval
7. For production on EC2, follow [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md)

## Sensitive Data Administration

Sensitive values are intentionally not written through normal DM chat because DM history is otherwise stored for retrieval.

Use the local admin script instead:

```bash
printf '%s' 'your-secret-value' | npm run sensitive:data -- put --guild YOUR_GUILD_ID --owner YOUR_USER_ID --label github --description "GitHub personal access token"
npm run sensitive:data -- list --guild YOUR_GUILD_ID --owner YOUR_USER_ID
npm run sensitive:data -- delete --guild YOUR_GUILD_ID --owner YOUR_USER_ID --label github
```

The script encrypts the value with `SENSITIVE_DATA_ENCRYPTION_KEY` before it writes to Supabase. Gigi only discloses those values in DM to the owning user.

## Local Verification

Run:

```bash
npm install
npm run typecheck
npm run build
npm run supabase:start
npm run supabase:db:reset
npm run dev
```

Then validate:

- `/ping` responds
- `/permission list`, `/permission grant`, and `/permission revoke` work for a user with `permission_admin`
- `/usage summary` and `/usage user` work for a user with `usage_admin`
- `/ingestion status` shows whether the current channel is enabled
- `/ingestion enable` turns ingestion on for the current or selected channel
- `/ingestion disable` turns ingestion off again
- `/relay dm` creates a participant-visible shared action through Gigi in `awaiting_confirmation`
- confirming the relay sends the DM and updates the same `agent_actions` row through its full lifecycle
- `/relay dm` only succeeds when the sender has `agent_action_dispatch` and the recipient has `agent_action_receive`
- `/task create` creates a follow-up task in `agent_actions`
- `/task list` shows open tasks visible to the requester
- `/task complete` closes a task and records the result summary
- `/assignment create` creates a draft notice record
- `/assignment list` returns recent assignments
- `/assignment publish` posts to the selected channel or current channel and mentions affected roles
- DM the bot with a general question
- DM the bot with `what tools can you call?` and confirm the answer stays limited to the actual bot runtime
- DM the bot with `can you give me a code execution environment?` and confirm it refuses unsupported tools cleanly
- DM the bot with a history question like `How many times did I say "ship it"?`
- DM the bot with a relay request like `send Mina a DM saying the release moved to Friday`, confirm the prompt, and verify the DM is only sent after confirmation
- DM the bot with `confirm!` or `cancel` when exactly one relay is pending and confirm the pending action resolves cleanly
- Seed a sensitive record with the local admin script, then DM the bot with `show my sensitive data` and `what is my github token`
- DM the bot with `remember my github token is ...` and confirm it refuses and skips ordinary history storage
- After `/relay dm`, ask the bot in DM what the requester wanted and confirm the answer can come from `agent_actions`
- Ask the bot in DM what tasks are still open and confirm the answer can come from open task records
- Ask the bot something like `what am I working on lately?` and confirm the answer can draw from `user_profiles` and `user_memory_snapshots`
- Confirm DM messages are being written immediately
- Confirm bot-authored DM replies and successful relay deliveries are also written into `messages`
- Confirm embeddings are written shortly after message storage rather than inline on the message hot path
- Confirm `model_usage_events` rows are written for retrieval responses, DM tool planning, semantic-query embeddings, and background message indexing
- Query `model_usage_daily_summary` and `model_usage_requester_daily_summary` in Supabase to confirm estimated USD cost rollups are populated
- If you enabled `channel_ingestion_policies`, confirm only enabled guild channels are stored
- Confirm ingestion policy changes create `audit_logs` rows without storing secrets or private message content
- Confirm relay permission denials and success/failure outcomes create `audit_logs` rows
- Confirm permission grants and revocations create `audit_logs` rows without storing secret values
- Check both `/healthz` and `/readyz`

## Safety Notes

- Never expose `SUPABASE_SERVICE_ROLE_KEY` or `OPENAI_API_KEY`
- Never expose `SENSITIVE_DATA_ENCRYPTION_KEY`
- Do not paste real secrets into docs, issues, or pull requests
- Review logs and generated output for sensitive data before sharing
- Do not commit linked-project credentials or copy remote database passwords into scripts
