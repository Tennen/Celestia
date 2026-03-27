package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/chentianyu/celestia/internal/core/pluginmgr"
)

func (s *Server) handleCatalogPlugins(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.runtime.PluginMgr.Catalog())
}

func (s *Server) handlePlugins(w http.ResponseWriter, r *http.Request) {
	views, err := s.runtime.PluginMgr.ListRuntimeViews(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) handleInstallPlugin(w http.ResponseWriter, r *http.Request) {
	var req pluginmgr.InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	record, err := s.runtime.PluginMgr.Install(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
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
	record, err := s.runtime.PluginMgr.UpdateConfig(r.Context(), r.PathValue("id"), payload.Config)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not installed") {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleEnablePlugin(w http.ResponseWriter, r *http.Request) {
	if err := s.runtime.PluginMgr.Enable(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDisablePlugin(w http.ResponseWriter, r *http.Request) {
	if err := s.runtime.PluginMgr.Disable(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDiscoverPlugin(w http.ResponseWriter, r *http.Request) {
	if err := s.runtime.PluginMgr.Discover(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleDeletePlugin(w http.ResponseWriter, r *http.Request) {
	if err := s.runtime.PluginMgr.Uninstall(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePluginLogs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"plugin_id": r.PathValue("id"),
		"logs":      s.runtime.PluginMgr.GetLogs(r.PathValue("id")),
	})
}
