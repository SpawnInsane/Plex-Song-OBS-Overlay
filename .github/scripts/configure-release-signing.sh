#!/usr/bin/env bash

set -euo pipefail

if [[ -z "${RELEASE_SIGNING_PRIVATE_KEY:-}" ]]; then
  echo "::error::Configure RELEASE_SIGNING_PRIVATE_KEY with the dedicated SSH private signing key."
  exit 1
fi

signing_key_path="${RUNNER_TEMP:?}/release-signing-key"
umask 077
printf '%s\n' "$RELEASE_SIGNING_PRIVATE_KEY" | tr -d '\r' > "$signing_key_path"
chmod 600 "$signing_key_path"

if ! ssh-keygen -y -f "$signing_key_path" >/dev/null; then
  echo "::error::RELEASE_SIGNING_PRIVATE_KEY is not a valid SSH private key."
  exit 1
fi

git config --local user.name "SpawnInsane"
git config --local user.email "36284692+SpawnInsane@users.noreply.github.com"
git config --local gpg.format ssh
git config --local user.signingkey "$signing_key_path"
git config --local commit.gpgsign true
