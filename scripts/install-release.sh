#!/usr/bin/env bash

set -euo pipefail

APP_DIR="${APP_DIR:-/opt/gigi-discord-bot}"
APP_USER="${APP_USER:-gigi}"
SERVICE_NAME="${SERVICE_NAME:-gigi-discord-bot}"
RELEASE_ARCHIVE="${RELEASE_ARCHIVE:-/tmp/gigi-discord-bot-release.tar.gz}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run this script as root on the Discord bot EC2 instance." >&2
  exit 1
fi

if [[ ! -f "${RELEASE_ARCHIVE}" ]]; then
  echo "Release archive not found: ${RELEASE_ARCHIVE}" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}" "${RELEASE_ARCHIVE}"' EXIT

mkdir -p "${APP_DIR}"
tar -xzf "${RELEASE_ARCHIVE}" -C "${TMP_DIR}"

rsync -a --delete \
  --exclude '.git' \
  --exclude '.env' \
  --exclude '.env.*' \
  "${TMP_DIR}/" "${APP_DIR}/"

cd "${APP_DIR}"
npm ci --omit=dev

chown -R "${APP_USER}:${APP_USER}" "${APP_DIR}"

systemctl restart "${SERVICE_NAME}"
systemctl status "${SERVICE_NAME}" --no-pager

for _ in $(seq 1 15); do
  if curl --fail --silent http://127.0.0.1:8080/healthz >/dev/null; then
    exit 0
  fi
  sleep 2
done

echo "Health check failed for ${SERVICE_NAME} after restart." >&2
exit 1
