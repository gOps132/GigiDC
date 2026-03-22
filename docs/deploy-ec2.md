# EC2 Deployment Guide

This guide deploys the Discord bot as a single EC2 service for the reduced V1 architecture.

If you want Terraform to create the EC2 instance for you, start with [docs/terraform.md](/Users/giancedrick/dev/projects/gigi/docs/terraform.md) and then return here for the app-level deployment steps.

If you want automated deploys after the first successful manual setup, continue with [docs/ci-cd.md](/Users/giancedrick/dev/projects/gigi/docs/ci-cd.md).

## Target Topology

- `EC2`: this Discord bot
- `Supabase`: external control-plane data store
- `OpenAI`: external model and embeddings provider

## Security Groups

### Discord bot EC2

Allow inbound:

- `22/tcp` from your admin IP only
- `80/tcp` from the internet if using Nginx for the public health endpoint
- optionally `443/tcp` if you terminate TLS on the instance later

Allow outbound:

- `443/tcp` to the public internet for Discord, Supabase, and OpenAI access

## New EC2 Host Setup

On the new Discord bot EC2:

```bash
sudo bash scripts/bootstrap-ec2.sh
```

This installs:

- Node.js 22
- git
- Nginx
- rsync
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
- `PRIMARY_GUILD_ID`
- `SUPABASE_URL`
- `SUPABASE_SERVICE_ROLE_KEY`
- `OPENAI_API_KEY`
- optional `OPENAI_RESPONSE_MODEL`
- optional `OPENAI_EMBEDDING_MODEL`

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

and proxies them to the Node app on `127.0.0.1:8080`.

## First Deployment

As the deployment user, clone or copy the repo into `/opt/gigi-discord-bot`, then make sure the service user owns that directory:

```bash
sudo chown -R gigi:gigi /opt/gigi-discord-bot
```

Then run:

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

After the first manual setup, you can switch to the GitHub Actions CD path in [docs/ci-cd.md](/Users/giancedrick/dev/projects/gigi/docs/ci-cd.md).

## Verification

Run these checks after deployment:

- `curl http://127.0.0.1:8080/healthz`
- `curl http://YOUR_DISCORD_BOT_PUBLIC_IP/healthz`
- `sudo systemctl status gigi-discord-bot --no-pager`
- `sudo journalctl -u gigi-discord-bot -n 100 --no-pager`
- verify `/ping`
- verify `/assignment create`
- DM the bot with a direct question
- DM the bot with a history question such as `How many times did I say "ship it"?`

## Notes

- A VPC itself is usually not the expensive part of this design.
- The main avoidable extra costs come from NAT gateways, load balancers, public IPv4 usage, and unnecessary public traffic paths.
- When you later add a real domain, replace the default Nginx `server_name _;` with your hostname and terminate TLS there.
