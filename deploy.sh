#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOCACHE="${GOCACHE:-$ROOT_DIR/.cache/go-build}"
GOMODCACHE="${GOMODCACHE:-$ROOT_DIR/.cache/gomod}"

declare -a OPTIONAL_FAILURES=()

log() {
  printf '[deploy] %s\n' "$*"
}

error() {
  printf '[deploy] ERROR: %s\n' "$*" >&2
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

run_optional_step() {
  local name="$1"
  shift

  log "Running ${name}"
  if (
    cd "$ROOT_DIR"
    "$@"
  ); then
    return 0
  else
    local status=$?
    error "${name} failed with exit code ${status}; continuing deployment"
    OPTIONAL_FAILURES+=("${name} (exit ${status})")
  fi
}

run_step "go test ./..." env GOCACHE="$GOCACHE" GOMODCACHE="$GOMODCACHE" go test ./...
run_step "make build" make build
run_optional_step "make docker-build-hikvision" make docker-build-hikvision
run_step "npm run build --workspace web/admin" npm run build --workspace web/admin

if ((${#OPTIONAL_FAILURES[@]} > 0)); then
  error "Deployment completed with non-blocking step failures: ${OPTIONAL_FAILURES[*]}"
else
  log "Deployment completed successfully"
fi
