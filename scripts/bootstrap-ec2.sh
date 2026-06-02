#!/usr/bin/env bash

set -euo pipefail

APP_USER="${APP_USER:-gigi}"
APP_DIR="${APP_DIR:-/opt/gigi-discord-bot}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run this script as root on the Discord bot EC2 instance." >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y ca-certificates curl docker.io docker-compose-plugin git nginx rsync

systemctl enable --now docker

if ! id "${APP_USER}" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "${APP_USER}"
fi

usermod -aG docker "${APP_USER}"

mkdir -p "${APP_DIR}"
chown -R "${APP_USER}:${APP_USER}" "${APP_DIR}"

mkdir -p /etc/gigi-discord-bot
chmod 0750 /etc/gigi-discord-bot

cat <<EOF
Bootstrap complete.

Next steps:
1. Copy .env.example to /etc/gigi-discord-bot/gigi-discord-bot.env and fill in real values.
2. Install deploy/nginx/gigi-discord-bot.conf if exposing health checks through Nginx.
3. Deploy the release image and compose files with scripts/install-release.sh.
EOF
