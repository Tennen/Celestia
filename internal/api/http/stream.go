package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
)

func (s *Server) handleStreamOffer(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := s.resolveStreamDevice(w, r)
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
	if body.SDP == "" {
		writeError(w, http.StatusBadRequest, errors.New("sdp is required"))
		return
	}

	// Ask the plugin for the RTSP URL (credentials stay server-side).
	urlResult, err := s.gateway.SendDeviceCommand(r.Context(), gatewayapi.DeviceCommandRequest{
		DeviceID: deviceID,
		Actor:    actorFromRequest(r),
		Action:   "stream_rtsp_url",
	})
	if err != nil {
		log.Printf("[stream] stream_rtsp_url command failed for device=%s: %v", deviceID, err)
		writeServiceError(w, err)
		return
	}
	rtspURL, _ := urlResult.Result.Payload["rtsp_url"].(string)
	if rtspURL == "" {
		log.Printf("[stream] plugin returned empty rtsp_url for device=%s payload=%v", deviceID, urlResult.Result.Payload)
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": "plugin did not return rtsp_url"})
		return
	}
	log.Printf("[stream] device=%s calling relay.Offer nat_ip=%q", deviceID, s.streamRelay.NatIP())

	sessionID, sdpAnswer, err := s.streamRelay.Offer(r.Context(), deviceID, rtspURL, body.SDP)
	if err != nil {
		log.Printf("[stream] relay.Offer failed for device=%s: %v", deviceID, err)
		writeServiceError(w, err)
		return
	}
	log.Printf("[stream] session=%s opened for device=%s", sessionID[:8], deviceID)

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"sdp":        sdpAnswer,
	})
}

func (s *Server) handleStreamClose(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.resolveStreamDevice(w, r); !ok {
		return
	}
	sessionID := r.PathValue("session_id")
	if err := s.streamRelay.Close(sessionID); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStreamICE(w http.ResponseWriter, r *http.Request) {
	// Vanilla ICE: all candidates are embedded in the SDP answer.
	// This endpoint is kept for compatibility but is not called in normal flow.
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
