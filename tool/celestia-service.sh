#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_DIR="${CELESTIA_RUNTIME_DIR:-$ROOT_DIR/data/runtime}"
PID_FILE="${CELESTIA_GATEWAY_PID_FILE:-$RUNTIME_DIR/gateway.pid}"
LOG_FILE="${CELESTIA_GATEWAY_LOG_FILE:-$RUNTIME_DIR/gateway.log}"
BIN="${CELESTIA_GATEWAY_BIN:-$ROOT_DIR/bin/gateway}"
ADDR="${CELESTIA_ADDR:-0.0.0.0:8080}"

mkdir -p "$RUNTIME_DIR"

running_pid() {
  if [[ ! -f "$PID_FILE" ]]; then
    return 1
  fi
  local pid
  pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [[ -z "$pid" ]]; then
    return 1
  fi
  if kill -0 "$pid" 2>/dev/null; then
    printf '%s\n' "$pid"
    return 0
  fi
  return 1
}

start_service() {
  if pid="$(running_pid)"; then
    printf 'already_running pid=%s log=%s\n' "$pid" "$LOG_FILE"
    return 0
  fi
  if [[ ! -x "$BIN" ]]; then
    printf 'gateway binary is not executable: %s\n' "$BIN" >&2
    return 1
  fi
  {
    printf '\n[%s] starting gateway addr=%s bin=%s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$ADDR" "$BIN"
  } >>"$LOG_FILE"
  (
    cd "$ROOT_DIR"
    nohup env CELESTIA_ADDR="$ADDR" "$BIN" >>"$LOG_FILE" 2>&1 &
    printf '%s\n' "$!" >"$PID_FILE"
  )
  printf 'started pid=%s log=%s\n' "$(cat "$PID_FILE")" "$LOG_FILE"
}

stop_service() {
  if ! pid="$(running_pid)"; then
    rm -f "$PID_FILE"
    printf 'stopped log=%s\n' "$LOG_FILE"
    return 0
  fi
  kill "$pid"
  for _ in {1..30}; do
    if ! kill -0 "$pid" 2>/dev/null; then
      rm -f "$PID_FILE"
      printf 'stopped pid=%s log=%s\n' "$pid" "$LOG_FILE"
      return 0
    fi
    sleep 0.2
  done
  printf 'failed to stop pid=%s\n' "$pid" >&2
  return 1
}

status_service() {
  if pid="$(running_pid)"; then
    printf 'running pid=%s addr=%s log=%s\n' "$pid" "$ADDR" "$LOG_FILE"
  else
    printf 'stopped addr=%s log=%s\n' "$ADDR" "$LOG_FILE"
  fi
}

restart_service() {
  (
    sleep 1
    "$0" stop >/dev/null 2>&1 || true
    "$0" start >/dev/null 2>&1
  ) &
  printf 'restart_scheduled log=%s\n' "$LOG_FILE"
}

logs_service() {
  local lines="${1:-120}"
  if [[ ! -f "$LOG_FILE" ]]; then
    printf 'log file does not exist: %s\n' "$LOG_FILE"
    return 0
  fi
  tail -n "$lines" "$LOG_FILE"
}

case "${1:-status}" in
  start)
    start_service
    ;;
  stop)
    stop_service
    ;;
  restart)
    restart_service
    ;;
  status)
    status_service
    ;;
  logs)
    logs_service "${2:-120}"
    ;;
  *)
    printf 'usage: %s {start|stop|restart|status|logs [lines]}\n' "$0" >&2
    exit 2
    ;;
esac
