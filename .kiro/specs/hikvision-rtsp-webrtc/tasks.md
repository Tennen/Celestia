# Implementation Plan: hikvision-rtsp-webrtc

## Overview

Implement live RTSP-to-WebRTC stream relay for Hikvision cameras. The plugin opens a real RTSP connection via gortsplib, bridges media into a pion/webrtc PeerConnection, and returns an SDP answer. Core exposes three HTTP signalling endpoints. The admin UI renders a live video element using native WebRTC APIs.

## Tasks

- [x] 1. Add Go module dependencies
  - Add `github.com/bluenviron/gortsplib/v4` and `github.com/pion/webrtc/v3` to `go.mod` and `go.sum` by running `go get` for each
  - Verify the module graph resolves without conflicts
  - _Requirements: 1.1, 2.1_

- [x] 2. Extend plugin config with stream fields
  - [x] 2.1 Add `MaxStreamSessions` and `StreamIdleTimeoutSeconds` to `CameraConfig` in `plugins/hikvision/internal/app/config.go`
    - Parse `max_stream_sessions` (default 4, min 1) and `stream_idle_timeout_seconds` (default 60, min 10) in `parseEntryConfig`
    - _Requirements: 9.1, 9.2, 9.3_
  - [x] 2.2 Write unit tests for config parsing of new stream fields
    - Extend `config_test.go` to cover default values, minimum enforcement, and explicit values for both new fields
    - _Requirements: 9.1, 9.2, 9.3_

- [x] 3. Implement RTSPRelay in `stream_relay.go`
  - [x] 3.1 Create `plugins/hikvision/internal/app/stream_relay.go` with `StreamSession` and `RTSPRelay` structs
    - Define `StreamSession` with `ID`, `EntryID`, `PC`, `RTSPClient`, `LastActivity`, `cancel`
    - Define `RTSPRelay` with `mu`, `sessions`, `maxSessions`, `idleTimeout`, `emitEvent`, `stopCleanup`
    - Implement `NewRTSPRelay(maxSessions int, idleTimeout time.Duration, emit func(models.Event)) *RTSPRelay`
    - _Requirements: 2.4, 8.1_
  - [x] 3.2 Implement `RTSPRelay.Offer`
    - Construct RTSP URL as `rtsp://<Username>:<Password>@<Host>:<RTSPPort><RTSPPath>` substituting `{channel}`; never expose credentials in return values
    - Open gortsplib client, DESCRIBE/SETUP/PLAY the RTSP stream
    - Create pion/webrtc `PeerConnection`, add video track (H.264/H.265) and audio track when present
    - Set remote description from SDP offer, produce SDP answer, assign unique `session_id`
    - Return error if RTSP unreachable, if answer not produced within 10 s, or if session limit exceeded
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 2.1, 2.2, 2.4, 2.7, 6.1, 6.2, 6.3_
  - [x] 3.3 Implement ICE candidate emission and trickle ICE wiring
    - Register `OnICECandidate` callback on the PeerConnection; emit `device.event.occurred` with `event_type: "ice_candidate"`, `session_id`, and candidate string
    - Register `OnICEConnectionStateChange` callback; on `failed` or `closed` state, close RTSP session and remove session from active set
    - _Requirements: 3.1, 8.4_
  - [x] 3.4 Implement `RTSPRelay.Close`, `RTSPRelay.AddICECandidate`, and `RTSPRelay.CloseAll`
    - `Close(sessionID)`: close PeerConnection and RTSP client for the session; return error if session not found
    - `AddICECandidate(sessionID, candidate)`: deliver candidate to PeerConnection; return error if session not found
    - `CloseAll()`: close every active session (called on plugin Stop)
    - _Requirements: 2.5, 2.6, 3.3, 5.4_
  - [x] 3.5 Implement `RTSPRelay.startCleanupLoop`
    - Tick at ≤15 s intervals; close sessions idle for longer than `idleTimeout`; emit `stream_timeout` event on cleanup
    - Emit `stream_disconnected` event when RTSP session drops after establishment
    - _Requirements: 8.2, 8.3, 1.5_
  - [x] 3.6 Write property test for session ID uniqueness
    - **Property 1: Every session_id returned by Offer is unique across concurrent calls**
    - **Validates: Requirements 2.4**
  - [x] 3.7 Write unit tests for RTSPRelay lifecycle
    - Test Close with unknown session_id returns error (req 2.6)
    - Test AddICECandidate with unknown session_id returns error (req 3.3)
    - Test CloseAll drains all sessions (req 5.4)
    - Test session limit rejection (req 2.7)
    - _Requirements: 2.5, 2.6, 2.7, 3.3, 5.4_

- [x] 4. Checkpoint — relay unit tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Implement stream command handlers in `stream_commands.go`
  - [x] 5.1 Create `plugins/hikvision/internal/app/stream_commands.go` with `handleStreamOffer`, `handleStreamClose`, `handleStreamICE`
    - `handleStreamOffer`: extract `sdp` param, call `p.relay.Offer`, return `{"session_id": ..., "sdp": ...}`
    - `handleStreamClose`: extract `session_id`, call `p.relay.Close`, return `{"closed": true}`
    - `handleStreamICE`: extract `session_id` and `candidate`, call `p.relay.AddICECandidate`, return `{"accepted": true}`
    - _Requirements: 5.1, 5.2, 5.3_
  - [x] 5.2 Write unit tests for stream command handlers
    - Test missing `sdp` param returns error
    - Test missing `session_id` param returns error for close and ice
    - _Requirements: 5.1, 5.2, 5.3_

