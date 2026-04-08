package httpapi

import (
	"encoding/json"
	"errors"
	"io"
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

func (s *Server) handleRefreshVisionEntityCatalog(w http.ResponseWriter, r *http.Request) {
	var req models.VisionEntityCatalogRefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.ServiceURL = strings.TrimSpace(req.ServiceURL)
	item, err := s.gateway.RefreshVisionEntityCatalog(r.Context(), req)
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

func (s *Server) handleVisionCapabilityEvidence(w http.ResponseWriter, r *http.Request) {
	var batch models.VisionServiceEventCaptureBatch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.gateway.ReportVisionCapabilityEvidence(r.Context(), batch); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleVisionCapture(w http.ResponseWriter, r *http.Request) {
	asset, err := s.gateway.GetVisionEventCapture(r.Context(), r.PathValue("captureID"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if asset.Capture.ContentType != "" {
		w.Header().Set("Content-Type", asset.Capture.ContentType)
	}
	w.Header().Set("Cache-Control", "private, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(asset.Data)
}
