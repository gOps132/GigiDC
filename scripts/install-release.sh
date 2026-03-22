#!/usr/bin/env bash

set -euo pipefail

APP_DIR="${APP_DIR:-/opt/gigi-discord-bot}"
APP_USER="${APP_USER:-gigi}"
SERVICE_NAME="${SERVICE_NAME:-gigi-discord-bot}"
RELEASE_DIR="$(cd "$(dirname "$0")/.." && pwd)"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run this script as root." >&2
  exit 1
fi

if ! command -v rsync >/dev/null 2>&1; then
  echo "rsync is required for release installs." >&2
  exit 1
fi

if [[ ! -f "${RELEASE_DIR}/package.json" || ! -f "${RELEASE_DIR}/package-lock.json" ]]; then
  echo "Release bundle is incomplete." >&2
  exit 1
fi

mkdir -p "${APP_DIR}"
rsync -a --delete "${RELEASE_DIR}/" "${APP_DIR}/"
chown -R "${APP_USER}:${APP_USER}" "${APP_DIR}"

sudo -u "${APP_USER}" bash -lc "cd '${APP_DIR}' && npm ci --omit=dev"

systemctl restart "${SERVICE_NAME}"
systemctl status "${SERVICE_NAME}" --no-pager
