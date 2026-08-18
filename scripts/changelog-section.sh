#!/usr/bin/env bash
#
# Prints one version's section from CHANGELOG.md, for use as release notes.
#
#   scripts/changelog-section.sh 0.3.0
#
# The CHANGELOG is written from the user's perspective, which is what a release
# reader wants. Goreleaser's alternative -- generating notes from commit subjects
# -- both leaks implementation detail and silently drops whole categories via its
# exclude filters.

set -euo pipefail

version="${1:-}"
if [ -z "$version" ]; then
  echo "Usage: $0 <version>   (e.g. 0.3.0, with or without a leading v)" >&2
  exit 1
fi

version="${version#v}"

changelog="$(dirname "$0")/../CHANGELOG.md"
if [ ! -f "$changelog" ]; then
  echo "CHANGELOG.md not found at $changelog" >&2
  exit 1
fi

# Print lines after the version's heading, stopping at the next version heading.
section=$(awk -v want="## [$version]" '
  index($0, want) == 1 { found = 1; next }
  found && /^## \[/    { exit }
  found                { print }
' "$changelog")

# Trim leading and trailing blank lines.
section=$(printf '%s\n' "$section" | awk 'NF {p = 1} p' | awk '{lines[NR] = $0} END {last = NR; while (last > 0 && lines[last] == "") last--; for (i = 1; i <= last; i++) print lines[i]}')

if [ -z "$section" ]; then
  echo "No section found for version $version in CHANGELOG.md" >&2
  exit 1
fi

printf '%s\n' "$section"
