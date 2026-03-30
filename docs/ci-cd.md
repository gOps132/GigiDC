---
title: CI/CD
description: Continuous integration and deployment workflow for GigiDC.
---

# CI/CD Guide

The repo includes:

- CI on every push to `main`
- CI on every pull request
- CD on every push to `main`

Workflow file:

- [ci.yml](/Users/giancedrick/dev/projects/gigi/.github/workflows/ci.yml)

## What CI Does

CI runs:

- `npm ci`
- `npm run typecheck`
- `npm run build`
- `terraform fmt -check`
- `terraform init -backend=false`
- `terraform validate`

This gives the repo a hard baseline for:

- TypeScript compile safety
- production build integrity
- Terraform formatting and configuration validity

## What CD Does

On pushes to `main`, after CI passes, the workflow:

1. builds the bot
2. creates a release tarball
3. uploads it to the Discord bot EC2 over SSH
4. runs [install-release.sh](/Users/giancedrick/dev/projects/gigi/scripts/install-release.sh) on that EC2 over SSH
5. installs production dependencies on the server
6. restarts `gigi-discord-bot`
7. checks `http://127.0.0.1:8080/healthz`
8. optionally verify `http://127.0.0.1:8080/readyz` during operational debugging

This avoids:

- `git pull` during deploy
- git ownership drift under `/opt/gigi-discord-bot`

## Required GitHub Secrets

Set these repository secrets before expecting deploys to work:

- `DEPLOY_HOST`
  - Discord bot EC2 public IP or hostname
- `DEPLOY_SSH_KEY`
  - private SSH key contents for the EC2

Optional secrets:

- `DEPLOY_USER`
  - default: `ubuntu`
- `DEPLOY_PORT`
  - default: `22`
- `DEPLOY_APP_DIR`
  - default: `/opt/gigi-discord-bot`
- `DEPLOY_APP_USER`
  - default: `gigi`
- `DEPLOY_SERVICE_NAME`
  - default: `gigi-discord-bot`

## EC2 Requirements

Before CD works, the EC2 still needs the one-time base setup from [docs/deploy-ec2.md](/Users/giancedrick/dev/projects/gigi/docs/deploy-ec2.md):

- Node.js 22
- Nginx
- rsync
- `gigi` user
- `/opt/gigi-discord-bot`
- `/etc/gigi-discord-bot/gigi-discord-bot.env`
- `gigi-discord-bot` systemd service

Important:

- the deploy SSH user must be able to run `sudo` non-interactively for the release install step
- `/opt/gigi-discord-bot` should be owned by `gigi:gigi`
- the EC2 must expose an SSH path that GitHub-hosted runners can reach

## First-Time Rollout

1. Finish the normal EC2 bootstrap manually once.
2. Add the GitHub secrets.
3. Confirm the EC2 can be reached over SSH with the same key stored in `DEPLOY_SSH_KEY`.
4. Push a small commit to `main`.
5. Watch the GitHub Actions run.
6. Verify on the EC2:
   - `sudo systemctl status gigi-discord-bot --no-pager`
   - `curl http://127.0.0.1:8080/healthz`
   - `curl http://127.0.0.1:8080/readyz`
7. Verify in Discord:
   - `/ping`

## Failure Recovery

If a deploy fails:

- inspect the GitHub Actions logs first
- inspect the EC2:
  - `sudo journalctl -u gigi-discord-bot -n 100 --no-pager`
- confirm the runtime env file still exists:
  - `/etc/gigi-discord-bot/gigi-discord-bot.env`
- confirm the service user still owns the app directory:
  - `sudo chown -R gigi:gigi /opt/gigi-discord-bot`
