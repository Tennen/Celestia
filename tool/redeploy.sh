#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVICE_SCRIPT="${CELESTIA_SERVICE_SCRIPT:-$ROOT_DIR/tool/celestia-service.sh}"
VERIFY_ATTEMPTS="${CELESTIA_REDEPLOY_VERIFY_ATTEMPTS:-20}"
VERIFY_DELAY_SECONDS="${CELESTIA_REDEPLOY_VERIFY_DELAY_SECONDS:-0.5}"

log() {
  printf '[redeploy] %s\n' "$*"
}

run_step() {
  local name="$1"
  shift

  log "Running ${name}"
  (
    cd "$ROOT_DIR"
    "$@"
  )
}

verify_running() {
  local status
  for ((i = 0; i < VERIFY_ATTEMPTS; i++)); do
    status="$("$SERVICE_SCRIPT" status)"
    printf '%s\n' "$status"
    if [[ "$status" == running\ * ]]; then
      return 0
    fi
    sleep "$VERIFY_DELAY_SECONDS"
  done
  "$SERVICE_SCRIPT" logs 80 >&2 || true
  return 1
}

run_step "deploy" "$ROOT_DIR/deploy.sh"
run_step "service stop" "$SERVICE_SCRIPT" stop
run_step "service start" "$SERVICE_SCRIPT" start
log "Verifying service"
verify_running
log "Redeploy completed successfully"
