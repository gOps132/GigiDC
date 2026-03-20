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
apt-get install -y curl git nginx ca-certificates gnupg

if ! command -v node >/dev/null 2>&1; then
  mkdir -p /etc/apt/keyrings
  curl -fsSL https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key \
    | gpg --dearmor -o /etc/apt/keyrings/nodesource.gpg
  echo "deb [signed-by=/etc/apt/keyrings/nodesource.gpg] https://deb.nodesource.com/node_22.x nodistro main" \
    > /etc/apt/sources.list.d/nodesource.list
  apt-get update
  apt-get install -y nodejs
fi

if ! id "${APP_USER}" >/dev/null 2>&1; then
  useradd --create-home --shell /bin/bash "${APP_USER}"
fi

mkdir -p "${APP_DIR}"
chown -R "${APP_USER}:${APP_USER}" "${APP_DIR}"

mkdir -p /etc/gigi-discord-bot
chmod 0750 /etc/gigi-discord-bot

cat <<EOF
Bootstrap complete.

Next steps:
1. Copy this repo into ${APP_DIR}
2. Copy .env.example to /etc/gigi-discord-bot/gigi-discord-bot.env and fill in real values
3. Install the systemd unit from deploy/systemd/gigi-discord-bot.service
4. Install the Nginx site from deploy/nginx/gigi-discord-bot.conf
5. Run npm ci && npm run build as ${APP_USER}
EOF
