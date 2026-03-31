package httpapi

import (
	"encoding/json"
	"net/http"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
)

func (s *Server) handleStreamOffer(w http.ResponseWriter, r *http.Request) {
	device, ok := s.resolveStreamDevice(w, r)
	if !ok {
		return
	}

	var body struct {
		SDP string `json:"sdp"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result, err := s.gateway.SendDeviceCommand(r.Context(), gatewayapi.DeviceCommandRequest{
		DeviceID: device,
		Actor:    actorFromRequest(r),
		Action:   "stream_offer",
		Params:   map[string]any{"sdp": body.SDP},
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	payload := result.Result.Payload
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": payload["session_id"],
		"sdp":        payload["sdp"],
	})
}

func (s *Server) handleStreamClose(w http.ResponseWriter, r *http.Request) {
	device, ok := s.resolveStreamDevice(w, r)
	if !ok {
		return
	}

	sessionID := r.PathValue("session_id")
	_, err := s.gateway.SendDeviceCommand(r.Context(), gatewayapi.DeviceCommandRequest{
		DeviceID: device,
		Actor:    actorFromRequest(r),
		Action:   "stream_close",
		Params:   map[string]any{"session_id": sessionID},
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStreamICE(w http.ResponseWriter, r *http.Request) {
	device, ok := s.resolveStreamDevice(w, r)
	if !ok {
		return
	}

	var body struct {
		SessionID string `json:"session_id"`
		Candidate string `json:"candidate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	_, err := s.gateway.SendDeviceCommand(r.Context(), gatewayapi.DeviceCommandRequest{
		DeviceID: device,
		Actor:    actorFromRequest(r),
		Action:   "stream_ice",
		Params:   map[string]any{"session_id": body.SessionID, "candidate": body.Candidate},
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// resolveStreamDevice validates the device exists, has the "stream" capability,
// and that its plugin is running. Returns the device ID on success.
func (s *Server) resolveStreamDevice(w http.ResponseWriter, r *http.Request) (string, bool) {
	deviceID := r.PathValue("id")
	view, err := s.gateway.GetDevice(r.Context(), deviceID)
	if err != nil {
		writeServiceError(w, err)
		return "", false
	}

	hasStream := false
	for _, cap := range view.Device.Capabilities {
		if cap == "stream" {
			hasStream = true
			break
		}
	}
	if !hasStream {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error": "device does not support streaming",
		})
		return "", false
	}

	if !s.plugins.IsRunning(view.Device.PluginID) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "plugin is not running",
		})
		return "", false
	}

	return deviceID, true
}
