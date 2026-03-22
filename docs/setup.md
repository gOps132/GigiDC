# Setup Guide

## Local Development

### Prerequisites

- Node.js 22.12.0 or newer
- npm
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
- `SUPABASE_URL`
- `SUPABASE_SERVICE_ROLE_KEY`
- `OPENAI_API_KEY`

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
2. Open the SQL editor
3. Run [supabase/migrations/001_initial_schema.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/001_initial_schema.sql)
4. Run [supabase/migrations/002_clawbot_control_plane.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/002_clawbot_control_plane.sql)
5. Run [supabase/migrations/003_v1_retrieval.sql](/Users/giancedrick/dev/projects/gigi/supabase/migrations/003_v1_retrieval.sql)
6. Create `role_policies` rows when you are ready to delegate assignment and guild-wide history access beyond Discord administrators

Example role policy shape:

```sql
insert into role_policies (guild_id, capability, discord_role_id)
values
  ('your-discord-guild-id', 'assignment_admin', 'your-assignment-admin-role-id'),
  ('your-discord-guild-id', 'history_guild_wide', 'your-history-enabled-role-id');
```

## OpenAI Setup

1. Set `OPENAI_API_KEY`
2. Optionally set `OPENAI_RESPONSE_MODEL`
3. Optionally set `OPENAI_EMBEDDING_MODEL`
4. For production on EC2, follow [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md)

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
- `/assignment create` creates a draft notice record
- `/assignment list` returns recent assignments
- `/assignment publish` posts to the selected channel or current channel and mentions affected roles
- DM the bot with a general question
- DM the bot with a history question like `How many times did I say "ship it"?`
- Confirm raw messages and embeddings are being written for visible text messages

## Safety Notes

- Never expose `SUPABASE_SERVICE_ROLE_KEY` or `OPENAI_API_KEY`
- Do not paste real secrets into docs, issues, or pull requests
- Review logs and generated output for sensitive data before sharing
