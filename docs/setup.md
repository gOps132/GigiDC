# Setup Guide

## Local Development

### Prerequisites

- Node.js 22.12.0 or newer
- npm
- A Discord application and bot
- A Supabase project
- A reachable Clawbot instance

### Environment Setup

1. Copy `.env.example` to `.env`
2. Put your real secrets and IDs into `.env`
3. Keep `.env` uncommitted

Required variables:

- `PORT`
- `DISCORD_TOKEN`
- `DISCORD_CLIENT_ID`
- `DISCORD_GUILD_ID`
- `SUPABASE_URL`
- `SUPABASE_SERVICE_ROLE_KEY`
- `BOT_PUBLIC_BASE_URL`
- `CLAWBOT_BASE_URL`
- `CLAWBOT_API_KEY`
- `CLAWBOT_WEBHOOK_SECRET`

## Discord Setup

1. Create a Discord application in the Developer Portal
2. Add a bot user
3. Enable the permissions your bot needs in your invite URL
4. Enable the `MESSAGE CONTENT INTENT` in the Discord Developer Portal if you want channel-history ingestion
4. Invite the bot to your test server
5. Use your server ID as `DISCORD_GUILD_ID` during development so slash command updates appear quickly

## Supabase Setup

1. Create the Supabase project
2. Open the SQL editor
3. Run [supabase/migrations/001_initial_schema.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/001_initial_schema.sql)
4. Run [supabase/migrations/002_clawbot_control_plane.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/002_clawbot_control_plane.sql)
5. Create `role_policies` rows when you are ready to delegate assignment, ingestion, and Clawbot-dispatch access beyond Discord administrators

Example role policy shape:

```sql
insert into role_policies (guild_id, capability, discord_role_id)
values
  ('your-discord-guild-id', 'assignment_admin', 'your-assignment-admin-role-id'),
  ('your-discord-guild-id', 'ingestion_admin', 'your-ingestion-admin-role-id'),
  ('your-discord-guild-id', 'clawbot_dispatch', 'your-reviewer-role-id');
```

## Clawbot Setup

1. Set `CLAWBOT_BASE_URL` to your running Clawbot instance
   - Recommended on EC2: use the OpenClaw private IP or private DNS when both instances are in the same VPC
2. Set `CLAWBOT_API_KEY` to the secret used for outgoing bot-to-Clawbot requests
3. Set `BOT_PUBLIC_BASE_URL` to the public URL of this bot service
4. Configure Clawbot to call `POST {BOT_PUBLIC_BASE_URL}/webhooks/clawbot` on job completion
5. Configure Clawbot to send `Authorization: Bearer {CLAWBOT_WEBHOOK_SECRET}` or `x-clawbot-webhook-secret: {CLAWBOT_WEBHOOK_SECRET}`
6. If your Clawbot uses different endpoint paths, override `CLAWBOT_JOB_PATH` and `CLAWBOT_INGEST_PATH`
7. For production on EC2, follow [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md)

Expected callback body:

```json
{
  "localJobId": "uuid-from-this-bot",
  "clawbotJobId": "job-id-from-clawbot",
  "status": "completed",
  "resultSummary": "Short safe summary for Discord posting",
  "artifactLinks": [
    "https://example.com/artifact/1"
  ],
  "errorMessage": null
}
```

## Local Verification

Run:

```bash
npm install
npm run typecheck
npm run build
npm run dev
```

Then validate:

- `/ping` responds
- `/assignment create` creates a draft record
- `/assignment list` returns recent assignments
- `/assignment publish` posts to the selected channel or current channel
- `/ingestion enable` marks a channel as ingestible
- A new message in that channel reaches Clawbot
- `/review pr`, `/generate summary`, or `/notes analyze` creates a job and Clawbot can callback successfully

## Safety Notes

- Never expose `SUPABASE_SERVICE_ROLE_KEY`, `CLAWBOT_API_KEY`, or `CLAWBOT_WEBHOOK_SECRET`
- Do not paste real secrets into docs, issues, or pull requests
- Review logs and generated output for sensitive data before sharing
