# Requirements Document

## Introduction

This feature adds live RTSP stream viewing for Hikvision cameras in the Celestia admin UI. The Hikvision plugin already connects to cameras over the HCNet SDK and exposes an RTSP URL in device state. This feature extends that path end-to-end: the plugin relays the real RTSP stream to a WebRTC peer using gortsplib and pion/webrtc, the core gateway exposes an HTTP signalling endpoint, and the admin UI renders the live video feed using the browser's native WebRTC APIs.

No mock streams, synthetic video, or test patterns are permitted at any layer. Every stream must originate from a real Hikvision camera reachable by the plugin process.

## Glossary

- **RTSP_Relay**: The component inside the Hikvision plugin process that connects to a camera's RTSP endpoint using gortsplib, transcodes or forwards the media tracks, and bridges them into a WebRTC peer connection via pion/webrtc.
- **WebRTC_Signalling_Handler**: The HTTP handler in `internal/api/http` that accepts an SDP offer from the admin UI, forwards it to the owning plugin via the gateway command path, and returns the SDP answer.
- **Plugin**: The Hikvision plugin process at `plugins/hikvision`.
- **Core**: The gateway process rooted at `cmd/gateway`, owning the HTTP API, device registry, state store, policy, and audit.
- **Admin_UI**: The Vite + React admin surface at `web/admin`, which reads and writes only through the gateway HTTP API.
- **SDP**: Session Description Protocol payload used in WebRTC offer/answer negotiation.
- **ICE**: Interactive Connectivity Establishment; the mechanism WebRTC uses to discover and negotiate network paths between peers.
- **RTSP_URL**: The `rtsp://` address stored in device state under the `rtsp_url` key, constructed from `CameraConfig.Host`, `CameraConfig.RTSPPort`, `CameraConfig.RTSPPath`, and camera credentials.
- **Stream_Session**: A single active WebRTC peer connection bridging one camera's RTSP feed to one browser tab. Identified by a `session_id` string.
- **Trickle_ICE**: The incremental exchange of ICE candidates after the initial SDP offer/answer, allowing connection establishment before all candidates are gathered.

---

## Requirements

### Requirement 1: RTSP Connection from Plugin

**User Story:** As a Celestia operator, I want the Hikvision plugin to connect to a camera's real RTSP stream, so that live video data is available for relay to the browser.

#### Acceptance Criteria

1. WHEN the RTSP_Relay receives a `stream_offer` command for a device, THE RTSP_Relay SHALL open a TCP or UDP connection to the camera's RTSP_URL using gortsplib.
2. IF the RTSP_URL is unreachable or the camera rejects the RTSP session, THEN THE RTSP_Relay SHALL return an error with the camera host, port, and the underlying transport error message.
3. WHILE an RTSP session is open, THE RTSP_Relay SHALL read H.264 or H.265 video RTP packets and, when present, AAC or G.711 audio RTP packets from the camera.
4. THE RTSP_Relay SHALL authenticate to the camera using the `Username` and `Password` from the matching `CameraConfig` entry; it SHALL NOT use anonymous or default credentials.
5. IF the RTSP session drops after establishment, THEN THE RTSP_Relay SHALL close the associated Stream_Session and emit a `device.event.occurred` event with `event_type: "stream_disconnected"` and the `session_id`.

### Requirement 2: WebRTC Peer Connection in Plugin

**User Story:** As a Celestia operator, I want the plugin to bridge the RTSP feed into a WebRTC peer connection, so that the browser can receive live video without requiring a separate media server.

#### Acceptance Criteria

1. WHEN the RTSP_Relay receives a valid SDP offer, THE RTSP_Relay SHALL create a pion/webrtc `PeerConnection`, add the video track (and audio track when present), and produce an SDP answer.
2. THE RTSP_Relay SHALL return the SDP answer within 10 seconds of receiving the SDP offer; IF it cannot, THEN THE RTSP_Relay SHALL return an error and release all allocated resources.
3. WHILE a Stream_Session is active, THE RTSP_Relay SHALL forward RTP packets from the RTSP connection into the corresponding pion/webrtc track without buffering more than 500ms of media.
4. THE RTSP_Relay SHALL assign each Stream_Session a unique `session_id` string and include it in the SDP answer response payload.
5. WHEN a `stream_close` command is received with a valid `session_id`, THE RTSP_Relay SHALL close the pion/webrtc PeerConnection and the underlying RTSP session for that Stream_Session.
6. IF a `stream_close` command is received with an unknown `session_id`, THEN THE RTSP_Relay SHALL return an error indicating the session was not found.
7. THE Plugin SHALL support at least 4 concurrent Stream_Sessions per plugin process; IF a `stream_offer` command would exceed the configured maximum, THEN THE RTSP_Relay SHALL return an error indicating the limit.

