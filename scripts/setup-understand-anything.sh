#!/usr/bin/env bash

set -euo pipefail

REPO_URL="${UNDERSTAND_ANYTHING_REPO_URL:-https://github.com/Lum1104/Understand-Anything.git}"
INSTALL_DIR="${UNDERSTAND_ANYTHING_DIR:-$HOME/.codex/understand-anything}"
SKILLS_DIR="${CODEX_SKILLS_DIR:-$HOME/.agents/skills}"
PLUGIN_LINK="${UNDERSTAND_ANYTHING_PLUGIN_LINK:-$HOME/.understand-anything-plugin}"
PLUGIN_ROOT="${INSTALL_DIR}/understand-anything-plugin"
SKILLS=(
  understand
  understand-chat
  understand-dashboard
  understand-diff
  understand-explain
  understand-onboard
)

if ! command -v git >/dev/null 2>&1; then
  echo "git is required to install Understand-Anything." >&2
  exit 1
fi

mkdir -p "${SKILLS_DIR}"

if [[ -d "${INSTALL_DIR}/.git" ]]; then
  echo "Updating existing Understand-Anything checkout at ${INSTALL_DIR}"
  git -C "${INSTALL_DIR}" pull --ff-only
else
  mkdir -p "$(dirname "${INSTALL_DIR}")"
  echo "Cloning Understand-Anything into ${INSTALL_DIR}"
  git clone "${REPO_URL}" "${INSTALL_DIR}"
fi

if [[ ! -d "${PLUGIN_ROOT}" ]]; then
  echo "Understand-Anything plugin root was not found at ${PLUGIN_ROOT}" >&2
  exit 1
fi

for skill in "${SKILLS[@]}"; do
  source_path="${PLUGIN_ROOT}/skills/${skill}"
  target_path="${SKILLS_DIR}/${skill}"

  if [[ ! -d "${source_path}" ]]; then
    echo "Expected skill directory is missing: ${source_path}" >&2
    exit 1
  fi

  if [[ -e "${target_path}" && ! -L "${target_path}" ]]; then
    echo "Skipping ${target_path} because it exists and is not a symlink." >&2
    continue
  fi

  rm -f "${target_path}"
  ln -s "${source_path}" "${target_path}"
done

if [[ -e "${PLUGIN_LINK}" && ! -L "${PLUGIN_LINK}" ]]; then
  echo "Skipping ${PLUGIN_LINK} because it exists and is not a symlink." >&2
else
  rm -f "${PLUGIN_LINK}"
  ln -s "${PLUGIN_ROOT}" "${PLUGIN_LINK}"
fi

echo
echo "Installed Understand-Anything."
echo "Verify with: ls -la \"${SKILLS_DIR}\" | grep understand"
echo "Restart Codex so it can discover the new skills."
