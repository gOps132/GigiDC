# Gigi Discord Bot

DM-first Discord bot for agentic chat, scoped history retrieval, and assignment notifications.

## What Is In This Repo

- A Node 22 + TypeScript Discord bot foundation
- DM-first agent experience backed by OpenAI
- Slash-command assignment notifier workflow
- Supabase/Postgres-backed control-plane state
- Raw message history + pgvector-ready retrieval foundation
- Project rules and planning docs

## Quick Start

### 1. Install dependencies

```bash
npm install
```

### 2. Create your local environment file

```bash
cp .env.example .env
```

Open `.env` and put your real API keys and IDs there:

- `DISCORD_TOKEN`
- `DISCORD_CLIENT_ID`
- `DISCORD_GUILD_ID`
- `PRIMARY_GUILD_ID`
- `SUPABASE_URL`
- `SUPABASE_SERVICE_ROLE_KEY`
- `OPENAI_API_KEY`

Do not commit `.env`.

### 3. Create the Discord application and bot

- Create an application in the Discord Developer Portal
- Create a bot user for that application
- Copy the bot token into `DISCORD_TOKEN`
- Copy the application client ID into `DISCORD_CLIENT_ID`
- Copy your server ID into `DISCORD_GUILD_ID` for guild-scoped slash command registration during development
- Invite the bot to your server with permissions to read, send, and use application commands

### 4. Configure Supabase and retrieval storage

- Create a Supabase project
- Copy the project URL into `SUPABASE_URL`
- Copy the server-side service role key into `SUPABASE_SERVICE_ROLE_KEY`
- Install the Supabase CLI
- Install Docker Desktop or another Docker-compatible runtime if you want the local Supabase stack
- This repo now includes `supabase/config.toml` and keeps database changes under `supabase/migrations/`
- Existing `001` to `005` files are the preserved baseline migrations for this project
- `004_cleanup_legacy_clawbot_tables.sql` is intentionally a no-op placeholder so the baseline history stays contiguous without dropping active tables
- For a linked remote project, apply them with `supabase db push`
- For local development, run `npm run supabase:start` and `npm run supabase:db:reset`
- Future schema changes should use `supabase migration new <name>` so new files follow the CLI-managed workflow

### 5. Configure OpenAI

- Set `OPENAI_API_KEY` to a valid OpenAI API key
- Optional: override `OPENAI_RESPONSE_MODEL` and `OPENAI_EMBEDDING_MODEL`
- The bot uses OpenAI for:
  - DM reasoning
  - semantic retrieval over stored chat history
  - no browser, OCR, or image understanding in V1

### 6. Start the bot

```bash
npm run dev
```

The bot can register slash commands at startup, starts a local health server on port `8080`, stores DM history immediately, stores guild history only for channels with ingestion explicitly enabled, and responds to direct messages agentically.

## EC2 Deployment

Deployment assets for the recommended two-EC2 layout are included in:

- [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md)
- [docs/ci-cd.md](/Users/giancedrick/dev/projects/gigi/docs/ci-cd.md)
- [docs/terraform.md](/Users/giancedrick/dev/projects/gigi/docs/terraform.md)
- [deploy/systemd/gigi-discord-bot.service](/Users/giancedrick/dev/projects/gigi/deploy/systemd/gigi-discord-bot.service)
- [deploy/nginx/gigi-discord-bot.conf](/Users/giancedrick/dev/projects/gigi/deploy/nginx/gigi-discord-bot.conf)
- [scripts/bootstrap-ec2.sh](/Users/giancedrick/dev/projects/gigi/scripts/bootstrap-ec2.sh)
- [scripts/deploy-discord-bot.sh](/Users/giancedrick/dev/projects/gigi/scripts/deploy-discord-bot.sh)
- [scripts/install-release.sh](/Users/giancedrick/dev/projects/gigi/scripts/install-release.sh)
- [terraform/terraform.tfvars.example](/Users/giancedrick/dev/projects/gigi/terraform/terraform.tfvars.example)

## Developer Foundations

The repo now includes a CI workflow at [ci.yml](/Users/giancedrick/dev/projects/gigi/.github/workflows/ci.yml).

On every push to `main` and on every pull request, CI runs:

- `npm ci`
- `npm run typecheck`
- `npm run build`
- `terraform fmt -check`
- `terraform init -backend=false`
- `terraform validate`

On pushes to `main`, CD also:

- builds a release bundle
- uploads the release to the bot EC2 over SSH
- installs the release on the server
- restarts `gigi-discord-bot`

CD setup instructions and required GitHub secrets are in [docs/ci-cd.md](/Users/giancedrick/dev/projects/gigi/docs/ci-cd.md).

