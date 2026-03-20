# Gigi Discord Bot

Discord control-plane bot for assignment announcements, role-gated workflows, and Clawbot-backed personalization and agent jobs.

## What Is In This Repo

- A Node 22 + TypeScript Discord bot foundation
- Slash command registration with `discord.js`
- Supabase-backed Discord control-plane state
- Clawbot-backed async jobs and channel-history ingestion
- Working local and Clawbot-backed command sets
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
- `SUPABASE_URL`
- `SUPABASE_SERVICE_ROLE_KEY`
- `BOT_PUBLIC_BASE_URL`
- `CLAWBOT_BASE_URL`
- `CLAWBOT_API_KEY`
- `CLAWBOT_WEBHOOK_SECRET`

Do not commit `.env`.

### 3. Create the Discord application and bot

- Create an application in the Discord Developer Portal
- Create a bot user for that application
- Copy the bot token into `DISCORD_TOKEN`
- Copy the application client ID into `DISCORD_CLIENT_ID`
- Copy your server ID into `DISCORD_GUILD_ID` for guild-scoped slash command registration during development
- Invite the bot to your server with permissions to read, send, and use application commands

### 4. Configure Supabase

- Create a Supabase project
- Copy the project URL into `SUPABASE_URL`
- Copy the server-side service role key into `SUPABASE_SERVICE_ROLE_KEY`
- Run the SQL in both migration files in order:
  - [supabase/migrations/001_initial_schema.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/001_initial_schema.sql)
  - [supabase/migrations/002_clawbot_control_plane.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/002_clawbot_control_plane.sql)

### 5. Configure Clawbot

- Set `CLAWBOT_BASE_URL` to your running Clawbot host, for example `http://YOUR-IP:3000`
  - Recommended on EC2: use the OpenClaw instance private IP or private DNS, not the public IP
- Set `CLAWBOT_API_KEY` to the secret this bot should use when calling Clawbot
- Set `BOT_PUBLIC_BASE_URL` to the public base URL where this bot can receive callbacks
- Configure your Clawbot to call `POST {BOT_PUBLIC_BASE_URL}/webhooks/clawbot`
- Configure Clawbot to send either:
  - `Authorization: Bearer {CLAWBOT_WEBHOOK_SECRET}`
  - or `x-clawbot-webhook-secret: {CLAWBOT_WEBHOOK_SECRET}`
- If your Clawbot uses different routes, override `CLAWBOT_JOB_PATH` and `CLAWBOT_INGEST_PATH`

### 6. Start the bot

```bash
npm run dev
```

The bot registers slash commands at startup, starts a local webhook server on port `8080`, and listens for Discord message events for channels where ingestion is enabled.

## EC2 Deployment

Deployment assets for the recommended two-EC2 layout are included in:

- [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md)
- [docs/terraform.md](/Users/giancedrick/dev/projects/gigi/docs/terraform.md)
- [deploy/systemd/gigi-discord-bot.service](/Users/giancedrick/dev/projects/gigi/deploy/systemd/gigi-discord-bot.service)
- [deploy/nginx/gigi-discord-bot.conf](/Users/giancedrick/dev/projects/gigi/deploy/nginx/gigi-discord-bot.conf)
- [scripts/bootstrap-ec2.sh](/Users/giancedrick/dev/projects/gigi/scripts/bootstrap-ec2.sh)
- [scripts/deploy-discord-bot.sh](/Users/giancedrick/dev/projects/gigi/scripts/deploy-discord-bot.sh)
- [terraform/terraform.tfvars.example](/Users/giancedrick/dev/projects/gigi/terraform/terraform.tfvars.example)

## Available Commands

- `/ping`
- `/assignment create`
- `/assignment publish`
- `/assignment list`
- `/ingestion enable`
- `/ingestion disable`
- `/ingestion status`
- `/review pr`
- `/generate tests`
- `/generate quiz`
- `/generate summary`
- `/notes analyze`

## Authorization Model

The bot checks Discord `Administrator` first, then capability mappings in `role_policies`.

Capabilities used by the current command set:

- `assignment_admin`
- `ingestion_admin`
- `clawbot_dispatch`

Example:

```sql
insert into role_policies (guild_id, capability, discord_role_id)
values
  ('your-discord-guild-id', 'assignment_admin', 'your-assignment-admin-role-id'),
  ('your-discord-guild-id', 'ingestion_admin', 'your-ingestion-admin-role-id'),
  ('your-discord-guild-id', 'clawbot_dispatch', 'your-reviewer-role-id');
```

## Clawbot Contract

This repo assumes:

- job dispatch endpoint: `POST {CLAWBOT_BASE_URL}{CLAWBOT_JOB_PATH}`
- ingestion endpoint: `POST {CLAWBOT_BASE_URL}{CLAWBOT_INGEST_PATH}`
- callback endpoint exposed by this bot: `POST {BOT_PUBLIC_BASE_URL}/webhooks/clawbot`

Dispatch payload includes local job ID, guild/channel/user metadata, task type, command name, input payload, and callback URL.

Callback payload must include:

- `localJobId`
- `clawbotJobId`
- `status` as `completed`, `failed`, or `cancelled`
- optional `resultSummary`
- optional `artifactLinks`
- optional `errorMessage`

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
- Verify `/ingestion enable` and then send a message in that channel to confirm Clawbot receives ingestion events
- Verify one async Clawbot command such as `/review pr`
- Re-check all modified lines for secrets and private data leaks

## Project Structure

```text
src/
  commands/         Slash command handlers
  config/           Environment validation
  discord/          Client bootstrap and command registration
  lib/              Shared clients and logging
  services/         Supabase-backed control-plane services and Clawbot integration
  web/              Webhook callback server
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
- [docs/setup.md](/Users/giancedrick/dev/projects/gigi/docs/setup.md)
- [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md)
- [docs/terraform.md](/Users/giancedrick/dev/projects/gigi/docs/terraform.md)
- [docs/credits.md](/Users/giancedrick/dev/projects/gigi/docs/credits.md)
- [AGENTS.md](/Users/giancedrick/dev/projects/gigi/AGENTS.md)

## External Resources and Credits

This project requires attribution for external resources. Current credited resources are listed in [docs/credits.md](/Users/giancedrick/dev/projects/gigi/docs/credits.md).
