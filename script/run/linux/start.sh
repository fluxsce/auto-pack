#!/usr/bin/env bash
set -euo pipefail
APP_HOME="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=common.sh
. "$APP_HOME/script/run/common.sh"
autopack_start "$(autopack_home)"
