#!/bin/sh
# Release notes from feat:/fix: commit subjects between the previous v* tag
# and TAG. Used by .github/workflows/release.yml and for backfilling.
# Usage: release-notes.sh TAG [REPO_URL]
set -eu
tag=$1
repo=${2:-https://github.com/filidorwiese/kekkai}
prev=$(git describe --tags --abbrev=0 --match 'v*' "$tag^" 2>/dev/null || true)
range=${prev:+$prev..}$tag

section() { # $1 heading, $2 type
  lines=$(git log --no-merges --format='- %s (%h)' "$range" | sed -n "s/^- $2\(([^)]*)\)\{0,1\}: /- /p")
  if [ -n "$lines" ]; then printf '## %s\n%s\n\n' "$1" "$lines"; fi
}
section Features feat
section Fixes fix
if [ -n "$prev" ]; then
  printf '**Full Changelog**: %s/compare/%s...%s\n' "$repo" "$prev" "$tag"
else
  printf '**Full Changelog**: %s/commits/%s\n' "$repo" "$tag"
fi
