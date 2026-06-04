#!/bin/sh
# Normalize shell scripts to Unix LF (run once on Linux if pack fails with "set: invalid option").
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
find "${ROOT}/scripts" -name '*.sh' -type f | while IFS= read -r f; do
  sed -i 's/\r$//' "$f"
  echo "fixed: $f"
done
echo "Done. Re-run: sh scripts/package-linux.sh"
