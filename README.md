# Celestia

Celestia is a monorepo for a process-isolated home gateway written in Go with a Vite/React admin console.

## Included Phases

- Phase 0: core runtime, plugin manager, registry, state store, event bus, audit/policy, HTTP gateway, gRPC plugin protocol
- Phase 1: Xiaomi MIoT cloud integration with multi-account, multi-region, aquarium control, and speaker text push
- Phase 2: Petkit cloud integration with feeder/litter/fountain support
- Phase 3: Haier hOn washer integration with model capability matrices

## Local Commands

```bash
go test ./...
make build
npm run build --workspace web/admin
CELESTIA_ADDR=127.0.0.1:8080 ./bin/gateway
```

The gateway serves the admin build from `web/admin/dist` and persists runtime data to SQLite.

## Real Plugin Config

Each vendor plugin now expects real cloud credentials. The admin UI ships JSON templates for:

- Xiaomi: `region` plus `access_token` / `refresh_token` or `auth_code`
- Petkit: `username`, `password`, `region`, `timezone`
- Haier: `email`, `password` or `refresh_token`, plus optional `mobile_id` and `timezone`

If credentials are missing or invalid, plugin enablement fails explicitly instead of falling back to demo devices.

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
