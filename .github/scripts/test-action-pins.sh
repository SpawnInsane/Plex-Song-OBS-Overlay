#!/usr/bin/env bash
set -euo pipefail

fixture_directory="$(mktemp -d)"
trap 'rm -rf "$fixture_directory"' EXIT
printf '%s\n' 'steps:' '  - uses: owner/action@0123456789abcdef0123456789abcdef01234567 # v1' > "$fixture_directory/pinned.yml"
printf '%s\n' 'steps:' '  - uses: ./local-action' > "$fixture_directory/local.yml"
printf '%s\n' 'steps:' '  - uses: owner/action@v1' > "$fixture_directory/mutable.yml"
printf '%s\n' 'steps:' '  - uses : owner/action@v1' > "$fixture_directory/spaced-mutable.yml"
printf '%s\n' 'steps:' '  - uses: owner/action@v1 # source @0123456789abcdef0123456789abcdef01234567' > "$fixture_directory/comment-bypass.yml"
printf '%s\n' 'env:' '  ACTION: &mutable owner/action@v1' 'steps:' '  - uses: *mutable' > "$fixture_directory/alias-bypass.yml"
awk -f .github/scripts/check-action-pins.awk "$fixture_directory/pinned.yml" "$fixture_directory/local.yml"
if awk -f .github/scripts/check-action-pins.awk "$fixture_directory/mutable.yml"; then exit 1; fi
if awk -f .github/scripts/check-action-pins.awk "$fixture_directory/spaced-mutable.yml"; then exit 1; fi
if awk -f .github/scripts/check-action-pins.awk "$fixture_directory/comment-bypass.yml"; then exit 1; fi
if awk -f .github/scripts/check-action-pins.awk "$fixture_directory/alias-bypass.yml"; then exit 1; fi
