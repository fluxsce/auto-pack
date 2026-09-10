#!/usr/bin/env bash
set -euo pipefail
APP_HOME="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=common.sh
. "$APP_HOME/script/run/common.sh"
ROOT="$(autopack_home)"
autopack_stop "$ROOT"
sleep 0.4
autopack_start "$ROOT"
