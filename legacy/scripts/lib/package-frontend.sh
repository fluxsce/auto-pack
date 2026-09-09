#!/bin/sh
# Build frontend dist on Linux — always npm ci (native deps are OS-specific).
set -e
set -u

FRONTEND_DIR="${1:?frontend directory required}"
cd "${FRONTEND_DIR}"

echo "    npm ci"
npm ci
echo "    vite build"
node node_modules/vite/bin/vite.js build

if ! find dist -name '*.js' -print -quit 2>/dev/null | grep -q .; then
  echo "ERROR: frontend dist has no JS under ${FRONTEND_DIR}/dist" >&2
  exit 1
fi
echo "    dist OK"
