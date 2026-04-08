# Events And Audit API

Back to the [API index](../api.md).

## List Events

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

Capability runtime health changes use:

- `capability.status.changed`

Vision detection reports arrive as `device.event.occurred` with `payload.capability_id = "vision_entity_stay_zone"` and are also projected into the camera's `device.state.changed` stream.

If Gateway has persisted screenshot evidence for a vision event, that same event record is enriched on read with:

- `payload.capture_count`
- `payload.captures`

Each `payload.captures` item includes:

- `capture_id`
- `event_id`
- `rule_id`
- `camera_device_id`
- `phase`
- `captured_at`
- `content_type`
- `size_bytes`

## Event Stream

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

## List Audits

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
