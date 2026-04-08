# Xiaomi OAuth API

Back to the [API index](../api.md).

## Start Xiaomi OAuth

`POST /api/v1/oauth/xiaomi/start`

Request body:

```json
{
  "plugin_id": "xiaomi",
  "account_name": "Home CN",
  "region": "cn",
  "client_id": "user-supplied-client-id",
  "redirect_base_url": "http://127.0.0.1:8080"
}
```

Response: HTTP `201`

```json
{
  "session": {
    "id": "session-id",
    "provider": "xiaomi",
    "plugin_id": "xiaomi",
    "account_name": "Home CN",
    "region": "cn",
    "client_id": "user-supplied-client-id",
    "redirect_url": "http://127.0.0.1:8080/api/v1/oauth/xiaomi/callback",
    "state": "opaque-state",
    "auth_url": "https://account.xiaomi.com/...",
    "status": "pending",
    "created_at": "2026-04-03T10:00:00Z",
    "updated_at": "2026-04-03T10:00:00Z",
    "state_expires_at": "2026-04-03T10:10:00Z"
  }
}
```

## Get OAuth Session

`GET /api/v1/oauth/xiaomi/sessions/{id}`

Response: the current `OAuthSession`.

## OAuth Callback

`GET /api/v1/oauth/xiaomi/callback`

This endpoint is browser-facing. It returns a short HTML page that:

- posts a `celestia:xiaomi-oauth` message back to the opener window
- closes itself shortly afterwards

The message payload includes:

```json
{
  "type": "celestia:xiaomi-oauth",
  "session_id": "session-id",
  "status": "completed",
  "error": ""
}
```
