# Vision Stay Zone Capability API

Back to the [API index](../api.md).

The vision stay-zone capability is exposed in the capability inventory under id `vision_entity_stay_zone`.

If you are implementing the downstream Vision Service itself, use the dedicated [Vision Service Integration Contract](vision-service-contract.md). This document is the Gateway-side capability API reference.

## Get Capability Detail

`GET /api/v1/capabilities/vision_entity_stay_zone`

Response:

```json
{
  "id": "vision_entity_stay_zone",
  "kind": "vision_entity_stay_zone",
  "name": "Vision Stay Zone Recognition",
  "description": "Gateway-managed stay-zone control plane for independent vision processing services.",
  "enabled": true,
  "status": "healthy",
  "summary": {
    "service_ws_url": "ws://127.0.0.1:8090/api/v1/capabilities/vision_entity_stay_zone",
    "model_name": "custom-pets.pt",
    "rule_count": 1,
    "enabled_rule_count": 1,
    "last_event_at": "2026-04-11T08:28:11Z",
    "last_synced_at": "2026-04-11T08:20:00Z"
  },
  "updated_at": "2026-04-11T08:28:11Z",
  "vision": {
    "config": {
      "service_ws_url": "ws://127.0.0.1:8090/api/v1/capabilities/vision_entity_stay_zone",
      "model_name": "custom-pets.pt",
      "recognition_enabled": true,
      "event_capture_retention_hours": 168,
      "rules": [
        {
          "id": "feeder-zone",
          "name": "Feeder Zone",
          "enabled": true,
          "camera_device_id": "hikvision:camera:entry-1",
          "recognition_enabled": true,
          "rtsp_source": {
            "url": "rtsp://user:pass@camera/stream"
          },
          "entity_selector": {
            "kind": "label",
            "value": "cat"
          },
          "zone": {
            "x": 0.12,
            "y": 0.28,
            "width": 0.34,
            "height": 0.22
          },
          "stay_threshold_seconds": 5
        }
      ],
      "updated_at": "2026-04-11T08:20:00Z"
    },
    "runtime": {
      "status": "healthy",
      "message": "tracking 1 stream(s) across 1 rule(s)",
      "service_version": "0.1.0",
      "last_synced_at": "2026-04-11T08:20:00Z",
      "last_reported_at": "2026-04-11T08:25:00Z",
      "last_event_at": "2026-04-11T08:28:11Z",
      "runtime": {
        "active_streams": 1
      },
      "sync_error": "",
      "updated_at": "2026-04-11T08:28:11Z"
    },
    "catalog": {
      "service_ws_url": "ws://127.0.0.1:8090/api/v1/capabilities/vision_entity_stay_zone",
      "schema_version": "celestia.vision.catalog.v1",
      "service_version": "0.1.0",
      "model_name": "custom-pets.pt",
      "fetched_at": "2026-04-11T08:18:00Z",
      "entities": [
        {
          "kind": "label",
          "value": "cat",
          "display_name": "Cat"
        }
      ]
    },
    "recent_events": []
  }
}
```

## Refresh Vision Entity Catalog

`POST /api/v1/capabilities/vision_entity_stay_zone/entities/refresh`

Request body:

```json
{
  "service_ws_url": "ws://127.0.0.1:8090/api/v1/capabilities/vision_entity_stay_zone",
  "model_name": "custom-pets.pt"
}
```

- `service_ws_url` is optional when the capability already has a saved websocket endpoint.
- `model_name` is optional. When omitted, Gateway uses the saved configured model for the same endpoint when available; otherwise it asks the Vision Service for the current/default runtime model.
- Gateway fetches the entity catalog over the Vision Service websocket protocol. It does not use REST for catalog refresh.

Response:

