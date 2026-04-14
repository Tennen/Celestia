package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
)

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	fromTS, err := parseOptionalRFC3339Time(r.URL.Query().Get("from_ts"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("from_ts: %w", err))
		return
	}
	toTS, err := parseOptionalRFC3339Time(r.URL.Query().Get("to_ts"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("to_ts: %w", err))
		return
	}
	beforeTS, err := parseOptionalRFC3339Time(r.URL.Query().Get("before_ts"))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("before_ts: %w", err))
		return
	}
	items, err := s.gateway.ListEvents(r.Context(), gatewayapi.EventFilter{
		PluginID: r.URL.Query().Get("plugin_id"),
		DeviceID: r.URL.Query().Get("device_id"),
		FromTS:   fromTS,
		ToTS:     toTS,
		BeforeTS: beforeTS,
		BeforeID: r.URL.Query().Get("before_id"),
		Limit:    parseLimit(r.URL.Query().Get("limit"), 100),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleEventStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	id, ch := s.runtime.EventBus.Subscribe(64)
	defer s.runtime.EventBus.Unsubscribe(id)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case event := <-ch:
			raw, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: %s\n", event.Type)
			fmt.Fprintf(w, "data: %s\n\n", raw)
			flusher.Flush()
		}
	}
}

func (s *Server) handleAudits(w http.ResponseWriter, r *http.Request) {
	items, err := s.gateway.ListAudits(r.Context(), gatewayapi.AuditFilter{
		DeviceID: r.URL.Query().Get("device_id"),
		Limit:    parseLimit(r.URL.Query().Get("limit"), 100),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}
