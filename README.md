# Celestia

Celestia is a monorepo for a process-isolated home gateway written in Go with a Vite/React admin console.

## Included Phases

- Phase 0: core runtime, plugin manager, registry, state store, event bus, audit/policy, HTTP gateway, gRPC plugin protocol
- Phase 1: Xiaomi MIoT cloud integration with multi-account, multi-region, aquarium control, and speaker text push
- Phase 2: Petkit cloud integration with feeder/litter/fountain support
- Phase 3: Haier hOn washer integration with model capability matrices
- Phase 4: Hikvision/EZVIZ local LAN integration with HCNetSDK PTZ and playback

## Local Commands

```bash
go test ./...
make build
make docker-build-hikvision
npm run build --workspace web/admin
./deploy.sh
CELESTIA_ADDR=127.0.0.1:8080 ./bin/gateway
go run ./cmd/celctl dashboard
```

`./deploy.sh` runs the same build sequence as the README commands. If `make docker-build-hikvision` fails, the script prints an error and continues the remaining deployment steps.

The gateway serves the admin build from `web/admin/dist` and persists runtime data to SQLite.

## Real Plugin Config

Each vendor plugin now expects real cloud credentials. The admin UI ships JSON templates for:

- Xiaomi: `region` plus either `username/password`, or `service_token/ssecurity/user_id`
- Xiaomi: optional OAuth `access_token` / `refresh_token` / `auth_code` remains supported, with explicit `client_id` + `redirect_url` required for refresh-token or auth-code exchange
- Petkit: `username`, `password`, `region`, `timezone`
- Petkit: optional `compat` overrides for `passport_base_url`, `china_base_url`, `api_version`, `client_header`, `user_agent`, and related app-signature fields when Petkit changes its mobile app contract
- Haier: `email`, `password` or `refresh_token`, plus optional `mobile_id` and `timezone`
- Hikvision: `sdk_lib_dir` plus `entries[]` with `host`, `port`, `username`, `password`, `channel`, and optional `rtsp_*` / `ptz_*` / `sdk_lib_dir_override`

If credentials are missing or invalid, plugin enablement fails explicitly instead of falling back to demo devices.

Plugin config defaults are now exposed by Core through the catalog schema. The admin `Config` view consumes that Core-owned default draft instead of maintaining a separate frontend-only preset list.

## Xiaomi Auth

Celestia supports two Xiaomi authentication modes:

1. Preferred pragmatic path: Xiaomi account login via `username/password`, which establishes a real `serviceToken/ssecurity` cloud session inside the plugin.
2. Optional browser OAuth flow: Admin can still start Xiaomi authorization directly from the Xiaomi plugin card. The gateway persists pending/completed OAuth sessions in SQLite, and the callback URL is Celestia's own `http(s)://<gateway-host>/api/v1/oauth/xiaomi/callback`.

For the non-OAuth path, you can also supply an already extracted Xiaomi cloud session by filling `service_token`, `ssecurity`, and `user_id`. If Xiaomi requires captcha or second-factor verification during password login, the plugin now fails explicitly with the upstream verification URL instead of fabricating a session.

## Plugin Runtime Mechanics

### Core orchestration

- Core starts each enabled plugin as a separate process and connects to it over the existing gRPC plugin protocol.
- On plugin enable or manual discover, Core calls `DiscoverDevices`, upserts the returned device inventory, then calls `GetDeviceState` per device to seed the unified state store.
- Runtime events flow from plugin `StreamEvents` into Core's event store and state store. If an event payload includes `state`, Core persists that snapshot immediately.
- Plugin health is checked by Core every 15 seconds through `HealthCheck`.
- Device commands always go through the HTTP API, then policy evaluation and audit logging, then `PluginMgr.ExecuteCommand`, which forwards the normalized command over gRPC to the owning plugin.

### Xiaomi MIoT

