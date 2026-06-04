#!/usr/bin/env bash
# Build frontend dist. Skips npm ci when node_modules is present.
set -euo pipefail

FRONTEND_DIR="${1:?frontend directory required}"
cd "${FRONTEND_DIR}"

VITE_JS="node_modules/vite/bin/vite.js"
if [[ ! -f "${VITE_JS}" ]]; then
  echo "    npm ci (node_modules missing)"
  npm ci
else
  echo "    skip npm ci (reuse node_modules)"
fi

echo "    vite build"
node "${VITE_JS}" build

if ! find dist -name '*.js' -print -quit | grep -q .; then
  echo "ERROR: frontend dist has no JS under ${FRONTEND_DIR}/dist" >&2
  exit 1
fi
echo "    dist OK"
