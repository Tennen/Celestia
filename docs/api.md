# Celestia API

Celestia exposes three HTTP surfaces:

- `/api/v1` for the admin UI and general runtime management
- `/api/external/v1` for stable device query/control
- `/api/ai/v1` for AI-agent-oriented device discovery and command invocation

All JSON endpoints return `Content-Type: application/json`.

## Shared Conventions

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

## Domain Index

- [Runtime And Plugin Management API](api/runtime-plugins.md): `/api/v1` health, dashboard, plugin catalog, install state, and plugin lifecycle operations
- [Capabilities Inventory API](api/capabilities.md): `/api/v1/capabilities` inventory and capability-specific entrypoints
- [Automation Capability API](api/automation.md): `automation` capability behavior and `/api/v1/automations` CRUD
- [Vision Stay Zone Capability API](api/vision-stay-zone.md): `vision_entity_stay_zone` capability detail, config sync, status, and event ingestion
- [Device Query, Control, and Preferences](api/devices.md): `/api/v1/devices`, `/api/external/v1/devices`, and admin-side device/control preferences
- [AI Agent API](api/ai.md): `/api/ai/v1` semantic device lookup and command execution
- [Events And Audit API](api/events-audit.md): event history, SSE stream, and audit history
- [Xiaomi OAuth API](api/xiaomi-oauth.md): Xiaomi OAuth session bootstrap and callback flow
- [Stream Signalling API](api/stream-signalling.md): WebRTC session setup for streaming-capable devices

## Notes

- `docs/api.md` remains the stable API entrypoint for repository links.
- Domain documents below this index are split by operational ownership and HTTP surface.
