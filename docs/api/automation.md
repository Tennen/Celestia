# Automation Capability API

Back to the [API index](../api.md).

The automation capability is exposed in the capability inventory under id `automation`.

These routes stay under `/api/v1` and let the admin UI manage Core-owned state-change automations.

Each automation has:

- one or more `conditions`
- exactly one `type: "state_changed"` condition; that single state-change condition starts the automation
- zero or more `type: "current_state"` conditions combined by `condition_logic` as extra runtime gates
- an optional daily time window
- one or more actions executed against existing devices

Supported match operators:

- `any` for `type: "state_changed"` condition `from`
- `equals`
- `not_equals`
- `in`
- `not_in`
- `exists`
- `missing`

Condition shapes:

- `type: "state_changed"` uses `from` and `to`
- `type: "current_state"` uses `match` against the latest persisted device state

Requests with zero or multiple `state_changed` conditions are rejected.

For `in` and `not_in`, `value` must be a JSON array. This allows one rule to match transitions like `D -> A/B/C` on the same state key.

`time_window.start` and `time_window.end` use `HH:MM` in the gateway's local timezone. Ranges that cross midnight are supported.

## List Automations

`GET /api/v1/automations`

Response:

```json
[
  {
    "id": "automation-1",
    "name": "Washer Done Voice Push",
    "enabled": true,
    "condition_logic": "all",
    "conditions": [
      {
        "type": "state_changed",
        "device_id": "haier:washer:home:washer-1",
        "state_key": "phase",
        "from": {
          "operator": "not_equals",
          "value": "ready"
        },
        "to": {
          "operator": "in",
          "value": ["ready", "dry_done", "wash_done"]
        }
      },
      {
        "type": "current_state",
        "device_id": "haier:washer:home:washer-1",
        "state_key": "machine_status",
        "match": {
          "operator": "equals",
          "value": "idle"
        }
      }
    ],
    "time_window": {
      "start": "08:00",
      "end": "23:00"
    },
    "actions": [
      {
        "device_id": "xiaomi:cn:speaker-1",
        "label": "Suggested · Voice push",
        "action": "push_voice_message",
        "params": {
          "message": "洗衣机已结束",
          "volume": 55
        }
      }
    ],
    "last_triggered_at": "2026-04-03T10:20:00Z",
    "last_run_status": "succeeded",
    "last_error": "",
    "created_at": "2026-04-03T09:50:00Z",
    "updated_at": "2026-04-03T10:20:00Z"
  }
]
```

## Create Automation

`POST /api/v1/automations`

Request body: the automation payload without a required `id`. Core assigns one when missing.

Response: HTTP `200` with the persisted `Automation`.

## Update Automation

`PUT /api/v1/automations/{automation_id}`

Request body: the automation payload. The path `automation_id` wins over any body `id`.

Response: HTTP `200` with the persisted `Automation`.

## Delete Automation

`DELETE /api/v1/automations/{automation_id}`

Response:

```json
{
  "ok": true
}
```
