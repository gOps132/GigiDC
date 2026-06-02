#!/usr/bin/env bash

set -euo pipefail

APP_DIR="${APP_DIR:-/opt/gigi-discord-bot}"
APP_USER="${APP_USER:-gigi}"
ENV_FILE="${ENV_FILE:-/etc/gigi-discord-bot/gigi-discord-bot.env}"
IMAGE_ARCHIVE="${IMAGE_ARCHIVE:-/tmp/gigi-discord-bot-image.tar}"
RELEASE_DIR="${RELEASE_DIR:-/tmp/gigi-discord-bot-release}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run this script as root on the Discord bot EC2 instance." >&2
  exit 1
fi

if [[ ! -f "${IMAGE_ARCHIVE}" ]]; then
  echo "Image archive not found: ${IMAGE_ARCHIVE}" >&2
  exit 1
fi

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Environment file not found: ${ENV_FILE}" >&2
  exit 1
fi

if [[ ! -f "${RELEASE_DIR}/compose.prod.yaml" ]]; then
  echo "compose.prod.yaml not found in ${RELEASE_DIR}" >&2
  exit 1
fi

mkdir -p "${APP_DIR}/db/migrations"
cp "${RELEASE_DIR}/compose.prod.yaml" "${APP_DIR}/compose.yaml"
rsync -a --delete "${RELEASE_DIR}/db/migrations/" "${APP_DIR}/db/migrations/"

docker load -i "${IMAGE_ARCHIVE}"

cd "${APP_DIR}"
GIGI_IMAGE_TAG="${IMAGE_TAG}" docker compose --env-file "${ENV_FILE}" -f compose.yaml up -d

chown -R "${APP_USER}:${APP_USER}" "${APP_DIR}"

for _ in $(seq 1 30); do
  if curl --fail --silent http://127.0.0.1:8080/healthz >/dev/null; then
    exit 0
  fi
  sleep 2
done

echo "Health check failed for Gigi after Docker Compose deploy." >&2
docker compose --env-file "${ENV_FILE}" -f compose.yaml ps >&2 || true
docker compose --env-file "${ENV_FILE}" -f compose.yaml logs --tail=100 >&2 || true
exit 1
