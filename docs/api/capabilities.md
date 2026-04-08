# Capabilities Inventory API

Back to the [API index](../api.md).

These routes stay under `/api/v1` and expose the Core-owned capability inventory.

Current built-in capability ids:

- `automation`
- `vision_entity_stay_zone`

Capability-specific payloads and mutation routes live in their own documents:

- [Automation Capability API](automation.md)
- [Vision Stay Zone Capability API](vision-stay-zone.md)

## List Capabilities

`GET /api/v1/capabilities`

Response:

```json
[
  {
    "id": "automation",
    "kind": "automation",
    "name": "Automations",
    "description": "Core-owned state-change automations that execute device actions.",
    "enabled": true,
    "status": "healthy",
    "summary": {
      "total": 3,
      "enabled_count": 2,
      "last_triggered_at": "2026-04-08T09:30:00Z"
    },
    "updated_at": "2026-04-08T09:30:00Z"
  },
  {
    "id": "vision_entity_stay_zone",
    "kind": "vision_entity_stay_zone",
    "name": "Vision Stay Zone Recognition",
    "description": "Gateway-managed stay-zone control plane for independent vision processing services.",
    "enabled": true,
    "status": "healthy",
    "summary": {
      "service_url": "http://127.0.0.1:8090",
      "rule_count": 2,
      "enabled_rule_count": 2,
      "last_event_at": "2026-04-08T09:28:11Z",
      "last_synced_at": "2026-04-08T09:20:00Z"
    },
    "updated_at": "2026-04-08T09:28:11Z"
  }
]
```

## Get Capability Detail

`GET /api/v1/capabilities/{capability_id}`

The response shape is capability-specific. See:

- [Automation Capability API](automation.md) for the `automation` capability and its CRUD surface
- [Vision Stay Zone Capability API](vision-stay-zone.md) for the `vision_entity_stay_zone` detail payload and mutation routes
