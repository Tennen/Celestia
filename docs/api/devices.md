# Device Query, Control, And Preferences

Back to the [API index](../api.md).

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

Controls may also include:

- `disabled: true` when the control is intentionally exposed but not executable in the current runtime state
- `disabled_reason` with a human-readable explanation, for example when a Hikvision cloud camera has RTSP configured for viewing but Ezviz PTZ credentials or vendor permission are missing

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

If the referenced control is currently disabled, the endpoint returns an error instead of silently falling back to a raw device command.

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
