# Celestia API

Celestia exposes three HTTP surfaces:

- `/api/v1` for the admin UI and general runtime management
- `/api/external/v1` for stable device query/control
- `/api/ai/v1` for AI-agent-oriented device discovery and command invocation

All JSON endpoints return `Content-Type: application/json`.

## Error Shapes

Most failures return:

```json
{
  "error": "human-readable message"
}
```

Policy-denied command requests return HTTP `403` with:

```json
{
  "allowed": false,
  "reason": "denied reason"
}
```

AI name-resolution conflicts return HTTP `409` with:

```json
{
  "error": "device \"Kitchen Lamp\" is ambiguous",
  "field": "device",
  "value": "Kitchen Lamp",
  "matches": [
    {
      "device_id": "xiaomi:cn:lamp-1",
      "device_name": "Kitchen Lamp"
    }
  ]
}
```

Command endpoints accept an optional `X-Actor` header. When omitted:

- `/api/v1/...` and `/api/external/v1/...` command routes default to `admin`
- `/api/ai/v1/commands` defaults to `ai`

## Admin Runtime API

### Health

`GET /api/v1/health`

Response:

```json
{
  "status": "ok",
  "time": "2026-04-03T10:15:00Z"
}
```

### Dashboard Summary

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

### Plugin Catalog

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

### Installed Plugins

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

### Install Plugin

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

### Update Plugin Config

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

### Enable / Disable / Discover / Delete Plugin

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

### Plugin Logs

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

### Automations

These routes stay under `/api/v1` and let the admin UI manage Core-owned state-change automations.

Each automation has:

- one or more `conditions`
- one or more `type: "state_changed"` conditions; any matching state-change condition can start the automation
- zero or more `type: "current_state"` conditions combined by `condition_logic` as extra runtime gates
- an optional daily time window
- one or more actions executed against existing devices

Supported match operators:

- `any` for `type: "state_changed"` condition `from`
- `equals`
- `not_equals`
- `in`
- `not_in`
- `exists`
- `missing`

Condition shapes:

- `type: "state_changed"` uses `from` and `to`
- `type: "current_state"` uses `match` against the latest persisted device state

For `in` and `not_in`, `value` must be a JSON array. This allows one rule to match transitions like `D -> A/B/C` on the same state key.

`time_window.start` and `time_window.end` use `HH:MM` in the gateway's local timezone. Ranges that cross midnight are supported.

#### List Automations

`GET /api/v1/automations`

Response:

```json
[
  {
    "id": "automation-1",
    "name": "Washer Done Voice Push",
    "enabled": true,
    "condition_logic": "all",
    "conditions": [
      {
        "type": "state_changed",
        "device_id": "haier:washer:home:washer-1",
        "state_key": "phase",
        "from": {
          "operator": "not_equals",
          "value": "ready"
        },
        "to": {
          "operator": "in",
          "value": ["ready", "dry_done", "wash_done"]
        }
      },
      {
        "type": "current_state",
        "device_id": "haier:washer:home:washer-1",
        "state_key": "machine_status",
        "match": {
          "operator": "equals",
          "value": "idle"
        }
      }
    ],
    "time_window": {
      "start": "08:00",
      "end": "23:00"
    },
    "actions": [
      {
        "device_id": "xiaomi:cn:speaker-1",
        "label": "Suggested · Voice push",
        "action": "push_voice_message",
        "params": {
          "message": "洗衣机已结束",
          "volume": 55
        }
      }
    ],
    "last_triggered_at": "2026-04-03T10:20:00Z",
    "last_run_status": "succeeded",
    "last_error": "",
    "created_at": "2026-04-03T09:50:00Z",
    "updated_at": "2026-04-03T10:20:00Z"
  }
]
```

#### Create Automation

`POST /api/v1/automations`

Request body: the automation payload without a required `id`. Core assigns one when missing.

Response: HTTP `200` with the persisted `Automation`.

#### Update Automation

`PUT /api/v1/automations/{automation_id}`

Request body: the automation payload. The path `automation_id` wins over any body `id`.

Response: HTTP `200` with the persisted `Automation`.

#### Delete Automation

`DELETE /api/v1/automations/{automation_id}`

Response:

```json
{
  "ok": true
}
```

## Device Query And Control API

`/api/v1/devices` and `/api/external/v1/devices` share the same response shape.

The `/api/external/v1` routes are the stable device query/control surface. The matching `/api/v1` routes are what the admin UI and `celctl` use internally.

### List Devices

`GET /api/v1/devices`

`GET /api/external/v1/devices`

Optional query parameters:

- `plugin_id`
- `kind`
- `q`

Response:

