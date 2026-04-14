package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	gateway "github.com/chentianyu/celestia/internal/api/gateway"
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

func (s *Server) handleVisionRuleEvents(w http.ResponseWriter, r *http.Request) {
	fromTS, err := parseOptionalRFC3339Time(r.URL.Query().Get("from_ts"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	toTS, err := parseOptionalRFC3339Time(r.URL.Query().Get("to_ts"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	beforeTS, err := parseOptionalRFC3339Time(r.URL.Query().Get("before_ts"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.gateway.ListVisionRuleEvents(r.Context(), r.PathValue("ruleID"), gateway.VisionRuleEventFilter{
		FromTS:   fromTS,
		ToTS:     toTS,
		BeforeTS: beforeTS,
		BeforeID: r.URL.Query().Get("before_id"),
		Limit:    parseLimit(r.URL.Query().Get("limit"), 50),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleDeleteVisionRuleEvent(w http.ResponseWriter, r *http.Request) {
	if err := s.gateway.DeleteVisionRuleEvent(r.Context(), r.PathValue("ruleID"), r.PathValue("eventID")); err != nil {
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
