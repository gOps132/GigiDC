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
- Run the SQL migration files in order:
  - [supabase/migrations/001_initial_schema.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/001_initial_schema.sql)
  - [supabase/migrations/002_clawbot_control_plane.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/002_clawbot_control_plane.sql)
  - [supabase/migrations/003_v1_retrieval.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/003_v1_retrieval.sql)

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

The bot registers slash commands at startup, starts a local health server on port `8080`, stores visible DM and guild message history, and responds to direct messages agentically.

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

## Available Commands and Interaction Modes

- `/ping`
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
- `history_guild_wide`

Example:

```sql
insert into role_policies (guild_id, capability, discord_role_id)
values
  ('your-discord-guild-id', 'assignment_admin', 'your-assignment-admin-role-id'),
  ('your-discord-guild-id', 'history_guild_wide', 'your-history-enabled-role-id');
```

## Retrieval Model

V1 uses a reduced retrieval-first architecture:

- raw Discord messages are the source of truth
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
```

## Testing Guidance

Before opening a PR or deploying:

- Run `npm run typecheck`
- Run `npm run build`
- Verify `/ping`
- Verify `/assignment create` and `/assignment publish` in a development Discord server
- DM the bot with one direct question and one history-based question
- Confirm visible messages are stored in Supabase
- Re-check all modified lines for secrets and private data leaks

## CI/CD

The repo includes a GitHub Actions workflow at [deploy.yml](/Users/giancedrick/dev/projects/gigi/.github/workflows/deploy.yml).

On every push to `main`, it:

- runs `npm ci`
- runs `npm run typecheck`
- runs `npm run build`
- packages a release bundle
- uploads the bundle to the Discord bot EC2
- installs production dependencies on the EC2
- restarts `gigi-discord-bot`

Setup instructions and required GitHub secrets are in [docs/ci-cd.md](/Users/giancedrick/dev/projects/gigi/docs/ci-cd.md).

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
docs/
  discord-bot-plan.md
  setup.md
  credits.md
```

## Documentation

- [docs/discord-bot-plan.md](/Users/giancedrick/dev/projects/gigi/docs/discord-bot-plan.md)
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