```json
[
  {
    "device": {
      "id": "xiaomi:cn:123456",
      "plugin_id": "xiaomi",
      "kind": "switch",
      "name": "Living Room Switch",
      "default_name": "Mi Smart Switch",
      "alias": "Living Room Switch",
      "room": "Living Room",
      "online": true,
      "capabilities": ["state", "commands"]
    },
    "state": {
      "device_id": "xiaomi:cn:123456",
      "plugin_id": "xiaomi",
      "ts": "2026-03-26T11:45:00Z",
      "state": {
        "toggle_2_1": true
      }
    },
    "controls": [
      {
        "id": "toggle-2-1",
        "kind": "toggle",
        "label": "Left Switch",
        "default_label": "Switch 1",
        "alias": "Left Switch",
        "visible": true,
        "state": true
      },
      {
        "id": "light-mode",
        "kind": "select",
        "label": "Light Mode",
        "visible": true,
        "value": "daylight",
        "options": [
          { "value": "daylight", "label": "Daylight" },
          { "value": "plant", "label": "Plant" }
        ],
        "command": {
          "action": "set_light_mode",
          "value_param": "value"
        }
      },
      {
        "id": "pump-level",
        "kind": "number",
        "label": "Pump Level",
        "visible": true,
        "value": 2,
        "min": 1,
        "max": 3,
        "step": 1,
        "command": {
          "action": "set_pump_level",
          "value_param": "value"
        }
      }
    ]
  }
]
```

Some devices may expose state-label metadata in `device.metadata.state_descriptors`.
This is especially useful for enum-like vendor states where the raw state value is a code but the UI should render a friendly label.

Example:

```json
{
  "device": {
    "metadata": {
      "state_descriptors": {
        "phase": {
          "label": "程序阶段",
          "options": [
            { "value": "11", "label": "烘干中" },
            { "value": "12", "label": "烘干程序结束" }
          ]
        },
        "prPhase": {
          "label": "程序阶段",
          "hidden": true
        }
      }
    }
  }
}
```

Clients should continue to compare against `state.state[key]` using the raw value, while using `state_descriptors[key].options` and `state_descriptors[key].label` for display and selection.

`controls[].kind` supports:

- `toggle` with boolean `state`
- `action` with one-shot execution through the action endpoint; action controls may also expose `command.action` as the underlying normalized action
- `select` with `value`, `options`, and `command`
- `number` with `value`, optional `min` / `max` / `step`, and `command`

For `select` and `number` controls, clients should call the generic device command endpoint using the embedded `command.action` and `command.value_param`.

Search behavior for `q` matches:

- `device_id`
- vendor-reported device name
- saved device alias
- room

### Get One Device

`GET /api/v1/devices/{device_id}`

`GET /api/external/v1/devices/{device_id}`

Returns the same shape as the list endpoint for a single device.

`controls` is always present. Devices without quick controls return `"controls": []`.

### Toggle Control

`POST /api/v1/toggle/{device_id.control_id}/on`

`POST /api/v1/toggle/{device_id.control_id}/off`

`POST /api/external/v1/toggle/{device_id.control_id}/on`

`POST /api/external/v1/toggle/{device_id.control_id}/off`

Optional header:

- `X-Actor: your-client-name`

Response:

```json
{
  "decision": {
    "allowed": true,
    "risk_level": "low"
  },
  "result": {
    "accepted": true,
    "message": "command accepted"
  }
}
```

### Run Action Control

`POST /api/v1/action/{device_id.control_id}`

`POST /api/external/v1/action/{device_id.control_id}`

Optional header:

- `X-Actor: your-client-name`

Response shape matches the toggle endpoint.

### Send Advanced Command

`POST /api/v1/devices/{device_id}/commands`

`POST /api/external/v1/devices/{device_id}/commands`

Request body:

```json
{
  "action": "feed_once",
  "params": {
    "portions": 1
  }
}
```

Optional header:

- `X-Actor: your-client-name`

Response shape matches the toggle endpoint.

Notes:

- Petkit feeder `feed_once` is a Celestia normalized command name.
- Additional feeder actions accepted by this generic endpoint include `manual_feed_dual`, `cancel_manual_feed`, `reset_desiccant`, `food_replenished`, `play_sound`, and `call_pet` when the selected model supports them.
- This generic endpoint is also how callers reach vendor-specific actions that are not exposed as quick controls.

Example dual-hopper feeder request:

```json
{
  "action": "manual_feed_dual",
  "params": {
    "amount1": 20,
    "amount2": 20
  }
}
```

## Admin Device Preference API

These routes stay under `/api/v1` and are used by the admin UI to customize device and quick-control presentation.

### Update Device Alias

`PUT /api/v1/devices/{device_id}/preference`

Request body:

```json
{
  "alias": "Kitchen Feeder"
}
```

