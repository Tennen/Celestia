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
    "service_ws_url": "ws://127.0.0.1:8090/ws/control",
    "model_name": "custom-pets.pt",
    "rule_count": 1,
    "enabled_rule_count": 1,
    "last_event_at": "2026-04-11T08:28:11Z",
    "last_synced_at": "2026-04-11T08:20:00Z"
  },
  "updated_at": "2026-04-11T08:28:11Z",
  "vision": {
    "config": {
      "service_ws_url": "ws://127.0.0.1:8090/ws/control",
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
          "behavior": "eating",
          "key_entities": [
            {
              "id": 101,
              "name": "Feeder Cat",
              "description": "orange tabby with a blue collar"
            },
            {
              "id": 102,
              "name": "Midnight",
              "image": {
                "base64": "...",
                "content_type": "image/jpeg"
              },
              "description": "solid black cat with a white chest"
            }
          ],
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
      "service_ws_url": "ws://127.0.0.1:8090/ws/control",
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
  "service_ws_url": "ws://127.0.0.1:8090/ws/control",
  "model_name": "custom-pets.pt"
}
```

- `service_ws_url` is optional when the capability already has a saved websocket endpoint.
- `model_name` is optional. When omitted, Gateway uses the saved configured model for the same endpoint when available; otherwise it asks the Vision Service for the current/default runtime model.
- Gateway fetches the entity catalog over the Vision Service websocket protocol. It does not use REST for catalog refresh.

Response:

```json
{
  "service_ws_url": "ws://127.0.0.1:8090/ws/control",
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
  "service_ws_url": "ws://127.0.0.1:8090/ws/control",
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
        "value": ""
      },
      "behavior": "eating",
      "key_entities": [
        {
          "id": 101,
          "name": "Feeder Cat",
          "description": "orange tabby with a blue collar"
        },
        {
          "id": 102,
          "name": "Midnight",
          "image": {
            "base64": "...",
            "content_type": "image/jpeg"
          },
          "description": "solid black cat with a white chest"
        }
      ],
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
- `entity_selector.value` is optional. When empty, Gateway persists the rule as an all-entities wildcard and syncs that empty selector to the Vision Service.
- `behavior` is optional. When present, Gateway persists it with the rule and syncs it to Vision Service so downstream semantic fallback checks can combine the target entity plus behavior.
- `key_entities` is optional. When present, Gateway persists the per-rule candidate set for post-event identity matching and only syncs the downstream-required fields.
- `key_entities[].name` is a Gateway/Admin-only display field. Gateway persists it for local history, filtering, and future statistics surfaces, but it does not send `name` to Vision Service.
- Each `key_entities[]` entry must provide a stable positive `id` plus at least one of `image` or `description`.
- `key_entities[].image.content_type` is optional. When omitted, Vision Service may default it to `image/jpeg`.
- If Gateway already has a fetched catalog for the same websocket endpoint and configured model, it validates each non-empty `entity_selector` against that catalog before accepting the config.

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
      "behavior": "eating",
      "key_entities": [
        {
          "id": 101,
          "description": "orange tabby with a blue collar"
        },
        {
          "id": 102,
          "image": {
            "base64": "...",
            "content_type": "image/jpeg"
          },
          "description": "solid black cat with a white chest"
        }
      ],
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

## List Rule Event History

`GET /api/v1/capabilities/vision_entity_stay_zone/rules/{ruleID}/events`

Optional query parameters:

- `limit` (default `50`)
- `from_ts` (inclusive RFC3339 lower bound)
- `to_ts` (exclusive RFC3339 upper bound)
- `before_ts` (RFC3339 cursor for older pages)
- `before_id` (recommended with `before_ts` to disambiguate equal timestamps)

Response:

```json
[
  {
    "id": "vision-recent",
    "type": "device.event.occurred",
    "plugin_id": "hikvision",
    "device_id": "hikvision:camera:entry-1",
    "ts": "2026-04-11T08:28:11Z",
    "payload": {
      "source": "capability:vision_entity_stay_zone",
      "capability_id": "vision_entity_stay_zone",
      "rule_id": "feeder-zone",
      "rule_name": "Feeder Zone",
      "event_status": "threshold_met",
      "dwell_seconds": 6,
      "entity_value": "cat",
      "key_entity_id": 101,
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
      ],
      "metadata": {
        "decision": {
          "source": "roi_vlm_fallback",
          "confidence_score": 0.91,
          "confidence_breakdown": {
            "detector": 0.52,
            "semantic": 0.96
          },
          "semantic_checker": {
            "verdict": "pass"
          }
        },
        "key_entity_match": {
          "winner_id": 101,
          "model_name": "qwen2.5-vl",
          "vote_counts": {
            "101": 2,
            "102": 1
          }
        }
      },
      "capture_count": 4,
      "captures": [
        {
          "capture_id": "vision-recent:start",
          "event_id": "vision-recent",
          "rule_id": "feeder-zone",
          "camera_device_id": "hikvision:camera:entry-1",
          "phase": "start",
          "captured_at": "2026-04-11T08:28:09Z",
          "content_type": "image/jpeg",
          "size_bytes": 48123,
          "metadata": {
            "annotations": {
              "image_kind": "raw",
              "coordinate_space": "normalized_xywh",
              "source": "ultralytics.boxes",
              "detections": [
                {
                  "kind": "label",
                  "value": "cat",
                  "display_name": "Cat",
                  "confidence": 0.93,
                  "track_id": "7",
                  "box": {
                    "x": 0.12,
                    "y": 0.24,
                    "width": 0.31,
                    "height": 0.42
                  }
                }
              ]
            }
          }
        },
        {
          "capture_id": "vision-recent:middle",
          "event_id": "vision-recent",
          "rule_id": "feeder-zone",
          "camera_device_id": "hikvision:camera:entry-1",
          "phase": "middle",
          "captured_at": "2026-04-11T08:28:15Z",
          "content_type": "image/jpeg",
          "size_bytes": 48712
        },
        {
          "capture_id": "vision-recent:end",
          "event_id": "vision-recent",
          "rule_id": "feeder-zone",
          "camera_device_id": "hikvision:camera:entry-1",
          "phase": "end",
          "captured_at": "2026-04-11T08:28:19Z",
          "content_type": "image/jpeg",
          "size_bytes": 48984
        },
        {
          "capture_id": "vision-recent:sample_002",
          "event_id": "vision-recent",
          "rule_id": "feeder-zone",
          "camera_device_id": "hikvision:camera:entry-1",
          "phase": "sample_002",
          "captured_at": "2026-04-11T08:28:21Z",
          "content_type": "image/jpeg",
          "size_bytes": 49201
        }
      ]
    }
  }
]
```

Rule history is still bounded by the configured `event_capture_retention_hours`, but callers can page within that retained window and narrow reads to a specific date range.

Important behavior:

- Gateway returns persisted `device.event.occurred` records for the requested rule only.
- Results are ordered newest-first.
- History is limited to the configured `event_capture_retention_hours` window so rule history and evidence expiration stay aligned in Admin.
- Current Vision Service behavior emits one completed `threshold_met` event per threshold-qualified stay. `dwell_seconds` is the full event duration, and Gateway no longer expects a follow-up `cleared` event.
- `payload.entities`, when present, contains the full set of recognized in-zone entities reported by the Vision Service for that event. `payload.entity_value` remains the backward-compatible primary entity field.
- `payload.key_entity_id`, when present, echoes the winning `rules[].key_entities[].id` chosen by downstream identity matching for that event.
- `payload.metadata.decision`, when present, contains Vision Service decision metadata such as source, confidence scoring, confidence breakdown, and semantic checker verdicts used by Admin for decision inspection.
- `payload.metadata.key_entity_match`, when present, contains downstream key-entity vote details such as frame matches, vote counts, winner, model name, and failure or skipped reasons.
- `payload.captures[].metadata.annotations`, when present, contains normalized detection boxes for that capture. If `image_kind` is `raw`, Admin overlays those boxes on top of the returned image. If `image_kind` is `annotated`, Admin treats the stored image bytes as already rendered.
- `payload.captures[]` may contain more than three images. `phase` is a non-empty label; Gateway orders `start` first, `end` last, and intermediate labels such as legacy `middle` or numbered samples like `sample_002` by time and `capture_id`.
- If stored evidence exists for a returned event, Gateway enriches the event payload with `capture_count` and `captures`.

## Delete Rule Event History Item

`DELETE /api/v1/capabilities/vision_entity_stay_zone/rules/{ruleID}/events/{eventID}`

Response:

```json
{
  "ok": true
}
```

Important behavior:

- Gateway only deletes persisted `device.event.occurred` records that belong to the specified rule.
- Gateway deletes any stored evidence captures linked to that event in the same operation.
- If `ruleID` does not exist, or `eventID` does not belong to that rule-scoped persisted vision event, Gateway returns `404`.
- This endpoint does not delete the separate projected `device.state.changed` record that may have been emitted for the same observation.

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
