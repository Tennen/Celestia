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
	config.ServiceWSURL = strings.TrimSpace(config.ServiceWSURL)
	config.ModelName = strings.TrimSpace(config.ModelName)
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
	req.ServiceWSURL = strings.TrimSpace(req.ServiceWSURL)
	req.ModelName = strings.TrimSpace(req.ModelName)
	item, err := s.gateway.RefreshVisionEntityCatalog(r.Context(), req)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
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
