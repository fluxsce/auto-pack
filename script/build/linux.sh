#!/usr/bin/env bash
# 全局构建入口（Linux）：前端 + 后端 + 按平台组装发行目录。
# 产物：build/dist/auto-pack-{version}-{os}-{arch}/ 以及同名 tar.gz / zip。
set -euo pipefail

OS=linux
ARCH=amd64
SKIP_WEB=0
SKIP_ARCHIVE=0

usage() {
  cat <<'EOF'
Usage: script/build/linux.sh [--os linux|windows] [--arch amd64|arm64] [--skip-web] [--skip-archive]
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --os)
      OS="$2"
      shift 2
      ;;
    --arch)
      ARCH="$2"
      shift 2
      ;;
    --skip-web)
      SKIP_WEB=1
      shift
      ;;
    --skip-archive)
      SKIP_ARCHIVE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown arg: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ "$OS" != "linux" && "$OS" != "windows" ]]; then
  echo "invalid --os $OS" >&2
  exit 1
fi
if [[ "$ARCH" != "amd64" && "$ARCH" != "arm64" ]]; then
  echo "invalid --arch $ARCH" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

step() { printf '==> %s\n' "$1"; }

# 发行打包机常是干净 clone：没有 node_modules，vue-tsc 不在 PATH。
# @niuma/ui 走 npm:niuma-ui，不依赖旁路仓库。
ensure_web_deps() {
  command -v pnpm >/dev/null || { echo "pnpm not found" >&2; exit 1; }
  if [[ ! -d "$ROOT/node_modules" || ! -e "$ROOT/node_modules/.bin/vue-tsc" ]]; then
    step "pnpm install (node_modules missing)"
    if [[ -f "$ROOT/pnpm-lock.yaml" ]]; then
      pnpm install --frozen-lockfile
    else
      pnpm install
    fi
  fi
}

VERSION=""
if command -v python3 >/dev/null 2>&1; then
  VERSION="$(python3 -c 'import json; print(json.load(open("package.json", encoding="utf-8"))["version"])' 2>/dev/null || true)"
fi
if [[ -z "$VERSION" ]]; then
  VERSION="$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' package.json | head -n 1)"
fi
if [[ -z "$VERSION" ]]; then
  echo "version not found in package.json" >&2
  exit 1
fi

if command -v node >/dev/null 2>&1; then
  step "sync AppVersion from package.json"
  node "$ROOT/script/sync-version.mjs"
else
  echo "node not found; AppVersion will come from ldflags" >&2
fi

FOLDER="auto-pack-${VERSION}-${OS}-${ARCH}"
DIST="$ROOT/build/dist"
PKG="$DIST/$FOLDER"

step "package $FOLDER"
rm -rf "$PKG"
mkdir -p "$PKG"

if [[ "$SKIP_WEB" -eq 0 ]]; then
  ensure_web_deps
  step "pnpm build (web -> server/web)"
  pnpm build
else
  [[ -f "$ROOT/server/web/index.html" ]] || { echo "SkipWeb but server/web/index.html missing" >&2; exit 1; }
fi
[[ -f "$ROOT/server/web/index.html" ]] || { echo "frontend missing: server/web/index.html" >&2; exit 1; }

BIN_NAME=autopack
if [[ "$OS" == "windows" ]]; then
  BIN_NAME=autopack.exe
fi
mkdir -p "$PKG/bin"
step "go build $OS/$ARCH -> $PKG/bin/$BIN_NAME"
CGO_ENABLED=0 GOOS="$OS" GOARCH="$ARCH" go build -C "$ROOT/server" -trimpath \
  -ldflags "-s -w -X auto-pack/server/internal/config.AppVersion=${VERSION}" \
  -o "$PKG/bin/$BIN_NAME" ./cmd

step "stage $PKG"
mkdir -p "$PKG/web"
cp -R "$ROOT/server/web/." "$PKG/web/"
find "$PKG/web" -type f -name '*.map' -delete

mkdir -p "$PKG/configs"
[[ -f "$ROOT/script/package/app.yaml.example" ]] || { echo "packaged config template missing" >&2; exit 1; }
cp "$ROOT/script/package/app.yaml.example" "$PKG/configs/app.yaml.example"

for engine in sqlite mysql postgres; do
  src="$ROOT/script/db/$engine"
  dst="$PKG/script/db/$engine"
  mkdir -p "$dst"
  [[ -f "$src/init.sql" ]] || { echo "DDL missing: $src/init.sql" >&2; exit 1; }
  for f in init.sql 98_patch.sql 99_seed.sql; do
    [[ -f "$src/$f" ]] && cp "$src/$f" "$dst/$f"
  done
done

mkdir -p "$PKG/script/run"
if [[ "$OS" == "windows" ]]; then
  cp "$ROOT/script/run/windows/service.ps1" "$PKG/script/run/service.ps1"
  cp "$ROOT/script/run/windows/start.bat" "$PKG/start.bat"
  cp "$ROOT/script/run/windows/stop.bat" "$PKG/stop.bat"
  cp "$ROOT/script/run/windows/restart.bat" "$PKG/restart.bat"
else
  cp "$ROOT/script/run/linux/common.sh" "$PKG/script/run/common.sh"
  cp "$ROOT/script/run/linux/start.sh" "$PKG/start.sh"
  cp "$ROOT/script/run/linux/stop.sh" "$PKG/stop.sh"
  cp "$ROOT/script/run/linux/restart.sh" "$PKG/restart.sh"
  chmod +x "$PKG/bin/autopack" "$PKG/start.sh" "$PKG/stop.sh" "$PKG/restart.sh" "$PKG/script/run/common.sh"
fi

cp "$ROOT/script/package/README.txt" "$PKG/README.txt"
printf '%s\n' "$VERSION" >"$PKG/VERSION"

if [[ "$SKIP_ARCHIVE" -eq 0 ]]; then
  mkdir -p "$DIST"
  if [[ "$OS" == "windows" ]]; then
    archive="$DIST/${FOLDER}.zip"
    rm -f "$archive"
    if command -v zip >/dev/null 2>&1; then
      (cd "$DIST" && zip -qr "$archive" "$FOLDER")
    else
      tar -C "$DIST" -czf "${DIST}/${FOLDER}.tar.gz" "$FOLDER"
      archive="${DIST}/${FOLDER}.tar.gz"
    fi
    echo "archive $archive"
  else
    archive="$DIST/${FOLDER}.tar.gz"
    rm -f "$archive"
    tar -C "$DIST" -czf "$archive" "$FOLDER"
    echo "archive $archive"
  fi
fi

echo "ready $PKG"
if [[ "$OS" == "windows" ]]; then
  echo "start: $PKG/start.bat"
else
  echo "start: $PKG/start.sh"
fi
