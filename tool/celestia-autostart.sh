#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVICE_SCRIPT="$ROOT_DIR/tool/celestia-service.sh"
LABEL="com.celestia.autostart"
DOMAIN="gui/$(id -u)"
PLIST_FILE="$HOME/Library/LaunchAgents/$LABEL.plist"
RUNTIME_DIR="${CELESTIA_RUNTIME_DIR:-$ROOT_DIR/data/runtime}"
SERVICE_PATH="${CELESTIA_SERVICE_PATH:-/opt/homebrew/bin:/usr/local/bin:${PATH:-/usr/bin:/bin:/usr/sbin:/sbin}}"

xml_escape() {
  local value="$1"
  value="${value//&/\&amp;}"
  value="${value//</\&lt;}"
  value="${value//>/\&gt;}"
  value="${value//\"/\&quot;}"
  value="${value//\'/\&apos;}"
  printf '%s' "$value"
}

render_plist() {
  cat <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>$LABEL</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string>
    <string>$(xml_escape "$SERVICE_SCRIPT")</string>
    <string>start</string>
  </array>
  <key>WorkingDirectory</key><string>$(xml_escape "$ROOT_DIR")</string>
  <key>RunAtLoad</key><true/>
  <key>EnvironmentVariables</key>
  <dict>
    <key>PATH</key><string>$(xml_escape "$SERVICE_PATH")</string>
    <key>CELESTIA_SERVICE_PATH</key><string>$(xml_escape "$SERVICE_PATH")</string>
    <key>CELESTIA_ADDR</key><string>$(xml_escape "${CELESTIA_ADDR:-0.0.0.0:8080}")</string>
    <key>CELESTIA_RUNTIME_DIR</key><string>$(xml_escape "$RUNTIME_DIR")</string>
    <key>CELESTIA_GATEWAY_BIN</key><string>$(xml_escape "${CELESTIA_GATEWAY_BIN:-$ROOT_DIR/bin/gateway}")</string>
    <key>CELESTIA_GATEWAY_PID_FILE</key><string>$(xml_escape "${CELESTIA_GATEWAY_PID_FILE:-$RUNTIME_DIR/gateway.pid}")</string>
    <key>CELESTIA_GATEWAY_LOG_FILE</key><string>$(xml_escape "${CELESTIA_GATEWAY_LOG_FILE:-$RUNTIME_DIR/gateway.log}")</string>
    <key>CELESTIA_LAUNCHD_LABEL</key><string>$(xml_escape "${CELESTIA_LAUNCHD_LABEL:-com.celestia.gateway}")</string>
  </dict>
  <key>StandardOutPath</key><string>$(xml_escape "$RUNTIME_DIR/autostart.log")</string>
  <key>StandardErrorPath</key><string>$(xml_escape "$RUNTIME_DIR/autostart.log")</string>
</dict>
</plist>
EOF
}

install_autostart() {
  if [[ ! -x "${CELESTIA_GATEWAY_BIN:-$ROOT_DIR/bin/gateway}" ]]; then
    printf 'Build the gateway before installing autostart.\n' >&2
    return 1
  fi
  mkdir -p "$(dirname "$PLIST_FILE")" "$RUNTIME_DIR"
  local staging_file
  staging_file="$(mktemp "$PLIST_FILE.XXXXXX")"
  render_plist >"$staging_file"
  if ! plutil -lint "$staging_file"; then
    rm -f "$staging_file"
    return 1
  fi
  chmod 644 "$staging_file"
  mv "$staging_file" "$PLIST_FILE"
  launchctl bootout "$DOMAIN/$LABEL" >/dev/null 2>&1 || true
  launchctl enable "$DOMAIN/$LABEL"
  launchctl bootstrap "$DOMAIN" "$PLIST_FILE"
  local status
  for ((attempt = 0; attempt < 40; attempt++)); do
    status="$("$SERVICE_SCRIPT" status)"
    if [[ "$status" == running\ * ]]; then
      printf 'autostart installed: %s\n%s\n' "$PLIST_FILE" "$status"
      return 0
    fi
    sleep 1
  done
  printf 'Autostart installed, but the gateway did not start; inspect %s/autostart.log\n' "$RUNTIME_DIR" >&2
  return 1
}

uninstall_autostart() {
  launchctl bootout "$DOMAIN/$LABEL" >/dev/null 2>&1 || true
  rm -f "$PLIST_FILE"
  printf 'autostart removed; the current gateway is unchanged\n'
}

status_autostart() {
  if [[ -f "$PLIST_FILE" ]]; then
    printf 'autostart installed: %s\n' "$PLIST_FILE"
    launchctl print "$DOMAIN/$LABEL"
  else
    printf 'autostart not installed\n'
  fi
}

if [[ "$(uname -s)" != Darwin ]]; then
  printf 'Celestia autostart requires macOS.\n' >&2
  exit 1
fi

case "${1:-status}" in
  install) install_autostart ;;
  uninstall) uninstall_autostart ;;
  status) status_autostart ;;
  print) render_plist ;;
  *) printf 'usage: %s {install|uninstall|status|print}\n' "$0" >&2; exit 2 ;;
esac
