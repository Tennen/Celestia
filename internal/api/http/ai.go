package httpapi

import (
	"encoding/json"
	"net/http"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
)

func (s *Server) handleAIDevices(w http.ResponseWriter, r *http.Request) {
	items, err := s.gateway.ListAIDevices(r.Context(), gatewayapi.DeviceFilter{
		PluginID: r.URL.Query().Get("plugin_id"),
		Kind:     r.URL.Query().Get("kind"),
		Query:    r.URL.Query().Get("q"),
	})
	if err != nil {
		writeAIServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAICommand(w http.ResponseWriter, r *http.Request) {
	var payload gatewayapi.AICommandRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	payload.Actor = actorFromRequestWithDefault(r, "ai")
	result, err := s.gateway.ExecuteAICommand(r.Context(), payload)
	if err != nil {
		writeAIServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
