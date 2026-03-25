# Celestia

Celestia is a monorepo for a process-isolated home gateway written in Go with a Vite/React admin console.

## Included Phases

- Phase 0: core runtime, plugin manager, registry, state store, event bus, audit/policy, HTTP gateway, gRPC plugin protocol
- Phase 1: Xiaomi plugin scaffold with multi-account, multi-region, light/switch/sensor/climate mapping, aquarium control, speaker voice push, and command/state/event flow
- Phase 2: Petkit plugin scaffold with feeder/litter/fountain capability coverage
- Phase 3: Haier washer plugin scaffold with model capability matrix behavior

## Local Commands

```bash
go test ./...
make build
npm run build --workspace web/admin
CELESTIA_ADDR=127.0.0.1:8080 ./bin/gateway
```

The gateway serves the admin build from `web/admin/dist` and persists runtime data to SQLite.

## Docker

```bash
docker compose up --build
```

The container exposes the gateway and admin UI on port `8080`.

## Admin Surface

- Dashboard summary
- Plugin catalog, install, config, enable/disable, discover, uninstall, logs
- Device inventory with live state
- Command dispatch with actor header support
- Event feed and audit feed
