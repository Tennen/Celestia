#!/usr/bin/env bash
set -euo pipefail

backend_port="${EZVIZ_BACKEND_PORT:-8099}"

python3 -m uvicorn service.app:app --host 127.0.0.1 --port "${backend_port}" &
backend_pid="$!"

cleanup() {
  if kill -0 "${backend_pid}" >/dev/null 2>&1; then
    kill "${backend_pid}" >/dev/null 2>&1 || true
    wait "${backend_pid}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

export CELESTIA_HIKVISION_BACKEND_BASE_URL="${CELESTIA_HIKVISION_BACKEND_BASE_URL:-http://127.0.0.1:${backend_port}}"

exec /opt/celestia/bin/hikvision-plugin
