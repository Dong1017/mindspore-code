#!/usr/bin/env bash
set -euo pipefail

echo "Error: local mirror publishing has been removed; releases are GitCode-only." >&2
echo "Use: ./scripts/release.sh <version> [notes]" >&2
exit 1
