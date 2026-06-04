#!/usr/bin/env bash
# Full release build for Linux (amd64). Run on Linux with Go and Node.js (pure Go SQLite, no CGO).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
APP_NAME="autopack"
GOOS="linux"
GOARCH="amd64"
PKG_ID="${APP_NAME}-${GOOS}-${GOARCH}"
STAGING="${ROOT}/build/${PKG_ID}"
ARCHIVE="${ROOT}/build/${PKG_ID}.tar.gz"
WEB_DIR="${STAGING}/web"
FRONTEND_DIR="${ROOT}/frontend"

echo "==> [1/4] Build frontend"
bash "${ROOT}/scripts/lib/package-frontend.sh" "${FRONTEND_DIR}"

echo "==> [2/4] Build Go binary (${GOOS}/${GOARCH})"
rm -rf "${STAGING}"
mkdir -p "${STAGING}/scripts/sqlite" "${STAGING}/data"
cd "${ROOT}/backend"
export CGO_ENABLED=0
export GOOS GOARCH
go build -ldflags="-s -w" -o "${STAGING}/${APP_NAME}" ./cmd

echo "==> [3/4] Stage runtime files (web static dir + scripts)"
mkdir -p "${WEB_DIR}"
rm -rf "${WEB_DIR:?}"/*
cp -r "${ROOT}/frontend/dist/"* "${WEB_DIR}/"
if ! find "${WEB_DIR}" -name '*.js' -print -quit | grep -q .; then
  echo "ERROR: no JS under ${WEB_DIR}; frontend dist copy failed." >&2
  exit 1
fi
echo "    Static files: ${WEB_DIR} (served by ${APP_NAME} at /autopack/)"

cp -r "${ROOT}/backend/configs" "${STAGING}/configs"
cp "${ROOT}/scripts/sqlite/001_init_schema.sql" "${STAGING}/scripts/sqlite/"
LINUX_SCRIPTS="${ROOT}/scripts/package/linux"
cp "${LINUX_SCRIPTS}/autopack-common.sh" "${LINUX_SCRIPTS}/start.sh" "${LINUX_SCRIPTS}/stop.sh" \
  "${LINUX_SCRIPTS}/restart.sh" "${LINUX_SCRIPTS}/status.sh" "${STAGING}/"
chmod +x "${STAGING}/${APP_NAME}" "${STAGING}/"*.sh
cp "${ROOT}/scripts/package/README.txt" "${STAGING}/README.txt"

echo "==> [4/4] Compress to ${ARCHIVE}"
rm -f "${ARCHIVE}"
tar -czf "${ARCHIVE}" -C "${ROOT}/build" "${PKG_ID}"

echo "Done."
echo "  Directory: ${STAGING}"
echo "  Archive:   ${ARCHIVE}"
echo "  Run:       cd ${STAGING} && ./start.sh"