### Requirement 3: Trickle ICE Candidate Exchange

**User Story:** As a Celestia operator, I want ICE candidates to be exchanged incrementally, so that stream connections establish quickly even on networks where full candidate gathering takes time.

#### Acceptance Criteria

1. WHEN the RTSP_Relay gathers a new local ICE candidate after the SDP answer is sent, THE RTSP_Relay SHALL emit a `device.event.occurred` event with `event_type: "ice_candidate"`, the `session_id`, and the candidate string in the event payload.
2. WHEN the WebRTC_Signalling_Handler receives a `POST /api/v1/devices/{id}/stream/ice` request with a valid `session_id` and `candidate` string, THE WebRTC_Signalling_Handler SHALL forward the candidate to the Plugin via the `stream_ice` command and return HTTP 204.
3. IF the `session_id` in a `stream_ice` command does not match an active Stream_Session, THEN THE RTSP_Relay SHALL return an error.

### Requirement 4: Core HTTP Signalling Endpoints

**User Story:** As a Celestia operator, I want the gateway to expose HTTP endpoints for WebRTC signalling, so that the admin UI can initiate and manage stream sessions through the standard API surface.

#### Acceptance Criteria

1. THE WebRTC_Signalling_Handler SHALL expose `POST /api/v1/devices/{id}/stream/offer` accepting a JSON body `{"sdp": "<offer SDP string>"}` and returning `{"session_id": "<id>", "sdp": "<answer SDP string>"}` with HTTP 200 on success.
2. THE WebRTC_Signalling_Handler SHALL expose `DELETE /api/v1/devices/{id}/stream/{session_id}` and return HTTP 204 on successful session teardown.
3. THE WebRTC_Signalling_Handler SHALL expose `POST /api/v1/devices/{id}/stream/ice` accepting `{"session_id": "<id>", "candidate": "<ICE candidate string>"}` and returning HTTP 204 on success.
4. IF the target device does not have `"stream"` in its `Capabilities` list, THEN THE WebRTC_Signalling_Handler SHALL return HTTP 422 with a JSON error body.
5. IF the target device's owning plugin is not running, THEN THE WebRTC_Signalling_Handler SHALL return HTTP 503 with a JSON error body.
6. WHEN a `stream_offer` or `stream_close` or `stream_ice` request passes Core policy and audit checks, THE WebRTC_Signalling_Handler SHALL forward the request to the owning plugin via the existing `ExecuteCommand` gRPC path using actions `stream_offer`, `stream_close`, and `stream_ice` respectively.
7. THE WebRTC_Signalling_Handler SHALL record each `stream_offer` and `stream_close` action in the Core audit log with the device ID, actor, and action name.

### Requirement 5: Plugin Command Routing for Stream Actions

**User Story:** As a Celestia operator, I want stream signalling commands to flow through the existing plugin command protocol, so that the architecture boundary between Core and plugin is preserved.

#### Acceptance Criteria

1. WHEN the Plugin receives an `ExecuteCommand` gRPC call with `action: "stream_offer"` and `params: {"sdp": "<offer>"}`, THE Plugin SHALL invoke the RTSP_Relay and return `{"session_id": "<id>", "sdp": "<answer>"}` in the command response payload.
2. WHEN the Plugin receives an `ExecuteCommand` gRPC call with `action: "stream_close"` and `params: {"session_id": "<id>"}`, THE Plugin SHALL close the matching Stream_Session and return `{"closed": true}`.
3. WHEN the Plugin receives an `ExecuteCommand` gRPC call with `action: "stream_ice"` and `params: {"session_id": "<id>", "candidate": "<string>"}`, THE Plugin SHALL deliver the ICE candidate to the matching pion/webrtc PeerConnection and return `{"accepted": true}`.
4. IF the Plugin is stopped while Stream_Sessions are active, THE Plugin SHALL close all active pion/webrtc PeerConnections and RTSP sessions before returning from `Stop`.

