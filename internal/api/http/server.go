package httpapi

import (
	"context"
	"log"
	"net/http"
	"time"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
	runtimepkg "github.com/chentianyu/celestia/internal/core/runtime"
)

type Server struct {
	runtime *runtimepkg.Runtime
	gateway gatewayapi.Service
	server  *http.Server
}

func New(addr string, runtime *runtimepkg.Runtime) *Server {
	s := &Server{
		runtime: runtime,
		gateway: gatewayapi.NewRuntimeService(runtime),
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
	mux.HandleFunc("PUT /api/v1/devices/{id}/preference", s.handleUpdateDevicePreference)
	mux.HandleFunc("PUT /api/v1/devices/{id}/controls/{controlId}", s.handleUpdateControlPreference)
	mux.HandleFunc("POST /api/v1/devices/{id}/commands", s.handleCommand)
	mux.HandleFunc("POST /api/v1/toggle/{id}/on", s.handleToggleOn)
	mux.HandleFunc("POST /api/v1/toggle/{id}/off", s.handleToggleOff)
	mux.HandleFunc("POST /api/v1/action/{id}", s.handleActionControl)
	mux.HandleFunc("GET /api/external/v1/devices", s.handleDevices)
	mux.HandleFunc("GET /api/external/v1/devices/{id}", s.handleDevice)
	mux.HandleFunc("POST /api/external/v1/devices/{id}/commands", s.handleCommand)
	mux.HandleFunc("POST /api/external/v1/toggle/{id}/on", s.handleToggleOn)
	mux.HandleFunc("POST /api/external/v1/toggle/{id}/off", s.handleToggleOff)
	mux.HandleFunc("POST /api/external/v1/action/{id}", s.handleActionControl)
	mux.HandleFunc("GET /api/v1/events", s.handleEvents)
	mux.HandleFunc("GET /api/v1/events/stream", s.handleEventStream)
	mux.HandleFunc("GET /api/v1/audits", s.handleAudits)
	mux.HandleFunc("POST /api/v1/oauth/xiaomi/start", s.handleXiaomiOAuthStart)
	mux.HandleFunc("GET /api/v1/oauth/xiaomi/sessions/{id}", s.handleXiaomiOAuthSession)
	mux.HandleFunc("GET /api/v1/oauth/xiaomi/callback", s.handleXiaomiOAuthCallback)
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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	payload, err := s.gateway.Health(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	summary, err := s.gateway.Dashboard(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
