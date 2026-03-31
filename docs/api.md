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
      "name": "Living Room Switch",
      "default_name": "Mi Smart Switch",
      "alias": "Living Room Switch"
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

### Update Device Alias

`PUT /api/v1/devices/{device_id}/preference`

Request body:

```json
{
  "alias": "Kitchen Feeder"
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

Behavior:

- `alias` sets a per-device display alias for that quick control
- `visible: false` hides the control from the main quick-control area
- `visible: true` shows it again
- sending `alias: ""` and `visible: true` resets the control back to default behavior

The updated preference is persisted in SQLite and reflected in subsequent `GET /api/v1/devices` and `GET /api/external/v1/devices` responses.


## Stream Signalling API

These endpoints stay under `/api/v1` and are used by the admin UI to initiate and manage WebRTC stream sessions for Hikvision cameras. All three endpoints require the target device to have `"stream"` in its `capabilities` list and its owning plugin to be running.

Credentials (camera username, password, and the credential-bearing RTSP URL) are never included in any response.

### Start a Stream Session

`POST /api/v1/devices/{id}/stream/offer`

Request body:

```json
{
  "sdp": "<WebRTC SDP offer string>"
}
```

Response — HTTP 200:

```json
{
  "session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "sdp": "<WebRTC SDP answer string>"
}
```

The `session_id` is a unique identifier for this stream session. Pass it to the close and ICE endpoints.

Error cases:

- **422 Unprocessable Entity** — device does not have `"stream"` in its capabilities:

```json
{
  "error": "device does not support streaming"
}
```

- **503 Service Unavailable** — the device's owning plugin is not running:

```json
{
  "error": "plugin is not running"
}
```

### Close a Stream Session

`DELETE /api/v1/devices/{id}/stream/{session_id}`

No request body.

Response — HTTP 204 No Content (empty body).

Error cases:

- **422 Unprocessable Entity** — device does not have `"stream"` in its capabilities.
- **503 Service Unavailable** — the device's owning plugin is not running.

### Deliver a Trickle ICE Candidate

`POST /api/v1/devices/{id}/stream/ice`

Request body:

```json
{
  "session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "candidate": "candidate:1 1 UDP 2122252543 192.168.1.42 54321 typ host"
}
```

Response — HTTP 204 No Content (empty body).

Error cases:

- **422 Unprocessable Entity** — device does not have `"stream"` in its capabilities.
- **503 Service Unavailable** — the device's owning plugin is not running.

### ICE Candidates from the Plugin

The plugin emits trickle ICE candidates as SSE events on the existing `device.event.occurred` stream. The admin UI should listen for events with `event_type: "ice_candidate"` matching the active `session_id` and call `RTCPeerConnection.addIceCandidate` with the candidate string.

Example SSE event payload:

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
{ "event_type": "stream_timeout",      "session_id": "<id>" }
```
