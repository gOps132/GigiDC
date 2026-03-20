# EC2 Deployment Guide

This guide deploys the Discord bot as a separate EC2 service that talks to your existing OpenClaw instance over private networking.

If you want Terraform to create the second EC2 instance for you, start with [docs/terraform.md](/Users/giancedrick/dev/projects/gigi/docs/terraform.md) and then return here for the app-level deployment steps.

## Target Topology

- `EC2 #1`: existing OpenClaw backend
- `EC2 #2`: this Discord bot
- `Supabase`: external control-plane data store

Recommended network model:

- Both EC2 instances in the same VPC
- Discord bot connects to OpenClaw over the OpenClaw instance private IP or private DNS
- Only the Discord bot instance needs public inbound HTTP for the Clawbot callback endpoint

## Security Groups

### Discord bot EC2

Allow inbound:

- `22/tcp` from your admin IP only
- `80/tcp` from the internet if using Nginx for the callback endpoint
- optionally `443/tcp` if you terminate TLS on the instance later

Allow outbound:

- `443/tcp` to the public internet for Discord and Supabase access
- OpenClaw app port to the OpenClaw instance private IP or security group

### OpenClaw EC2

Allow inbound:

- OpenClaw app port from the Discord bot EC2 security group only
- `22/tcp` from your admin IP only

Do not expose the OpenClaw app port publicly if private routing is available.

## New EC2 Host Setup

On the new Discord bot EC2:

```bash
sudo bash scripts/bootstrap-ec2.sh
```

This installs:

- Node.js 22
- git
- Nginx
- a dedicated `gigi` service user
- the expected app and env directories

## Application Layout

Expected production paths:

- app checkout: `/opt/gigi-discord-bot`
- systemd env file: `/etc/gigi-discord-bot/gigi-discord-bot.env`
- systemd unit: `/etc/systemd/system/gigi-discord-bot.service`
- Nginx site: `/etc/nginx/sites-available/gigi-discord-bot.conf`

## Environment File

Create:

```bash
sudo cp .env.example /etc/gigi-discord-bot/gigi-discord-bot.env
sudo chmod 640 /etc/gigi-discord-bot/gigi-discord-bot.env
sudo chown root:gigi /etc/gigi-discord-bot/gigi-discord-bot.env
```

Fill in real values:

- `NODE_ENV=production`
- `LOG_LEVEL=info`
- `PORT=8080`
- `DISCORD_TOKEN`
- `DISCORD_CLIENT_ID`
- `DISCORD_GUILD_ID`
- `SUPABASE_URL`
- `SUPABASE_SERVICE_ROLE_KEY`
- `BOT_PUBLIC_BASE_URL`
- `CLAWBOT_BASE_URL`
- `CLAWBOT_API_KEY`
- `CLAWBOT_WEBHOOK_SECRET`

Recommended values for your setup:

- `BOT_PUBLIC_BASE_URL=http://YOUR_DISCORD_BOT_PUBLIC_IP`
- `CLAWBOT_BASE_URL=http://OPENCLAW_PRIVATE_IP:3000`

If OpenClaw uses different routes, also set:

- `CLAWBOT_JOB_PATH`
- `CLAWBOT_INGEST_PATH`

## Systemd Setup

Copy the unit file:

```bash
sudo cp deploy/systemd/gigi-discord-bot.service /etc/systemd/system/gigi-discord-bot.service
sudo systemctl daemon-reload
sudo systemctl enable gigi-discord-bot
```

## Nginx Setup

Copy the site:

```bash
sudo cp deploy/nginx/gigi-discord-bot.conf /etc/nginx/sites-available/gigi-discord-bot.conf
sudo ln -sf /etc/nginx/sites-available/gigi-discord-bot.conf /etc/nginx/sites-enabled/gigi-discord-bot.conf
sudo nginx -t
sudo systemctl reload nginx
```

This exposes:

- `GET /healthz`
- `POST /webhooks/clawbot`

and proxies them to the Node app on `127.0.0.1:8080`.

## First Deployment

As the `gigi` user, clone or copy the repo into `/opt/gigi-discord-bot`, then run:

```bash
npm ci
npm run build
```

Start the service:

```bash
sudo systemctl start gigi-discord-bot
sudo systemctl status gigi-discord-bot --no-pager
```

For later deploys:

```bash
bash scripts/deploy-discord-bot.sh
```

## OpenClaw Callback Configuration

Configure OpenClaw to call:

```text
POST {BOT_PUBLIC_BASE_URL}/webhooks/clawbot
```

Send either:

- `Authorization: Bearer {CLAWBOT_WEBHOOK_SECRET}`
- or `x-clawbot-webhook-secret: {CLAWBOT_WEBHOOK_SECRET}`

Expected callback body:

```json
{
  "localJobId": "uuid-from-this-bot",
  "clawbotJobId": "openclaw-job-id",
  "status": "completed",
  "resultSummary": "Safe summary for Discord",
  "artifactLinks": [
    "https://example.com/artifact/1"
  ],
  "errorMessage": null
}
```

## Verification

Run these checks after deployment:

- `curl http://127.0.0.1:8080/healthz`
- `curl http://YOUR_DISCORD_BOT_PUBLIC_IP/healthz`
- `sudo systemctl status gigi-discord-bot --no-pager`
- `sudo journalctl -u gigi-discord-bot -n 100 --no-pager`
- verify `/ping`
- verify `/assignment create`
- enable ingestion on one channel and send a message
- run `/review pr` and confirm OpenClaw callback posting works

## Notes

- A VPC itself is usually not the expensive part of this design.
- The main avoidable extra costs come from NAT gateways, load balancers, public IPv4 usage, and unnecessary public traffic paths.
- When you later add a real domain, update `BOT_PUBLIC_BASE_URL` and replace the default Nginx `server_name _;`.