### Requirement 6: RTSP URL Construction and Credential Injection

**User Story:** As a Celestia operator, I want the plugin to construct the RTSP URL from the camera's configured parameters, so that no credentials are exposed through the API or stored in device state visible to the UI.

#### Acceptance Criteria

1. THE RTSP_Relay SHALL construct the RTSP URL as `rtsp://<Username>:<Password>@<Host>:<RTSPPort><RTSPPath>` using values from the matching `CameraConfig`, substituting `{channel}` in `RTSPPath` with the configured `Channel` value.
2. THE Plugin SHALL NOT include the camera `Username` or `Password` in any value returned through the `ExecuteCommand` response, device state, or emitted events.
3. THE Plugin SHALL NOT expose the credential-bearing RTSP URL in any field returned to Core or the Admin_UI; the `rtsp_url` field in device state SHALL contain only the host and port without credentials.

### Requirement 7: Admin UI Stream Viewer Component

**User Story:** As a Celestia operator, I want to open a live video feed for a Hikvision camera from the device detail page in the admin UI, so that I can monitor camera output without leaving the admin interface.

#### Acceptance Criteria

1. WHEN the Admin_UI renders a device detail page for a device with `"stream"` in its capabilities, THE Admin_UI SHALL display a "Live View" button.
2. WHEN the operator clicks "Live View", THE Admin_UI SHALL POST an SDP offer to `POST /api/v1/devices/{id}/stream/offer` and attach the returned SDP answer to a browser `RTCPeerConnection`.
3. WHILE a Stream_Session is active, THE Admin_UI SHALL display the video element with the live feed and a "Stop" button.
4. WHEN the operator clicks "Stop" or navigates away from the device detail page, THE Admin_UI SHALL send `DELETE /api/v1/devices/{id}/stream/{session_id}` and close the local `RTCPeerConnection`.
5. WHEN the Admin_UI receives an SSE event with `event_type: "ice_candidate"` for the active `session_id`, THE Admin_UI SHALL call `RTCPeerConnection.addIceCandidate` with the candidate from the event payload.
6. WHEN the Admin_UI gathers a local ICE candidate from the `RTCPeerConnection`, THE Admin_UI SHALL POST it to `POST /api/v1/devices/{id}/stream/ice`.
7. IF the `POST /api/v1/devices/{id}/stream/offer` request returns an error, THE Admin_UI SHALL display the error message returned by the API and SHALL NOT leave a dangling `RTCPeerConnection` open.
8. THE Admin_UI SHALL use only the gateway HTTP API and the existing SSE event stream for all stream signalling; it SHALL NOT open a direct connection to the plugin process or to the camera.

### Requirement 8: Stream Session Lifecycle and Cleanup

**User Story:** As a Celestia operator, I want stale stream sessions to be cleaned up automatically, so that camera resources and plugin memory are not leaked by abandoned browser tabs.

#### Acceptance Criteria

1. THE RTSP_Relay SHALL track the last activity timestamp for each Stream_Session.
2. WHEN a Stream_Session has had no ICE activity and no media forwarding for 60 seconds, THE RTSP_Relay SHALL close the PeerConnection and RTSP session and emit a `device.event.occurred` event with `event_type: "stream_timeout"` and the `session_id`.
3. THE RTSP_Relay SHALL check for idle sessions at an interval no greater than 15 seconds.
4. WHEN the pion/webrtc PeerConnection for a Stream_Session transitions to the `failed` or `closed` ICE connection state, THE RTSP_Relay SHALL immediately close the associated RTSP session and remove the Stream_Session from the active set.

### Requirement 9: Configuration

**User Story:** As a Celestia operator, I want to control stream relay behaviour through the existing plugin configuration, so that I can tune resource usage per deployment without modifying code.

#### Acceptance Criteria

1. THE Plugin SHALL read a `max_stream_sessions` integer from the plugin config entry, defaulting to 4 when absent or zero.
2. THE Plugin SHALL read a `stream_idle_timeout_seconds` integer from the plugin config entry, defaulting to 60 when absent or zero, with a minimum enforced value of 10.
3. IF `max_stream_sessions` is set to a value less than 1, THE Plugin SHALL treat it as 1.
4. THE Plugin config schema exposed in the plugin manifest SHALL document `max_stream_sessions` and `stream_idle_timeout_seconds` as optional integer fields.
