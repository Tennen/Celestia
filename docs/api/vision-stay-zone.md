# Vision Stay Zone Capability API

Back to the [API index](../api.md).

The vision stay-zone capability is exposed in the capability inventory under id `vision_entity_stay_zone`.

If you are implementing the downstream Vision Service itself, use the dedicated [Vision Service Integration Contract](vision-service-contract.md). This document remains the Gateway-side capability API reference.

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
    "service_url": "http://127.0.0.1:8090",
    "rule_count": 1,
    "enabled_rule_count": 1
  },
  "updated_at": "2026-04-08T09:20:00Z",
  "vision": {
    "config": {
      "service_url": "http://127.0.0.1:8090",
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
      "updated_at": "2026-04-08T09:20:00Z"
    },
    "runtime": {
      "status": "healthy",
      "message": "vision config synced",
      "service_version": "1.2.0",
      "last_synced_at": "2026-04-08T09:20:00Z",
      "last_reported_at": "2026-04-08T09:25:00Z",
      "last_event_at": "2026-04-08T09:28:11Z",
      "runtime": {
        "active_streams": 1
      },
      "sync_error": "",
      "updated_at": "2026-04-08T09:28:11Z"
    },
    "catalog": {
      "service_url": "http://127.0.0.1:8090",
      "schema_version": "celestia.vision.catalog.v1",
      "service_version": "1.2.0",
      "model_name": "yolo11m-coco",
      "fetched_at": "2026-04-08T09:18:00Z",
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
  "service_url": "http://127.0.0.1:8090"
}
```

`service_url` is optional when the capability already has a saved Vision Service address. Gateway uses this route to fetch the current model-supported recognizable entity list before rules are configured.

Response:

```json
{
  "service_url": "http://127.0.0.1:8090",
  "schema_version": "celestia.vision.catalog.v1",
  "service_version": "1.2.0",
  "model_name": "yolo11m-coco",
  "fetched_at": "2026-04-08T09:18:00Z",
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
  "service_url": "http://127.0.0.1:8090",
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

Gateway is the source of truth for this config. It persists the config first, then attempts to push a normalized copy to the external Vision Service at:

- `PUT {service_url}/api/v1/capabilities/vision_entity_stay_zone`

If Gateway already has a fetched entity catalog for the same `service_url`, it validates each `entity_selector` against that catalog before accepting the config. This lets the admin flow follow the intended sequence:

1. refresh current recognizable entities from the Vision Service
2. choose `cat` or another advertised entity
3. bind camera, RTSP source, zone, and stay threshold
4. save and sync the full rule set downstream

`event_capture_retention_hours` is Gateway-owned persistence policy for screenshots uploaded later by the Vision Service. It is not used by the Vision engine itself; Gateway applies it when storing and serving evidence images.

The pushed payload is a stable control-plane structure:

```json
{
  "schema_version": "celestia.vision.control.v1",
  "sent_at": "2026-04-08T09:20:00Z",
  "recognition_enabled": true,
  "callbacks": {
    "status_path": "/api/v1/capabilities/vision_entity_stay_zone/status",
    "event_path": "/api/v1/capabilities/vision_entity_stay_zone/events",
    "evidence_path": "/api/v1/capabilities/vision_entity_stay_zone/evidence"
  },
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

If the Vision Service is unreachable, Gateway still persists the config and returns a degraded `runtime.status` plus `runtime.sync_error`.

## Report Vision Service Status

`POST /api/v1/capabilities/vision_entity_stay_zone/status`

This endpoint is intended for the external Vision Service to report runtime health:

```json
{
  "status": "healthy",
  "message": "tracking 1 stream",
  "service_version": "1.2.0",
  "reported_at": "2026-04-08T09:25:00Z",
  "runtime": {
    "active_streams": 1
  }
}
```

Response: HTTP `200` with the persisted `VisionCapabilityStatus`.

## Report Vision Events

`POST /api/v1/capabilities/vision_entity_stay_zone/events`

Request body:

```json
{
  "events": [
    {
      "event_id": "vision-evt-1",
      "rule_id": "feeder-zone",
      "camera_device_id": "hikvision:camera:entry-1",
      "status": "threshold_met",
      "observed_at": "2026-04-08T09:28:11Z",
      "dwell_seconds": 6,
      "entity_value": "cat",
      "metadata": {
        "track_id": "trk-7"
      }
    }
  ]
}
```

Supported `status` values:

- `threshold_met`
- `cleared`

If the Vision Service will later upload screenshots for this event, it should set a stable `event_id`.

On each reported event, Gateway does two things:

1. Appends a structured `device.event.occurred` event with `payload.capability_id = "vision_entity_stay_zone"`.
2. Projects the event into the owning camera's persisted device state and emits a `device.state.changed` event so existing automations can react without any Vision-engine coupling.

For each vision rule, Gateway maintains these projected camera state keys:

- `vision_rule_<rule_id>_match_count`
- `vision_rule_<rule_id>_active`
- `vision_rule_<rule_id>_last_event_at`
- `vision_rule_<rule_id>_last_entity_value`
- `vision_rule_<rule_id>_last_dwell_seconds`
- `vision_rule_<rule_id>_last_status`

`threshold_met` increments `vision_rule_<rule_id>_match_count`, which lets existing state-change automations trigger on recurring detections.

## Report Vision Event Evidence

`POST /api/v1/capabilities/vision_entity_stay_zone/evidence`

This endpoint is intended for the external Vision Service to persist screenshots tied to a previously accepted vision event.

Request body:

```json
{
  "captures": [
    {
      "capture_id": "vision-evt-1:start",
      "event_id": "vision-evt-1",
      "rule_id": "feeder-zone",
      "camera_device_id": "hikvision:camera:entry-1",
      "phase": "start",
      "captured_at": "2026-04-08T09:28:05Z",
      "content_type": "image/jpeg",
      "image_base64": "/9j/4AAQSk..."
    },
    {
      "capture_id": "vision-evt-1:middle",
      "event_id": "vision-evt-1",
      "rule_id": "feeder-zone",
      "camera_device_id": "hikvision:camera:entry-1",
      "phase": "middle",
      "captured_at": "2026-04-08T09:28:08Z",
      "content_type": "image/jpeg",
      "image_base64": "/9j/4AAQSk..."
    },
    {
      "capture_id": "vision-evt-1:end",
      "event_id": "vision-evt-1",
      "rule_id": "feeder-zone",
      "camera_device_id": "hikvision:camera:entry-1",
      "phase": "end",
      "captured_at": "2026-04-08T09:28:11Z",
      "content_type": "image/jpeg",
      "image_base64": "/9j/4AAQSk..."
    }
  ]
}
```

Supported `phase` values:

- `start`
- `middle`
- `end`

Response: HTTP `200` with `{ "ok": true }`.

Gateway stores the image bytes in Core-owned persistence, applies the configured retention window, and exposes the stored screenshot metadata back on the matching event record as `payload.captures`.

## Get Vision Event Capture

`GET /api/v1/capabilities/vision_entity_stay_zone/captures/{captureID}`

Response: raw image bytes with the persisted image `Content-Type`.

Admin uses this route to render screenshots attached to specific vision events.