## Available Commands and Interaction Modes

- `/ping`
- `/ingestion enable`
- `/ingestion disable`
- `/ingestion status`
- `/assignment create`
- `/assignment publish`
- `/assignment list`

DM the bot for:

- freeform questions
- semantic history search
- exact phrase count questions such as `How many times did I say "ship it"?`
- conversational follow-ups over your DM history

## Authorization Model

The bot checks Discord `Administrator` first, then capability mappings in `role_policies`.

Capabilities used by the current command set:

- `assignment_admin`
- `ingestion_admin`
- `history_guild_wide`

Example:

```sql
insert into role_policies (guild_id, capability, discord_role_id)
values
  ('your-discord-guild-id', 'assignment_admin', 'your-assignment-admin-role-id'),
  ('your-discord-guild-id', 'ingestion_admin', 'your-ingestion-admin-role-id'),
  ('your-discord-guild-id', 'history_guild_wide', 'your-history-enabled-role-id');
```

## Retrieval Model

V1 uses a reduced retrieval-first architecture:

- raw Discord messages are the source of truth
- DM history is always eligible for storage
- guild-channel ingestion is opt-in through `channel_ingestion_policies`
- ingestion policy changes and permission denials are written to `audit_logs`
- DM scope-selection prompts are persisted briefly in Supabase so restarts do not invalidate active menus
- exact analytics use SQL/text search first
- semantic questions use OpenAI embeddings over stored messages
- image attachments are stored as metadata only
- no OCR, memory promotion, or digest pipeline in V1

## Development Scripts

```bash
npm run dev
npm run build
npm run typecheck
npm run register:commands
npm run supabase:start
npm run supabase:stop
npm run supabase:status
npm run supabase:db:reset
npm run supabase:db:push
npm run supabase:migration:new -- add_feature_name
```

## Testing Guidance

Before opening a PR or deploying:

- Run `npm run typecheck`
- Run `npm run build`
- Run `terraform fmt -check` inside `terraform/` when infrastructure files change
- Run `terraform validate` inside `terraform/` after `terraform init -backend=false` when infrastructure files change
- Verify `/ping`
- Verify `/ingestion status`, `/ingestion enable`, and `/ingestion disable` in a development Discord server
- Verify `/assignment create` and `/assignment publish` in a development Discord server
- DM the bot with one direct question and one history-based question
- Confirm DM messages are stored in Supabase
- If you enable a row in `channel_ingestion_policies`, confirm guild messages for that channel are stored
- Check `GET /readyz` as well as `GET /healthz`
- Re-check all modified lines for secrets and private data leaks

## Project Structure

```text
src/
  commands/         Slash command handlers
  config/           Environment validation
  discord/          Client bootstrap and command registration
  lib/              Shared clients and logging
  services/         Control-plane, history, and DM retrieval services
  web/              Health server
deploy/
  nginx/            Reverse-proxy template for EC2
  systemd/          System service template for EC2
scripts/            EC2 bootstrap and deploy helpers
supabase/
  migrations/       SQL schema for the app
.codex/
  skills/           Repo-local agent workflow skills
docs/
  diagrams/         Durable architecture and flow diagrams
  discord-bot-plan.md
  agent-skills.md
  project-visualization-workflow.md
  setup.md
  credits.md
```

## Documentation

- [docs/discord-bot-plan.md](/Users/giancedrick/dev/projects/gigi/docs/discord-bot-plan.md)
- [docs/agent-skills.md](/Users/giancedrick/dev/projects/gigi/docs/agent-skills.md)
- [docs/project-visualization-workflow.md](/Users/giancedrick/dev/projects/gigi/docs/project-visualization-workflow.md)
- [docs/architecture-v1.md](/Users/giancedrick/dev/projects/gigi/docs/architecture-v1.md)
- [docs/roadmap.md](/Users/giancedrick/dev/projects/gigi/docs/roadmap.md)
- [docs/setup.md](/Users/giancedrick/dev/projects/gigi/docs/setup.md)
- [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md)
- [docs/ci-cd.md](/Users/giancedrick/dev/projects/gigi/docs/ci-cd.md)
- [docs/terraform.md](/Users/giancedrick/dev/projects/gigi/docs/terraform.md)
- [docs/credits.md](/Users/giancedrick/dev/projects/gigi/docs/credits.md)
- [AGENTS.md](/Users/giancedrick/dev/projects/gigi/AGENTS.md)

## External Resources and Credits

This project requires attribution for external resources. Current credited resources are listed in [docs/credits.md](/Users/giancedrick/dev/projects/gigi/docs/credits.md).
