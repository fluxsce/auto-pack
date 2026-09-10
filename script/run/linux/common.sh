#!/usr/bin/env bash
# 发行包内启停共用函数。由根目录 start/stop/restart.sh 以 APP_HOME 调用。

set -euo pipefail

autopack_home() {
  local home="${APP_HOME:-}"
  if [[ -z "$home" ]]; then
    echo "APP_HOME is required" >&2
    exit 1
  fi
  cd "$home"
  pwd -P
}

autopack_exe() {
  local root="$1"
  if [[ -x "$root/bin/autopack" ]]; then
    printf '%s\n' "$root/bin/autopack"
    return
  fi
  if [[ -f "$root/bin/autopack" ]]; then
    printf '%s\n' "$root/bin/autopack"
    return
  fi
  echo "binary not found: $root/bin/autopack" >&2
  exit 1
}

autopack_pid_file() {
  printf '%s\n' "$1/run/autopack.pid"
}

autopack_pid_running() {
  local pid="$1"
  [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null
}

autopack_running_pid() {
  local root="$1"
  local file
  file="$(autopack_pid_file "$root")"
  if [[ ! -f "$file" ]]; then
    echo 0
    return
  fi
  local pid
  pid="$(tr -d '[:space:]' <"$file" || true)"
  if [[ -z "$pid" ]] || ! [[ "$pid" =~ ^[0-9]+$ ]]; then
    rm -f "$file"
    echo 0
    return
  fi
  if autopack_pid_running "$pid"; then
    echo "$pid"
    return
  fi
  rm -f "$file"
  echo 0
}

autopack_ensure_runtime() {
  local root="$1"
  mkdir -p "$root/data" "$root/logs" "$root/run"
  if [[ ! -f "$root/configs/app.yaml" ]]; then
    if [[ ! -f "$root/configs/app.yaml.example" ]]; then
      echo "missing $root/configs/app.yaml and app.yaml.example" >&2
      exit 1
    fi
    cp "$root/configs/app.yaml.example" "$root/configs/app.yaml"
    echo "created configs/app.yaml from example; change secrets before production"
  fi
}

autopack_start() {
  local root="$1"
  local alive
  alive="$(autopack_running_pid "$root")"
  if [[ "$alive" != "0" ]]; then
    echo "already running (pid $alive)"
    return 0
  fi
  autopack_ensure_runtime "$root"
  local exe
  exe="$(autopack_exe "$root")"
  if [[ ! -x "$exe" ]]; then
    chmod +x "$exe" || true
  fi
  nohup "$exe" -config "$root/configs" >>"$root/logs/stdout.log" 2>>"$root/logs/stderr.log" &
  local pid=$!
  echo "$pid" >"$(autopack_pid_file "$root")"
  echo "started pid $pid"
  echo "console http://127.0.0.1:8080/autopack/"
}

autopack_stop() {
  local root="$1"
  local alive
  alive="$(autopack_running_pid "$root")"
  if [[ "$alive" == "0" ]]; then
    echo "not running"
    return 0
  fi
  echo "stopping pid $alive"
  kill -TERM "$alive" 2>/dev/null || true
  local i
  for i in $(seq 1 40); do
    if ! autopack_pid_running "$alive"; then
      break
    fi
    sleep 0.2
  done
  if autopack_pid_running "$alive"; then
    kill -KILL "$alive" 2>/dev/null || true
  fi
  rm -f "$(autopack_pid_file "$root")"
  echo "stopped"
}