```json
{
  "service_ws_url": "ws://127.0.0.1:8090/api/v1/capabilities/vision_entity_stay_zone",
  "schema_version": "celestia.vision.catalog.v1",
  "service_version": "0.1.0",
  "model_name": "custom-pets.pt",
  "fetched_at": "2026-04-11T08:18:00Z",
  "entities": [
    {
      "kind": "label",
      "value": "cat",
      "display_name": "Cat"
    },
    {
      "kind": "label",
      "value": "dog",
      "display_name": "Dog"
    }
  ]
}
```

## Save Vision Capability Config

`PUT /api/v1/capabilities/vision_entity_stay_zone`

Request body:

```json
{
  "service_ws_url": "ws://127.0.0.1:8090/api/v1/capabilities/vision_entity_stay_zone",
  "model_name": "custom-pets.pt",
  "recognition_enabled": true,
  "event_capture_retention_hours": 168,
  "rules": [
    {
      "id": "feeder-zone",
      "name": "Feeder Zone",
      "enabled": true,
      "camera_device_id": "hikvision:camera:entry-1",
      "recognition_enabled": true,
      "rtsp_source": {
        "url": "rtsp://user:pass@camera/stream"
      },
      "entity_selector": {
        "kind": "label",
        "value": "cat"
      },
      "zone": {
        "x": 0.12,
        "y": 0.28,
        "width": 0.34,
        "height": 0.22
      },
      "stay_threshold_seconds": 5
    }
  ]
}
```

Response: HTTP `200` with the persisted `CapabilityDetail` for `vision_entity_stay_zone`.

Important behavior:

- `service_ws_url` must be a full `ws://` or `wss://` endpoint including the websocket path.
- `model_name` is optional. When present, Gateway sends `select_model` before `sync_config` and re-applies that selection after reconnects.
- Gateway persists the config first, then syncs the full desired state to the Vision Service over the websocket session.
- Gateway starts and maintains that websocket session during runtime init, so recognition does not depend on Admin interaction to establish the connection.
- When the selected camera already exposes an `rtsp_url` in device state or metadata, clients may omit `rtsp_source.url`. Gateway resolves and persists it before sync.
- If the camera does not expose RTSP and the rule is enabled for recognition, save is rejected explicitly.
- If Gateway already has a fetched catalog for the same websocket endpoint and configured model, it validates each `entity_selector` against that catalog before accepting the config.

The synced websocket control payload is:

```json
{
  "schema_version": "celestia.vision.control.ws.v1",
  "sent_at": "2026-04-11T08:20:00Z",
  "recognition_enabled": true,
  "rules": [
    {
      "id": "feeder-zone",
      "name": "Feeder Zone",
      "enabled": true,
      "camera": {
        "device_id": "hikvision:camera:entry-1",
        "plugin_id": "hikvision",
        "vendor_device_id": "192.0.2.10:8000:ch1",
        "name": "Patio Camera",
        "entry_id": "entry-1"
      },
      "rtsp_source": {
        "url": "rtsp://user:pass@camera/stream"
      },
      "entity_selector": {
        "kind": "label",
        "value": "cat"
      },
      "zone": {
        "x": 0.12,
        "y": 0.28,
        "width": 0.34,
        "height": 0.22
      },
      "stay_threshold_seconds": 5
    }
  ]
}
```

If the Vision Service is unreachable, Gateway still persists the config and returns a degraded `runtime.status` plus `runtime.sync_error`. The background runtime keeps retrying the websocket connection and re-sends model selection plus `sync_config` after reconnect.

## Vision Service Event Ingestion

Gateway no longer exposes REST endpoints for Vision Service status, event, or evidence callbacks.

The Vision Service must use the websocket protocol in [vision-service-contract.md](vision-service-contract.md) to deliver:

- `runtime_status`
- `rule_events`
- `evidence`

Gateway consumes those websocket messages, projects state changes into device state, appends Core events, and persists evidence images.

## Get Evidence Capture

`GET /api/v1/capabilities/vision_entity_stay_zone/captures/{captureID}`

Response: the raw capture bytes with `Content-Type` set from the stored evidence asset.

Gateway expires stored captures according to `event_capture_retention_hours`.