- [x] 6. Wire relay into `plugin.go` and `commands.go`
  - [x] 6.1 Add `relay *RTSPRelay` field to `Plugin` struct in `plugin.go`
    - In `Setup`, construct `NewRTSPRelay` using `MaxStreamSessions` and `StreamIdleTimeoutSeconds` from the first entry config (or global config); store on `p.relay`
    - In `Stop`, call `p.relay.CloseAll()` before returning
    - _Requirements: 5.4, 9.1, 9.2_
  - [x] 6.2 Add `stream_offer`, `stream_close`, `stream_ice` cases to the switch in `commands.go`
    - Delegate to `p.handleStreamOffer`, `p.handleStreamClose`, `p.handleStreamICE` respectively
    - _Requirements: 5.1, 5.2, 5.3_

- [x] 7. Implement Core HTTP stream handlers in `internal/api/http/stream.go`
  - [x] 7.1 Create `internal/api/http/stream.go` with `handleStreamOffer`, `handleStreamClose`, `handleStreamICE`
    - `handleStreamOffer`: decode `{"sdp": "..."}`, check `"stream"` capability (→ 422) and plugin running (→ 503), call `SendDeviceCommand` with `stream_offer`, write 200 JSON response
    - `handleStreamClose`: check capability and plugin state, call `SendDeviceCommand` with `stream_close`, write 204
    - `handleStreamICE`: decode `{"session_id": "...", "candidate": "..."}`, check capability and plugin state, call `SendDeviceCommand` with `stream_ice`, write 204
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7_
  - [x] 7.2 Write unit tests for stream HTTP handlers
    - Test 422 when device lacks `"stream"` capability
    - Test 503 when plugin is not running
    - Test 200/204 happy paths with a stub gateway
    - _Requirements: 4.4, 4.5_

- [x] 8. Register stream routes in `server.go`
  - Add three route registrations to `New` in `internal/api/http/server.go`:
    ```
    POST   /api/v1/devices/{id}/stream/offer
    DELETE /api/v1/devices/{id}/stream/{session_id}
    POST   /api/v1/devices/{id}/stream/ice
    ```
  - _Requirements: 4.1, 4.2, 4.3_

- [x] 9. Checkpoint — backend compiles and stream routes respond
  - Ensure all tests pass, ask the user if questions arise.

- [x] 10. Implement `useStreamSession.ts` hook
  - [x] 10.1 Create `web/admin/src/hooks/useStreamSession.ts`
    - Create `RTCPeerConnection`, generate SDP offer, POST to `/api/v1/devices/{id}/stream/offer`
    - Set returned SDP answer as remote description
    - Send local ICE candidates via POST to `/api/v1/devices/{id}/stream/ice`
    - Listen for SSE `device.event.occurred` events with `event_type: "ice_candidate"` matching active `session_id`; call `addIceCandidate`
    - On stop or unmount, DELETE `/api/v1/devices/{id}/stream/{session_id}` and close the PeerConnection
    - Expose `session` state (`idle | connecting | active | error`), `startStream`, `stopStream`, `videoRef`
    - On offer error, close any partially-opened PeerConnection before setting error state
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 7.7, 7.8_
  - [x] 10.2 Write unit tests for useStreamSession hook
    - Test that error state is set and PeerConnection is closed when offer request fails
    - Test that stopStream sends DELETE and closes PeerConnection
    - _Requirements: 7.4, 7.7_

- [x] 11. Implement `StreamViewerPanel.tsx` component
  - [x] 11.1 Create `web/admin/src/components/admin/StreamViewerPanel.tsx`
    - Accept `deviceId: string` prop
    - Use `useStreamSession` hook
    - Render "Live View" button when `state === 'idle'`
    - Render `<video ref={videoRef} autoPlay playsInline>` and "Stop" button when `state === 'active'`
    - Render error message when `state === 'error'`
    - _Requirements: 7.1, 7.2, 7.3, 7.7_

- [x] 12. Wire `StreamViewerPanel` into `DeviceWorkspace.tsx`
  - Import `StreamViewerPanel` in `web/admin/src/components/admin/DeviceWorkspace.tsx`
  - Render `<StreamViewerPanel deviceId={deviceView.device.id} />` inside the device detail section when `deviceView.device.capabilities` includes `"stream"`
  - _Requirements: 7.1_

- [x] 13. Update API documentation
  - Add the three new endpoints to `docs/api.md` with request/response shapes, status codes, error cases (422, 503), and example payloads
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

- [x] 14. Final checkpoint — all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- The relay must never expose camera credentials in any return value, event payload, or device state field
- File size limit is 500 lines; split `stream_relay.go` if RTP forwarding logic grows large
- The `"stream"` capability must be added to the Hikvision plugin manifest's `Capabilities` slice so the HTTP handlers and UI gate correctly
