#!/usr/bin/env sh
# Shared helpers for start/stop/restart (POSIX sh).

APP_ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
PID_FILE="${APP_ROOT}/data/autopack.pid"
LOG_DIR="${APP_ROOT}/logs"
LOG_FILE="${LOG_DIR}/autopack-stdout.log"
BINARY="${APP_ROOT}/autopack"

is_running() {
  if [ ! -f "${PID_FILE}" ]; then
    return 1
  fi
  pid="$(cat "${PID_FILE}" 2>/dev/null)" || return 1
  [ -n "${pid}" ] && kill -0 "${pid}" 2>/dev/null
}

start_app() {
  if is_running; then
    echo "autopack already running (pid $(cat "${PID_FILE}"))"
    return 1
  fi
  if [ ! -x "${BINARY}" ]; then
    echo "autopack binary not found or not executable: ${BINARY}" >&2
    return 1
  fi
  mkdir -p "${LOG_DIR}" "${APP_ROOT}/data"
  cd "${APP_ROOT}" || exit 1
  nohup "${BINARY}" >> "${LOG_FILE}" 2>&1 &
  echo "$!" > "${PID_FILE}"
  echo "autopack started (pid $(cat "${PID_FILE}"), log ${LOG_FILE})"
}

stop_app() {
  if ! is_running; then
    rm -f "${PID_FILE}"
    echo "autopack is not running"
    return 0
  fi
  pid="$(cat "${PID_FILE}")"
  echo "stopping autopack (pid ${pid})..."
  kill "${pid}" 2>/dev/null || true
  i=0
  while kill -0 "${pid}" 2>/dev/null; do
    i=$((i + 1))
    if [ "${i}" -ge 30 ]; then
      kill -9 "${pid}" 2>/dev/null || true
      break
    fi
    sleep 1
  done
  rm -f "${PID_FILE}"
  echo "autopack stopped"
}

status_app() {
  if is_running; then
    echo "autopack running (pid $(cat "${PID_FILE}"))"
    return 0
  fi
  echo "autopack stopped"
  return 1
}

restart_app() {
  stop_app || true
  sleep 1
  start_app
}
