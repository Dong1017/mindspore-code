#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

VERSION=""
NOTES=""

for arg in "$@"; do
  case "$arg" in
    --*)
      echo "Error: unsupported option ${arg}; releases are GitCode-only" >&2
      exit 1
      ;;
    *)
      if [ -z "$VERSION" ]; then
        VERSION="$arg"
      elif [ -z "$NOTES" ]; then
        NOTES="$arg"
      fi
      ;;
  esac
done

if [ -z "$VERSION" ]; then
  echo "Usage: ./scripts/release-all.sh <version> [notes]"
  echo ""
  echo "Examples:"
  echo "  ./scripts/release-all.sh v0.5.1 \"Fix bug\""
  exit 1
fi

if [[ "${VERSION}" != v* ]]; then
  echo "Error: version must include a leading v, for example v0.5.1" >&2
  exit 1
fi

: "${NOTES:="Release $VERSION"}"

# ── Ensure we're on main ────────────────────────────────────────
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$CURRENT_BRANCH" != "main" ]; then
  echo "Error: must release from main (currently on $CURRENT_BRANCH)" >&2
  exit 1
fi

# ── Step 0: Update embedded skills ─────────────────────────────
echo "==> Update embedded skills"
"${SCRIPT_DIR}/update-skills.sh"

echo "==> GitCode release"
"${SCRIPT_DIR}/release.sh" "$VERSION" "$NOTES"

echo ""
echo "Done. Release $VERSION complete."
