#!/usr/bin/env bash

set -euo pipefail

APP_DIR="${APP_DIR:-/opt/gigi-discord-bot}"
SERVICE_NAME="${SERVICE_NAME:-gigi-discord-bot}"

if [[ ! -d "${APP_DIR}" ]]; then
  echo "APP_DIR does not exist: ${APP_DIR}" >&2
  exit 1
fi

cd "${APP_DIR}"

npm ci
npm run build

sudo systemctl restart "${SERVICE_NAME}"
sudo systemctl status "${SERVICE_NAME}" --no-pager
