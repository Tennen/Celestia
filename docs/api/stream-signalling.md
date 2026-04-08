# Stream Signalling API

Back to the [API index](../api.md).

These endpoints stay under `/api/v1` and are used by the admin UI to initiate and manage WebRTC stream sessions for Hikvision cameras.

All three endpoints require the target device to have `"stream"` in its `capabilities` list and its owning plugin to be running.

Credentials (camera username, password, and any credential-bearing RTSP URL) are never included in responses.

## Start A Stream Session

`POST /api/v1/devices/{id}/stream/offer`

Request body:

```json
{
  "sdp": "<WebRTC SDP offer string>"
}
```

Response: HTTP `200`

```json
{
  "session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "sdp": "<WebRTC SDP answer string>"
}
```

The `session_id` is the stream session identifier to use with the close and ICE endpoints.

Error cases:

- `400` when `sdp` is missing or malformed
- `422` when the device does not support streaming
- `503` when the device's owning plugin is not running
- `502` when the plugin cannot provide a usable RTSP URL for relay setup

## Close A Stream Session

`DELETE /api/v1/devices/{id}/stream/{session_id}`

Response: HTTP `204 No Content`

Error cases:

- `422` when the device does not support streaming
- `503` when the device's owning plugin is not running

## Deliver A Trickle ICE Candidate

`POST /api/v1/devices/{id}/stream/ice`

Request body:

```json
{
  "session_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "candidate": "candidate:1 1 UDP 2122252543 192.168.1.42 54321 typ host"
}
```

Response: HTTP `204 No Content`

## ICE Candidates From The Plugin

The plugin emits trickle ICE candidates on the existing SSE stream as `device.event.occurred` events.

Listen for payloads such as:

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
{ "event_type": "stream_timeout", "session_id": "<id>" }
```
