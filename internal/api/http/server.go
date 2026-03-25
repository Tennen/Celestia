package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/core/pluginmgr"
	runtimepkg "github.com/chentianyu/celestia/internal/core/runtime"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
	"github.com/google/uuid"
)

type Server struct {
	runtime *runtimepkg.Runtime
	server  *http.Server
}

func New(addr string, runtime *runtimepkg.Runtime) *Server {
	s := &Server{
		runtime: runtime,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/dashboard", s.handleDashboard)
	mux.HandleFunc("GET /api/v1/catalog/plugins", s.handleCatalogPlugins)
	mux.HandleFunc("GET /api/v1/plugins", s.handlePlugins)
	mux.HandleFunc("POST /api/v1/plugins", s.handleInstallPlugin)
	mux.HandleFunc("PUT /api/v1/plugins/{id}/config", s.handleUpdatePluginConfig)
	mux.HandleFunc("POST /api/v1/plugins/{id}/enable", s.handleEnablePlugin)
	mux.HandleFunc("POST /api/v1/plugins/{id}/disable", s.handleDisablePlugin)
	mux.HandleFunc("POST /api/v1/plugins/{id}/discover", s.handleDiscoverPlugin)
	mux.HandleFunc("DELETE /api/v1/plugins/{id}", s.handleDeletePlugin)
	mux.HandleFunc("GET /api/v1/plugins/{id}/logs", s.handlePluginLogs)
	mux.HandleFunc("GET /api/v1/devices", s.handleDevices)
	mux.HandleFunc("GET /api/v1/devices/{id}", s.handleDevice)
	mux.HandleFunc("POST /api/v1/devices/{id}/commands", s.handleCommand)
	mux.HandleFunc("GET /api/v1/events", s.handleEvents)
	mux.HandleFunc("GET /api/v1/events/stream", s.handleEventStream)
	mux.HandleFunc("GET /api/v1/audits", s.handleAudits)
	mux.Handle("/", http.FileServer(http.Dir("./web/admin/dist")))
	s.server = &http.Server{
		Addr:              addr,
		Handler:           withCORS(withLogging(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

func (s *Server) Start() error {
	log.Printf("gateway listening on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	summary, err := s.runtime.Dashboard(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

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

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := s.runtime.Registry.List(r.Context(), storage.DeviceFilter{
		PluginID: r.URL.Query().Get("plugin_id"),
		Kind:     r.URL.Query().Get("kind"),
		Query:    r.URL.Query().Get("q"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	states, err := s.runtime.State.List(r.Context(), storage.StateFilter{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	stateMap := map[string]models.DeviceStateSnapshot{}
	for _, item := range states {
		stateMap[item.DeviceID] = item
	}
	type deviceView struct {
		Device models.Device               `json:"device"`
		State  models.DeviceStateSnapshot  `json:"state"`
	}
	out := make([]deviceView, 0, len(devices))
	for _, device := range devices {
		out = append(out, deviceView{Device: device, State: stateMap[device.ID]})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDevice(w http.ResponseWriter, r *http.Request) {
	device, ok, err := s.runtime.Registry.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("device not found"))
		return
	}
	state, _, err := s.runtime.State.Get(r.Context(), device.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"device": device,
		"state":  state,
	})
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	device, ok, err := s.runtime.Registry.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("device not found"))
		return
	}
	var payload struct {
		Action string         `json:"action"`
		Params map[string]any `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	actor := actorFromRequest(r)
	decision := s.runtime.Policy.Evaluate(actor, payload.Action)
	auditRecord := models.AuditRecord{
		ID:        uuid.NewString(),
		Actor:     actor,
		DeviceID:  device.ID,
		Action:    payload.Action,
		Params:    payload.Params,
		Allowed:   decision.Allowed,
		RiskLevel: decision.RiskLevel,
		CreatedAt: time.Now().UTC(),
	}
	if !decision.Allowed {
		auditRecord.Result = "denied"
		_ = s.runtime.Audit.Append(r.Context(), auditRecord)
		writeJSON(w, http.StatusForbidden, map[string]any{
			"allowed": false,
			"reason":  decision.Reason,
		})
		return
	}
	resp, err := s.runtime.PluginMgr.ExecuteCommand(r.Context(), device, models.CommandRequest{
		DeviceID:  device.ID,
		Action:    payload.Action,
		Params:    payload.Params,
		RequestID: uuid.NewString(),
	})
	if err != nil {
		auditRecord.Result = "failed"
		_ = s.runtime.Audit.Append(r.Context(), auditRecord)
		writeError(w, http.StatusBadGateway, err)
		return
	}
	auditRecord.Result = "accepted"
	if err := s.runtime.Audit.Append(r.Context(), auditRecord); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"decision": decision,
		"result":   resp,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	limit := parseLimit(r.URL.Query().Get("limit"), 100)
	events, err := s.runtime.Store.ListEvents(r.Context(), storage.EventFilter{
		PluginID: r.URL.Query().Get("plugin_id"),
		DeviceID: r.URL.Query().Get("device_id"),
		Limit:    limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
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
	limit := parseLimit(r.URL.Query().Get("limit"), 100)
	items, err := s.runtime.Audit.List(r.Context(), storage.AuditFilter{
		DeviceID: r.URL.Query().Get("device_id"),
		Limit:    limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}

func parseLimit(raw string, defaultValue int) int {
	if raw == "" {
		return defaultValue
	}
	var limit int
	if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil || limit <= 0 {
		return defaultValue
	}
	return limit
}

func actorFromRequest(r *http.Request) string {
	actor := strings.TrimSpace(r.Header.Get("X-Actor"))
	if actor == "" {
		actor = "admin"
	}
	return actor
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-Actor")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(started))
	})
}

