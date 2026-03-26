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
- Xiaomi: `region`, and for token refresh / auth-code exchange also explicit `client_id` + `redirect_url`
- Petkit: `username`, `password`, `region`, `timezone`
- Haier: `email`, `password` or `refresh_token`, plus optional `mobile_id` and `timezone`

If credentials are missing or invalid, plugin enablement fails explicitly instead of falling back to demo devices.

## Xiaomi OAuth

Celestia now owns the Xiaomi browser OAuth flow:

- Admin can start Xiaomi authorization directly from the Xiaomi plugin card.
- The gateway persists pending/completed OAuth sessions in SQLite.
- The Xiaomi callback URL is Celestia's own `http(s)://<gateway-host>/api/v1/oauth/xiaomi/callback`.
- Xiaomi `client_id` must allow that exact callback URL in your own OAuth application registration.
- The project does not depend on Home Assistant and does not use `homeassistant.local` as a redirect target.

After OAuth completes, the admin UI injects the returned Xiaomi account tokens back into the current config draft so you can save or install the plugin with the refreshed credentials.

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