- Default poll interval is 30 seconds. The plugin enforces a minimum of 15 seconds even if config is lower.
- `Start` performs an immediate full refresh, then repeats full account/device polling on the ticker.
- Discovery is cloud-backed. Each account lists homes and devices through Xiaomi cloud APIs, builds unified device metadata from MIoT spec data, then reads current properties through `miotspec/prop/get` or the OAuth equivalent.
- Single-device refresh reads only that device's mapped MIoT properties and updates the plugin cache.
- Commands are property/action based:
  - property writes use MIoT `prop/set`
  - speaker or action-style commands use MIoT `action`
  - after every accepted command the plugin refreshes that single device again
- Xiaomi emits `device.state.changed` events during poll cycles when a state diff is detected, and also after post-command refreshes. OAuth or session-derived credentials are persisted back through Core-owned plugin config updates.

### Petkit

- Default poll interval is 30 seconds. The plugin also enforces a minimum of 30 seconds.
- `Start` launches a poll loop that performs a refresh immediately, then waits for the next tick.
- Sync is account-wide and HTTP-based. Each cycle logs in if needed, loads family/group membership, loads device detail per device, and loads feeder records for feeder models.
- There is no direct vendor push stream. Petkit events are synthesized from polling deltas plus the latest feeder event record when available.
- Device commands are routed by device kind:
  - feeder commands use the Petkit cloud control endpoints
  - litter box commands call `controlDevice`
  - fountain commands go through the Petkit BLE relay flow, including connect, poll, and control requests over Petkit cloud HTTP APIs
- After command execution, the plugin refreshes the target device again when possible. Runtime session data such as session id and expiry is persisted back to Core-owned plugin config.
- Current implementation detail: `RefreshDeviceByID` performs a full account sync and then selects the requested device from that result, so an explicit single-device read still traverses the whole Petkit account.

### Haier hOn

- Default poll interval is 20 seconds. The plugin enforces a minimum of 10 seconds.
- `Start` performs an immediate refresh and then runs account polling on the ticker.
- Each refresh authenticates with refresh token or email/password, persists any new refresh token through Core config persistence, loads appliances, loads per-model command metadata, and then loads attributes plus optional statistics and maintenance payloads.
- Haier command support is model-driven. The plugin derives a capability matrix from vendor command metadata and maps normalized Core actions onto the discovered vendor command names.
- Command execution is a direct vendor send:
  - Core action -> Haier command mapping -> `commands/v1/send`
  - then a single-device refresh reloads attributes/statistics/maintenance for the targeted washer
- State-change events are emitted on explicit single-device refresh paths, such as post-command refreshes. The background poll keeps the plugin's internal cache fresh but does not currently emit per-device change events for every polling diff.

### Hikvision EZVIZ (Native on linux/arm64, Docker fallback elsewhere)

- The Hikvision plugin uses HCNetSDK arm64 shared libraries.
- On `linux/arm64`, Celestia now installs and runs `hikvision-plugin` like the other plugins. The root `make build` target and root `Dockerfile` build an SDK-enabled binary for that platform and expose the bundled SDK under `/opt/celestia/sdk/lib/arm64` in the gateway image.
- On non-`linux/arm64` environments, the same install flow still works, but `hikvision-plugin` falls back to launcher mode and starts the dedicated Hikvision Docker runtime.
- Hikvision device identity is now derived from `host` + `port` + `channel`, so renaming an entry updates the existing device instead of leaving the old name behind as a stale row.
- The standalone Docker runtime remains available. `plugins/hikvision/Dockerfile` still builds the server-mode image for independent container execution, and `CELESTIA_HIKVISION_PLUGIN_MODE=launcher` can be used to force the Docker path even on `linux/arm64`.
- Build the standalone plugin image from repository root:

```bash
docker buildx build --platform linux/arm64 -f plugins/hikvision/Dockerfile -t celestia-hikvision-plugin:latest .
```

- Optional gateway-side environment variables:
  - `CELESTIA_HIKVISION_DOCKER_IMAGE` (default `celestia-hikvision-plugin:latest`)
  - `CELESTIA_HIKVISION_DOCKER_PLATFORM` (for example `linux/arm64`)
  - `CELESTIA_HIKVISION_DOCKER_NETWORK` (for example `bridge` or `host`)
  - `CELESTIA_HIKVISION_SDK_LIB_DIR` (optional override for the HCNetSDK library directory used by the current runtime)
  - `CELESTIA_HIKVISION_PLUGIN_MODE` (`launcher` to force Docker fallback, `server` only for native linux/arm64 SDK builds or the standalone plugin container)
