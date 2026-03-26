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

- Xiaomi: `region` plus either `username/password`, or `service_token/ssecurity/user_id`
- Xiaomi: optional OAuth `access_token` / `refresh_token` / `auth_code` remains supported, with explicit `client_id` + `redirect_url` required for refresh-token or auth-code exchange
- Petkit: `username`, `password`, `region`, `timezone`
- Haier: `email`, `password` or `refresh_token`, plus optional `mobile_id` and `timezone`

If credentials are missing or invalid, plugin enablement fails explicitly instead of falling back to demo devices.

## Xiaomi Auth

Celestia supports two Xiaomi authentication modes:

1. Preferred pragmatic path: Xiaomi account login via `username/password`, which establishes a real `serviceToken/ssecurity` cloud session inside the plugin.
2. Optional browser OAuth flow: Admin can still start Xiaomi authorization directly from the Xiaomi plugin card. The gateway persists pending/completed OAuth sessions in SQLite, and the callback URL is Celestia's own `http(s)://<gateway-host>/api/v1/oauth/xiaomi/callback`.

For the non-OAuth path, you can also supply an already extracted Xiaomi cloud session by filling `service_token`, `ssecurity`, and `user_id`. If Xiaomi requires captcha or second-factor verification during password login, the plugin now fails explicitly with the upstream verification URL instead of fabricating a session.

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
