package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	items, err := s.gateway.ListCapabilities(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCapability(w http.ResponseWriter, r *http.Request) {
	item, err := s.gateway.GetCapability(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateVisionCapability(w http.ResponseWriter, r *http.Request) {
	var config models.VisionCapabilityConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	config.ServiceURL = strings.TrimSpace(config.ServiceURL)
	item, err := s.gateway.SaveVisionCapabilityConfig(r.Context(), config)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleVisionCapabilityStatus(w http.ResponseWriter, r *http.Request) {
	var report models.VisionServiceStatusReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.gateway.ReportVisionCapabilityStatus(r.Context(), report)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleVisionCapabilityEvents(w http.ResponseWriter, r *http.Request) {
	var batch models.VisionServiceEventBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.gateway.ReportVisionCapabilityEvents(r.Context(), batch); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
