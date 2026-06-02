#!/usr/bin/env bash

set -euo pipefail

DEPLOY_HOST="${DEPLOY_HOST:?DEPLOY_HOST is required}"
DEPLOY_USER="${DEPLOY_USER:-ubuntu}"
DEPLOY_PORT="${DEPLOY_PORT:-22}"
IMAGE_TAG="${IMAGE_TAG:-local}"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
COMMIT="$(git rev-parse --short HEAD)"

docker build \
  --build-arg VERSION="${IMAGE_TAG}" \
  --build-arg COMMIT="${COMMIT}" \
  --build-arg BUILD_TIME="${BUILD_TIME}" \
  -t "gigi-discord-bot:${IMAGE_TAG}" .

docker save "gigi-discord-bot:${IMAGE_TAG}" -o /tmp/gigi-discord-bot-image.tar

ssh -p "${DEPLOY_PORT}" "${DEPLOY_USER}@${DEPLOY_HOST}" "mkdir -p /tmp/gigi-discord-bot-release/db/migrations"
scp -P "${DEPLOY_PORT}" /tmp/gigi-discord-bot-image.tar "${DEPLOY_USER}@${DEPLOY_HOST}:/tmp/gigi-discord-bot-image.tar"
scp -P "${DEPLOY_PORT}" compose.prod.yaml "${DEPLOY_USER}@${DEPLOY_HOST}:/tmp/gigi-discord-bot-release/compose.prod.yaml"
scp -P "${DEPLOY_PORT}" db/migrations/*.sql "${DEPLOY_USER}@${DEPLOY_HOST}:/tmp/gigi-discord-bot-release/db/migrations/"
scp -P "${DEPLOY_PORT}" scripts/install-release.sh "${DEPLOY_USER}@${DEPLOY_HOST}:/tmp/gigi-discord-bot-release/install-release.sh"

ssh -p "${DEPLOY_PORT}" "${DEPLOY_USER}@${DEPLOY_HOST}" \
  "sudo IMAGE_TAG='${IMAGE_TAG}' RELEASE_DIR=/tmp/gigi-discord-bot-release bash /tmp/gigi-discord-bot-release/install-release.sh"
