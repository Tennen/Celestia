# Celestia API

## External Device API

Celestia exposes a stable external query/control surface under `/api/external/v1`.

### List Devices

`GET /api/external/v1/devices`

Optional query parameters:

- `plugin_id`
- `kind`
- `q`

Response shape:

```json
[
  {
    "device": {
      "id": "xiaomi:cn:123456",
      "plugin_id": "xiaomi",
      "kind": "switch",
      "name": "Living Room Switch"
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

`controls[].kind` supports:

- `toggle` with boolean `state`
- `action` with a single-click execution flow
- `select` with `value`, `options`, and `command`
- `number` with `value`, optional `min` / `max` / `step`, and `command`

For `select` and `number` controls, clients should send the selected value through the generic device command endpoint using the embedded `command.action` and `command.value_param`.

### Get One Device

`GET /api/external/v1/devices/{device_id}`

Returns the same shape as the list endpoint for a single device.

`controls` is always present. Devices without quick controls return `"controls": []`.

### Toggle Control

`POST /api/external/v1/toggle/{device_id.control_id}/on`

`POST /api/external/v1/toggle/{device_id.control_id}/off`

Optional header:

- `X-Actor: your-client-name`

### Run Action Control

`POST /api/external/v1/action/{device_id.control_id}`

Optional header:

- `X-Actor: your-client-name`

### Send Advanced Command

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

Notes:

- For Petkit feeders, `feed_once` is a Celestia normalized command that maps to the upstream Petkit manual-feed API. It is not an upstream Petkit event name.
- Additional Petkit feeder actions accepted by the same endpoint include `manual_feed_dual`, `cancel_manual_feed`, `reset_desiccant`, `food_replenished`, `play_sound`, and `call_pet` when the selected feeder model supports them.
- Example dual-hopper manual feed request:

```json
{
  "action": "manual_feed_dual",
  "params": {
    "amount1": 20,
    "amount2": 20
  }
}
```

## Admin Control Preference API

These endpoints stay under `/api/v1` and are used by the admin UI to customize quick controls per device.

### Update Control Alias / Visibility

`PUT /api/v1/devices/{device_id}/controls/{control_id}`

Request body:

```json
{
  "alias": "Left Switch",
  "visible": false
}
```

Behavior:

- `alias` sets a per-device display alias for that quick control
- `visible: false` hides the control from the main quick-control area
- `visible: true` shows it again
- sending `alias: ""` and `visible: true` resets the control back to default behavior

The updated preference is persisted in SQLite and reflected in subsequent `GET /api/v1/devices` and `GET /api/external/v1/devices` responses.
