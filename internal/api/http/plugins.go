package httpapi

import (
	"encoding/json"
	"net/http"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
)

func (s *Server) handleCatalogPlugins(w http.ResponseWriter, r *http.Request) {
	views, err := s.gateway.ListCatalogPlugins(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	views, err := s.gateway.ListPlugins(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handleInstallPlugin(w http.ResponseWriter, r *http.Request) {
	var req gatewayapi.InstallPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	record, err := s.gateway.InstallPlugin(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) handleUpdatePluginConfig(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Config map[string]any `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	record, err := s.gateway.UpdatePluginConfig(r.Context(), gatewayapi.UpdatePluginConfigRequest{
		PluginID: r.PathValue("id"),
		Config:   payload.Config,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleEnablePlugin(w http.ResponseWriter, r *http.Request) {
	if err := s.gateway.EnablePlugin(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDisablePlugin(w http.ResponseWriter, r *http.Request) {
	if err := s.gateway.DisablePlugin(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDiscoverPlugin(w http.ResponseWriter, r *http.Request) {
	if err := s.gateway.DiscoverPlugin(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeletePlugin(w http.ResponseWriter, r *http.Request) {
	if err := s.gateway.DeletePlugin(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePluginLogs(w http.ResponseWriter, r *http.Request) {
	view, err := s.gateway.GetPluginLogs(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}
