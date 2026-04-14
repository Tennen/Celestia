# Events And Audit API

Back to the [API index](../api.md).

## List Events

`GET /api/v1/events`

Optional query parameters:

- `plugin_id`
- `device_id`
- `limit` (default `100`)
- `from_ts` (inclusive RFC3339 lower bound)
- `to_ts` (exclusive RFC3339 upper bound)
- `before_ts` (RFC3339 cursor for older pages)
- `before_id` (recommended with `before_ts` to disambiguate equal timestamps)

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

This endpoint exposes the global persisted Core event feed. Vision capability `event_capture_retention_hours` does not trim this list; that setting only applies to rule-scoped vision history and capture evidence.

For cursor-based pagination, request the next older page with the last event from the current page:

```text
GET /api/v1/events?limit=100&before_ts=2026-04-03T10:00:00Z&before_id=evt-1
```

For `device.state.changed`, Core enriches the payload before publishing it to SSE subscribers and persisting it:

- `payload.state` is the new snapshot
- `payload.previous_state` is the last persisted snapshot for that device
- `payload.changed_keys` lists the keys whose values changed

Device inventory lifecycle events use:

- `device.discovered`
- `device.updated`

When a plugin emits either inventory event with `payload.device`, Core updates the persisted device registry before the event is published. `device.updated` is the path used for runtime changes to device metadata such as `online`, name, or capability metadata that are not represented inside `device.state.changed`.

Core-generated automation execution events use:

- `automation.triggered`
- `automation.failed`

Capability runtime health changes use:

- `capability.status.changed`

Vision detection reports arrive as `device.event.occurred` with `payload.capability_id = "vision_entity_stay_zone"` and are also projected into the camera's `device.state.changed` stream.

For rule-scoped persisted vision history in Admin, use the Vision Stay Zone capability endpoints documented in [vision-stay-zone.md](vision-stay-zone.md):

- `GET /api/v1/capabilities/vision_entity_stay_zone/rules/{ruleID}/events`
- `DELETE /api/v1/capabilities/vision_entity_stay_zone/rules/{ruleID}/events/{eventID}`

If Gateway has persisted screenshot evidence for a vision event, that same event record is enriched on read with:

- `payload.entities`
- `payload.metadata.decision`
- `payload.capture_count`
- `payload.captures`

`payload.entities` is the Vision Service-reported set of recognized in-zone entities for that event. `payload.entity_value` remains the backward-compatible primary entity field.
`payload.metadata.decision`, when present, carries Vision Service decision details such as the inference source, confidence score, confidence breakdown, and semantic checker verdicts.

Each `payload.captures` item includes:

- `capture_id`
- `event_id`
- `rule_id`
- `camera_device_id`
- `phase`
- `captured_at`
- `content_type`
- `size_bytes`
- optional `metadata.annotations`

`metadata.annotations` uses normalized `box.{x,y,width,height}` coordinates. `image_kind=raw` means clients may render overlays from that box list; `image_kind=annotated` means the image bytes were already rendered by Vision Service.

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
