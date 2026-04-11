# Runtime And Plugin Management API

Back to the [API index](../api.md).

These routes stay under `/api/v1` and are used by the admin UI and core runtime-management flows.

## Health

`GET /api/v1/health`

Response:

```json
{
  "status": "ok",
  "time": "2026-04-03T10:15:00Z"
}
```

## Dashboard Summary

`GET /api/v1/dashboard`

Response:

```json
{
  "plugins": 4,
  "enabled_plugins": 3,
  "devices": 12,
  "online_devices": 9,
  "events": 138,
  "audits": 42
}
```

## Plugin Catalog

`GET /api/v1/catalog/plugins`

Returns the built-in installable plugin catalog:

```json
[
  {
    "id": "xiaomi",
    "name": "Xiaomi Cloud",
    "description": "Xiaomi MIoT cloud integration",
    "binary_name": "xiaomi-plugin",
    "manifest": {
      "id": "xiaomi",
      "name": "Xiaomi Cloud",
      "version": "1.0.0",
      "vendor": "Celestia",
      "capabilities": ["discovery", "commands", "events"],
      "device_kinds": ["light", "switch", "aquarium", "speaker"]
    }
  }
]
```

## Installed Plugins

`GET /api/v1/plugins`

Returns runtime state for installed plugins:

```json
[
  {
    "record": {
      "plugin_id": "xiaomi",
      "version": "1.0.0",
      "status": "enabled",
      "binary_path": "/abs/path/bin/xiaomi-plugin",
      "config": {},
      "installed_at": "2026-04-03T09:00:00Z",
      "updated_at": "2026-04-03T09:10:00Z",
      "last_health_status": "healthy"
    },
    "manifest": {
      "id": "xiaomi",
      "name": "Xiaomi Cloud",
      "version": "1.0.0",
      "vendor": "Celestia",
      "capabilities": ["discovery", "commands", "events"],
      "device_kinds": ["light", "switch", "aquarium", "speaker"]
    },
    "health": {
      "plugin_id": "xiaomi",
      "status": "healthy",
      "message": "",
      "checked_at": "2026-04-03T10:15:00Z"
    },
    "running": true,
    "recent_logs": []
  }
]
```

## Install Plugin

`POST /api/v1/plugins`

Request body:

```json
{
  "plugin_id": "xiaomi",
  "binary_path": "/abs/path/bin/xiaomi-plugin",
  "config": {
    "accounts": []
  },
  "metadata": {}
}
```

Response: HTTP `201` with a `PluginInstallRecord`.

## Update Plugin Config

`PUT /api/v1/plugins/{plugin_id}/config`

Request body:

```json
{
  "config": {
    "accounts": []
  }
}
```

Response: HTTP `200` with the updated `PluginInstallRecord`.

For camera plugins, Core-owned config may include transport hints that the Admin relay uses directly. For example, Hikvision accepts:

```json
{
  "config": {
    "mode": "lan",
    "stream_rtsp_transport": "tcp",
    "entries": [
      {
        "name": "front-door",
        "host": "192.168.1.100",
        "username": "admin",
        "password": "<hikvision-password>"
      }
    ]
  }
}
```

`stream_rtsp_transport` supports `udp` (default) and `tcp`. Switching to `tcp` is useful when Admin live preview shows macroblocking, green frames, or unstable playback over UDP.

## Enable / Disable / Discover / Delete Plugin

`POST /api/v1/plugins/{plugin_id}/enable`

`POST /api/v1/plugins/{plugin_id}/disable`

`POST /api/v1/plugins/{plugin_id}/discover`

`DELETE /api/v1/plugins/{plugin_id}`

Response:

```json
{
  "ok": true
}
```

## Plugin Logs

`GET /api/v1/plugins/{plugin_id}/logs`

Response:

```json
{
  "plugin_id": "xiaomi",
  "logs": [
    "2026-04-03T10:00:00Z connected",
    "2026-04-03T10:01:00Z discovery finished"
  ]
}
```