Response:

```json
{
  "device_id": "petkit:feeder:pet-parent",
  "alias": "Kitchen Feeder",
  "updated_at": "2026-04-03T10:20:00Z"
}
```

Behavior:

- `alias` sets a per-device display alias while preserving the vendor-reported name as `device.default_name`
- sending `alias: ""` resets the device back to the vendor-reported `device.name`
- subsequent `GET /api/v1/devices`, `GET /api/v1/devices/{id}`, `GET /api/external/v1/devices`, and `GET /api/external/v1/devices/{id}` responses reflect the alias
- device list search `q` also matches saved aliases

### Update Control Alias / Visibility

`PUT /api/v1/devices/{device_id}/controls/{control_id}`

Request body:

```json
{
  "alias": "Left Switch",
  "visible": false
}
```

Response:

```json
{
  "device_id": "xiaomi:cn:123456",
  "control_id": "toggle-2-1",
  "alias": "Left Switch",
  "visible": false,
  "updated_at": "2026-04-03T10:20:00Z"
}
```

Behavior:

- `alias` sets a per-device display alias for that quick control
- `visible: false` hides the control from the main admin quick-control area
- `visible: true` shows it again
- sending `alias: ""` and `visible: true` resets the control back to default behavior

The updated preference is persisted in SQLite and reflected in subsequent `GET /api/v1/devices` and `GET /api/external/v1/devices` responses.

## AI Agent API

These routes stay under `/api/ai/v1` and are optimized for semantic device lookup and invocation.

The AI catalog is intentionally minimal:

- device `name` plus `aliases`
- command `name` plus `aliases`
- user-settable `params`
- fixed/default command params only when they matter for semantic disambiguation

The AI catalog is generated from device control metadata. To invoke vendor-specific commands that are not declared as controls, use the raw `action` form on the AI command endpoint.

### List AI Devices

`GET /api/ai/v1/devices`

Optional query parameters:

- `plugin_id`
- `kind`
- `q`

Response:

```json
[
  {
    "id": "petkit:feeder:pet-parent",
    "name": "Kitchen Feeder",
    "aliases": ["Pet Feeder"],
    "commands": [
      {
        "name": "Feed Once",
        "aliases": ["feed-once", "feed_once"],
        "action": "feed_once",
        "params": [
          {
            "name": "portions",
            "type": "number",
            "default": 1,
            "min": 1,
            "step": 1
          }
        ]
      },
      {
        "name": "Power",
        "aliases": ["power"],
        "action": "set_power",
        "params": [
          {
            "name": "on",
            "type": "boolean",
            "required": true
          }
        ]
      }
    ]
  }
]
```

Notes:

- command aliases include control aliases, default labels, control IDs, and unique underlying action names when that mapping is unambiguous
- commands hidden from the admin quick-control area are still queryable here if the underlying control metadata exists
- select parameters accept either the option `value` or its `label`

### Invoke AI Command

`POST /api/ai/v1/commands`

This endpoint supports semantic resolution and raw action execution.

#### 1. Semantic target resolution

Request body:

```json
{
  "target": "Kitchen Feeder.Feed Once",
  "params": {
    "portions": 2
  }
}
```

Alternative explicit form:

```json
{
  "device_name": "Kitchen Feeder",
  "command": "Feed Once",
  "params": {
    "portions": 2
  }
}
```

Direct command-only form:

```json
{
  "command": "Feed Once",
  "params": {
    "portions": 2
  }
}
```

Room-qualified form:

```json
{
  "target": "Kitchen.Feed Once",
  "params": {
    "portions": 2
  }
}
```

Semantic resolution rules for request fields:

- `target: "device-or-room.command"` resolves by splitting on the last `.`
- `target: "command"` is treated as a direct command lookup across all devices
- if a device or command name itself contains `.`, use explicit fields instead of `target`

#### 2. Raw action execution on a resolved device

Request body:

```json
{
  "device_id": "petkit:feeder:pet-parent",
  "action": "manual_feed_dual",
  "params": {
    "amount1": 20,
    "amount2": 20
  }
}
```

This mode bypasses AI command-name resolution and forwards the provided `action` to the owning plugin after policy/audit checks.

Response:

```json
{
  "device": {
    "id": "petkit:feeder:pet-parent",
    "name": "Kitchen Feeder"
  },
  "command": {
    "name": "Feed Once",
    "action": "feed_once",
    "target": "Kitchen Feeder.Feed Once",
    "params": {
      "portions": 2
    }
  },
  "decision": {
    "allowed": true,
    "risk_level": "low"
  },
  "result": {
    "accepted": true,
    "message": "command accepted"
  }
}
```

Resolution rules:

