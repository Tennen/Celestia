package httpapi

import (
	"encoding/json"
	"net/http"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
)

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	items, err := s.gateway.ListDevices(r.Context(), gatewayapi.DeviceFilter{
		PluginID: r.URL.Query().Get("plugin_id"),
		Kind:     r.URL.Query().Get("kind"),
		Query:    r.URL.Query().Get("q"),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDevice(w http.ResponseWriter, r *http.Request) {
	item, err := s.gateway.GetDevice(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateDevicePreference(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Alias string `json:"alias"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	pref, err := s.gateway.UpdateDevicePreference(r.Context(), gatewayapi.UpdateDevicePreferenceRequest{
		DeviceID: r.PathValue("id"),
		Alias:    payload.Alias,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pref)
}

func (s *Server) handleUpdateControlPreference(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Alias   string `json:"alias"`
		Visible *bool  `json:"visible"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	pref, err := s.gateway.UpdateControlPreference(r.Context(), gatewayapi.UpdateControlPreferenceRequest{
		DeviceID:  r.PathValue("id"),
		ControlID: r.PathValue("controlId"),
		Alias:     payload.Alias,
		Visible:   payload.Visible,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pref)
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Action string         `json:"action"`
		Params map[string]any `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.gateway.SendDeviceCommand(r.Context(), gatewayapi.DeviceCommandRequest{
		DeviceID: r.PathValue("id"),
		Actor:    actorFromRequest(r),
		Action:   payload.Action,
		Params:   payload.Params,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleToggleOn(w http.ResponseWriter, r *http.Request) {
	s.handleToggleControl(w, r, true)
}

func (s *Server) handleToggleOff(w http.ResponseWriter, r *http.Request) {
	s.handleToggleControl(w, r, false)
}

func (s *Server) handleToggleControl(w http.ResponseWriter, r *http.Request, on bool) {
	result, err := s.gateway.ToggleControl(r.Context(), gatewayapi.ToggleControlRequest{
		CompoundControlID: r.PathValue("id"),
		Actor:             actorFromRequest(r),
		On:                on,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleActionControl(w http.ResponseWriter, r *http.Request) {
	result, err := s.gateway.RunActionControl(r.Context(), gatewayapi.ActionControlRequest{
		CompoundControlID: r.PathValue("id"),
		Actor:             actorFromRequest(r),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
