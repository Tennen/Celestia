#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUNTIME_DIR="${CELESTIA_RUNTIME_DIR:-$ROOT_DIR/data/runtime}"
PID_FILE="${CELESTIA_GATEWAY_PID_FILE:-$RUNTIME_DIR/gateway.pid}"
LOG_FILE="${CELESTIA_GATEWAY_LOG_FILE:-$RUNTIME_DIR/gateway.log}"
RESTART_PID_FILE="${CELESTIA_RESTART_PID_FILE:-$RUNTIME_DIR/gateway-restart.pid}"
BIN="${CELESTIA_GATEWAY_BIN:-$ROOT_DIR/bin/gateway}"
ADDR="${CELESTIA_ADDR:-0.0.0.0:8080}"
STOP_TIMEOUT_SECONDS="${CELESTIA_STOP_TIMEOUT_SECONDS:-30}"
RESTART_DELAY_SECONDS="${CELESTIA_RESTART_DELAY_SECONDS:-1}"

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

stop_wait_ticks() {
  local seconds="$STOP_TIMEOUT_SECONDS"
  if [[ ! "$seconds" =~ ^[0-9]+$ ]] || (( seconds <= 0 )); then
    seconds=30
  fi
  printf '%s\n' $((seconds * 5))
}

wait_for_pid_exit() {
  local pid="$1"
  local ticks
  ticks="$(stop_wait_ticks)"
  for ((i = 0; i < ticks; i++)); do
    if ! kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
    sleep 0.2
  done
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
  if wait_for_pid_exit "$pid"; then
    rm -f "$PID_FILE"
    printf 'stopped pid=%s log=%s\n' "$pid" "$LOG_FILE"
    return 0
  fi
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
  if [[ ! -x "$BIN" ]]; then
    printf 'gateway binary is not executable: %s\n' "$BIN" >&2
    return 1
  fi
  nohup env \
    CELESTIA_RUNTIME_DIR="$RUNTIME_DIR" \
    CELESTIA_GATEWAY_PID_FILE="$PID_FILE" \
    CELESTIA_GATEWAY_LOG_FILE="$LOG_FILE" \
    CELESTIA_RESTART_PID_FILE="$RESTART_PID_FILE" \
    CELESTIA_GATEWAY_BIN="$BIN" \
    CELESTIA_ADDR="$ADDR" \
    CELESTIA_STOP_TIMEOUT_SECONDS="$STOP_TIMEOUT_SECONDS" \
    CELESTIA_RESTART_DELAY_SECONDS="$RESTART_DELAY_SECONDS" \
    "$0" restart-worker >>"$LOG_FILE" 2>&1 < /dev/null &
  printf '%s\n' "$!" >"$RESTART_PID_FILE"
  printf 'restart_scheduled pid=%s log=%s\n' "$(cat "$RESTART_PID_FILE")" "$LOG_FILE"
}

restart_worker() {
  printf '\n[%s] restart worker starting\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  sleep "$RESTART_DELAY_SECONDS"
  if ! stop_service; then
    if pid="$(running_pid)"; then
      printf '[%s] graceful stop timed out; sending SIGKILL pid=%s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$pid" >&2
      kill -KILL "$pid" 2>/dev/null || true
      if ! wait_for_pid_exit "$pid"; then
        printf '[%s] failed to stop pid=%s after SIGKILL\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$pid" >&2
        return 1
      fi
      rm -f "$PID_FILE"
      printf '[%s] stopped pid=%s with SIGKILL\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$pid"
    fi
  fi
  start_service
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
  restart-worker)
    restart_worker
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
