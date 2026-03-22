# CI/CD Deployment

This repo includes a GitHub Actions workflow that builds the bot and deploys a release bundle to the Discord bot EC2 on every push to `main`.

Workflow file:

- [deploy.yml](/Users/giancedrick/dev/projects/gigi/.github/workflows/deploy.yml)

## Deployment Model

- CI runs on GitHub Actions
- build and typecheck happen in CI, not on the EC2
- CI uploads a tarball release bundle to the Discord bot EC2 over SSH
- the EC2 installs production dependencies and restarts `gigi-discord-bot`

This avoids:

- manual `git pull` on the EC2
- GitHub credentials on the server
- git ownership drift in `/opt/gigi-discord-bot`

## One-Time EC2 Requirements

Before enabling the workflow, make sure the bot EC2 has:

- Node.js 22
- `rsync`
- the `gigi-discord-bot` systemd service installed
- the environment file at `/etc/gigi-discord-bot/gigi-discord-bot.env`
- passwordless `sudo` for the SSH user that runs deploys

If you used [scripts/bootstrap-ec2.sh](/Users/giancedrick/dev/projects/gigi/scripts/bootstrap-ec2.sh), install `rsync` if it was missing from earlier runs:

```bash
sudo apt-get update
sudo apt-get install -y rsync
```

Make sure the app directory is owned by the service user:

```bash
sudo mkdir -p /opt/gigi-discord-bot
sudo chown -R gigi:gigi /opt/gigi-discord-bot
```

## Required GitHub Secrets

Add these repository secrets in GitHub:

- `DEPLOY_HOST`
  - Value: your Discord bot EC2 public IP or hostname
- `DEPLOY_USER`
  - Value: the SSH user used for deploys, usually `ubuntu`
- `DEPLOY_SSH_KEY`
  - Value: the private SSH key contents that can access the Discord bot EC2

Optional secrets if you need non-default values:

- `DEPLOY_PORT`
  - Default: `22`
- `DEPLOY_APP_DIR`
  - Default: `/opt/gigi-discord-bot`
- `DEPLOY_APP_USER`
  - Default: `gigi`
- `DEPLOY_SERVICE_NAME`
  - Default: `gigi-discord-bot`

## Release Contents

The workflow packages:

- `dist/`
- `package.json`
- `package-lock.json`
- `.env.example`
- [scripts/install-release.sh](/Users/giancedrick/dev/projects/gigi/scripts/install-release.sh)

The server keeps its real runtime configuration in:

- `/etc/gigi-discord-bot/gigi-discord-bot.env`

That file is never shipped from CI.

## First-Time Setup

1. Add the GitHub secrets listed above.
2. Confirm the EC2 is reachable with the same SSH key you stored in `DEPLOY_SSH_KEY`.
3. Confirm manual deploys already work once.
4. Push a small commit to `main` or run the workflow manually from GitHub Actions.

## Verification

After a workflow run:

- check the GitHub Actions job log
- run `sudo systemctl status gigi-discord-bot --no-pager` on the EC2
- run `curl http://127.0.0.1:8080/healthz`
- run `curl http://YOUR_DISCORD_BOT_PUBLIC_IP/healthz`
- verify `/ping` in Discord

## Failure Recovery

If deploy fails on the EC2:

- inspect the workflow logs first
- inspect `sudo journalctl -u gigi-discord-bot -n 100 --no-pager`
- confirm `/etc/gigi-discord-bot/gigi-discord-bot.env` still exists and is readable by the service
- confirm `/opt/gigi-discord-bot` is writable during deployment and readable by the `gigi` user
