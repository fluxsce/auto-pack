#!/bin/sh
# Full release build for Linux (amd64). Requires Go and Node.js (pure Go SQLite, no CGO).
# Usage: sh scripts/package-linux.sh
set -e
set -u

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
cd "${FRONTEND_DIR}"
echo "    npm ci (always reinstall on Linux — do not reuse node_modules from Windows)"
npm ci
echo "    vite build"
node node_modules/vite/bin/vite.js build
if ! find dist -name '*.js' -print -quit 2>/dev/null | grep -q .; then
  echo "ERROR: frontend dist has no JS under ${FRONTEND_DIR}/dist" >&2
  exit 1
fi
echo "    dist OK"
cd "${ROOT}"

echo "==> [2/4] Build Go binary (${GOOS}/${GOARCH})"
rm -rf "${STAGING}"
mkdir -p "${STAGING}/scripts/sqlite" "${STAGING}/data"
cd "${ROOT}/backend"
export CGO_ENABLED=0
export GOOS="${GOOS}"
export GOARCH="${GOARCH}"
go build -ldflags="-s -w" -o "${STAGING}/${APP_NAME}" ./cmd

echo "==> [3/4] Stage runtime files (web static dir + scripts)"
mkdir -p "${WEB_DIR}"
rm -rf "${WEB_DIR}"/*
cp -r "${ROOT}/frontend/dist/"* "${WEB_DIR}/"
if ! find "${WEB_DIR}" -name '*.js' -print -quit 2>/dev/null | grep -q .; then
  echo "ERROR: no JS under ${WEB_DIR}; frontend dist copy failed." >&2
  exit 1
fi
echo "    Static files: ${WEB_DIR} (served by ${APP_NAME} at /autopack/)"

cp -r "${ROOT}/backend/configs" "${STAGING}/configs"
cp "${ROOT}"/scripts/sqlite/*.sql "${STAGING}/scripts/sqlite/"
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