- Plugin config draft example:

```json
{
  "sdk_lib_dir": "/opt/celestia/sdk/lib/arm64",
  "entries": [
    {
      "name": "front-door",
      "host": "192.168.1.100",
      "port": 8000,
      "username": "admin",
      "password": "<hikvision-password>",
      "channel": 1
    }
  ],
  "poll_interval_seconds": 30
}
```
- The plugin directly uses HCNetSDK in-process, maps each configured camera entry to `camera_like`, supports PTZ movement and playback commands, and emits state/command events back to Core.

## Repository Layout

- `cmd/gateway`: gateway entrypoint that wires SQLite storage, runtime reconciliation, HTTP API, and graceful shutdown.
- `cmd/celctl`: agent-oriented CLI built on Cobra with a structured subcommand surface for plugins/devices/events/audits and normalized command dispatch.
- `internal/api/http`: the only supported admin and external control surface. It serves device, plugin, audit, event, and OAuth endpoints.
- `internal/core`: Core runtime services for plugin management, registry, state, audit, policy, event bus, quick-control modeling, and Xiaomi OAuth orchestration.
- `internal/coreapi`: Core-owned gRPC helpers that plugins use for approved back-calls such as config persistence.
- `internal/models`: shared runtime models exchanged across Core, plugins, storage, and API layers.
- `internal/pluginapi`: generated/handwritten gRPC protocol bindings and struct encoding helpers for plugin RPCs.
- `internal/pluginruntime`: shared plugin server scaffolding used by vendor plugin binaries.
- `internal/pluginutil`: small shared helpers used by plugin implementations.
- `internal/storage/sqlite`: production persistence for plugin records, devices, states, events, audits, OAuth sessions, and control preferences.
- `internal/xiaomi/oauth`: Xiaomi OAuth helpers shared by Core and the Xiaomi plugin.
- `plugins/xiaomi`: Xiaomi MIoT plugin process. `internal/app` owns plugin RPC behavior, `internal/cloud` owns cloud auth and MIoT requests, `internal/mapper` turns MIoT models into unified capabilities, and `internal/spec` caches MIoT spec data.
- `plugins/petkit`: Petkit plugin process. `internal/app` contains auth, sync, mapping, command dispatch, BLE relay handling, and runtime config persistence.
- `plugins/haier`: Haier hOn plugin process. `internal/app` contains auth, appliance discovery, capability derivation, command mapping, refresh, and token persistence.
- `plugins/hikvision`: Hikvision/EZVIZ plugin process. `cmd/main.go` is the single plugin entrypoint and auto-selects native server mode on linux/arm64 SDK builds, with launcher mode plus Docker fallback elsewhere unless explicitly overridden. `internal/app` hosts config validation, direct HCNetSDK lifecycle/login/command handling, state polling, PTZ/playback command mapping, and runtime events. `plugins/hikvision/Dockerfile` packages the HCNetSDK runtime.
- `proto`: plugin protocol definition.
- `web/admin`: Vite/React admin console that consumes only the gateway HTTP API.
- `docs`: repository Markdown docs, including API references.
- `docs/cli.md`: CLI tooling decision and shared API/CLI service architecture.
- `data`: local runtime SQLite databases and smoke-test data paths.
- `bin`: built gateway/plugin binaries.

## Docker

```bash
docker compose up --build
```

The container exposes the gateway and admin UI on port `8080`.

## API Docs

- External device query/control API and admin control preference endpoints: [docs/api.md](/Users/chentianyu/workspace/private/Celestia/docs/api.md)

## Admin Surface

- Dashboard summary
- Plugin catalog, install, runtime view, Core-owned config view, enable/disable, discover, uninstall, logs
- Device inventory with live state
- Command dispatch with actor header support
- Event feed and audit feed