- `device_id` resolves a single device directly
- when `action` is omitted and `device_id` is absent:
- `device_name` or the left side of `target` can match either device `name` / `aliases` or a room name
- `command` or the right side of `target` resolves against command `name` plus `aliases`
- if no device or room qualifier is supplied, Celestia searches the command across all devices
- same-name collisions are allowed; Celestia returns HTTP `409` instead of guessing

Parameter handling:

- toggle commands require `on`
- number and select commands require their declared value parameter
- action commands only accept user parameters explicitly declared in control metadata
- parameter names are matched case-insensitively after punctuation normalization
- number parameters accept numeric strings such as `"2"`
- boolean parameters accept `true` / `false`, `on` / `off`, `yes` / `no`, and `1` / `0`

## Events And Audit API

### List Events

`GET /api/v1/events`

Optional query parameters:

- `plugin_id`
- `device_id`
- `limit` (default `100`)

Response:

```json
[
  {
    "id": "evt-1",
    "type": "device.state.changed",
    "plugin_id": "xiaomi",
    "device_id": "xiaomi:cn:123456",
    "ts": "2026-04-03T10:00:00Z",
    "payload": {
      "previous_state": {
        "power": false
      },
      "state": {
        "power": true
      },
      "changed_keys": ["power"]
    }
  }
]
```

For `device.state.changed`, Core enriches the payload before publishing it to SSE subscribers and persisting it:

- `payload.state` is the new snapshot
- `payload.previous_state` is the last persisted snapshot for that device
- `payload.changed_keys` lists the keys whose values changed

Core-generated automation execution events use:

- `automation.triggered`
- `automation.failed`

### Event Stream

`GET /api/v1/events/stream`

Server-Sent Events stream:

- `event:` is set to the Celestia event type
- `data:` contains the full JSON event payload
- a `: ping` keepalive comment is emitted every 15 seconds

Example:

```text
event: device.state.changed
data: {"id":"evt-1","type":"device.state.changed","device_id":"xiaomi:cn:123456","ts":"2026-04-03T10:00:00Z","payload":{"state":{"power":true}}}
```

### List Audits

`GET /api/v1/audits`

Optional query parameters:

- `device_id`
- `limit` (default `100`)

Response:

```json
[
  {
    "id": "audit-1",
    "actor": "admin",
    "device_id": "petkit:feeder:pet-parent",
    "action": "feed_once",
    "params": {
      "portions": 1
    },
    "result": "accepted",
    "risk_level": "low",
    "allowed": true,
    "created_at": "2026-04-03T10:00:00Z"
  }
]
```

## Xiaomi OAuth API

### Start Xiaomi OAuth

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

### Get OAuth Session

`GET /api/v1/oauth/xiaomi/sessions/{id}`

Response: the current `OAuthSession`.

### OAuth Callback

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

## Stream Signalling API

These endpoints stay under `/api/v1` and are used by the admin UI to initiate and manage WebRTC stream sessions for Hikvision cameras.

All three endpoints require the target device to have `"stream"` in its `capabilities` list and its owning plugin to be running.

Credentials (camera username, password, and any credential-bearing RTSP URL) are never included in responses.

### Start A Stream Session

`POST /api/v1/devices/{id}/stream/offer`

Request body:

```json
{
  "sdp": "<WebRTC SDP offer string>"
}
```

Response: HTTP `200`

```json
{
  "session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "sdp": "<WebRTC SDP answer string>"
}
```

The `session_id` is the stream session identifier to use with the close and ICE endpoints.

Error cases:

- `400` when `sdp` is missing or malformed
- `422` when the device does not support streaming
- `503` when the device's owning plugin is not running
- `502` when the plugin cannot provide a usable RTSP URL for relay setup

### Close A Stream Session

`DELETE /api/v1/devices/{id}/stream/{session_id}`

Response: HTTP `204 No Content`

Error cases:

- `422` when the device does not support streaming
- `503` when the device's owning plugin is not running

### Deliver A Trickle ICE Candidate

`POST /api/v1/devices/{id}/stream/ice`

Request body:

```json
{
  "session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "candidate": "candidate:1 1 UDP 2122252543 192.168.1.42 54321 typ host"
}
```

Response: HTTP `204 No Content`

### ICE Candidates From The Plugin

The plugin emits trickle ICE candidates on the existing SSE stream as `device.event.occurred` events.

Listen for payloads such as:

```json
{
  "event_type": "ice_candidate",
  "session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "candidate": "candidate:1 1 UDP 2122252543 10.0.0.5 56789 typ host"
}
```

Additional lifecycle events emitted by the plugin:

```json
{ "event_type": "stream_disconnected", "session_id": "<id>" }
{ "event_type": "stream_timeout", "session_id": "<id>" }
```
