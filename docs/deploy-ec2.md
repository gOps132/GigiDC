---
title: EC2 Deployment
description: Deploy the Go foundation runtime to EC2 with Docker Compose.
---

# EC2 Deployment Guide

## Topology

- EC2 host
- Docker Compose
- Go app container
- PostgreSQL + pgvector container
- optional Nginx reverse proxy for health endpoints

## Bootstrap

```bash
sudo bash scripts/bootstrap-ec2.sh
```

This installs Docker, Docker Compose plugin, Nginx, rsync, and creates the `gigi` user plus app/env directories.

## Environment

Create:

```bash
sudo cp .env.example /etc/gigi-discord-bot/gigi-discord-bot.env
sudo chmod 640 /etc/gigi-discord-bot/gigi-discord-bot.env
sudo chown root:gigi /etc/gigi-discord-bot/gigi-discord-bot.env
```

Fill at least:

- `GIGI_ENV=production`
- `GIGI_HTTP_ADDR=:8080`
- `GIGI_DATABASE_URL=postgres://gigi:YOUR_PASSWORD@db:5432/gigi?sslmode=disable`
- `POSTGRES_DB=gigi`
- `POSTGRES_USER=gigi`
- `POSTGRES_PASSWORD=YOUR_PASSWORD`

Discord and OpenAI env vars are reserved for later slices and can stay blank for the foundation runtime.

## Manual Deploy

From your workstation:

```bash
DEPLOY_HOST=your-host IMAGE_TAG=manual bash scripts/deploy-discord-bot.sh
```

## Verify

```bash
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/readyz
docker compose -f /opt/gigi-discord-bot/compose.yaml ps
docker compose -f /opt/gigi-discord-bot/compose.yaml logs --tail=100
```
