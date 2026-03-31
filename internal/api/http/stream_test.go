package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
	"github.com/chentianyu/celestia/internal/models"
)

// stubGateway implements gatewayapi.Service for stream handler tests.
// Only GetDevice and SendDeviceCommand are exercised; all others return zero values.
type stubGateway struct {
	device        models.DeviceView
	deviceErr     error
	commandResult gatewayapi.CommandExecutionResult
	commandErr    error
}

func (g *stubGateway) Health(_ context.Context) (gatewayapi.HealthStatus, error) {
	return gatewayapi.HealthStatus{}, nil
}
func (g *stubGateway) Dashboard(_ context.Context) (models.DashboardSummary, error) {
	return models.DashboardSummary{}, nil
}
func (g *stubGateway) ListCatalogPlugins(_ context.Context) ([]models.CatalogPlugin, error) {
	return nil, nil
}
func (g *stubGateway) ListPlugins(_ context.Context) ([]models.PluginRuntimeView, error) {
	return nil, nil
}
func (g *stubGateway) InstallPlugin(_ context.Context, _ gatewayapi.InstallPluginRequest) (models.PluginInstallRecord, error) {
	return models.PluginInstallRecord{}, nil
}
func (g *stubGateway) UpdatePluginConfig(_ context.Context, _ gatewayapi.UpdatePluginConfigRequest) (models.PluginInstallRecord, error) {
	return models.PluginInstallRecord{}, nil
}
func (g *stubGateway) EnablePlugin(_ context.Context, _ string) error  { return nil }
func (g *stubGateway) DisablePlugin(_ context.Context, _ string) error { return nil }
func (g *stubGateway) DiscoverPlugin(_ context.Context, _ string) error { return nil }
func (g *stubGateway) DeletePlugin(_ context.Context, _ string) error  { return nil }
func (g *stubGateway) GetPluginLogs(_ context.Context, _ string) (gatewayapi.PluginLogsView, error) {
	return gatewayapi.PluginLogsView{}, nil
}
func (g *stubGateway) ListDevices(_ context.Context, _ gatewayapi.DeviceFilter) ([]models.DeviceView, error) {
	return nil, nil
}
func (g *stubGateway) GetDevice(_ context.Context, _ string) (models.DeviceView, error) {
	return g.device, g.deviceErr
}
func (g *stubGateway) UpdateDevicePreference(_ context.Context, _ gatewayapi.UpdateDevicePreferenceRequest) (models.DevicePreference, error) {
	return models.DevicePreference{}, nil
}
func (g *stubGateway) UpdateControlPreference(_ context.Context, _ gatewayapi.UpdateControlPreferenceRequest) (models.DeviceControlPreference, error) {
	return models.DeviceControlPreference{}, nil
}
func (g *stubGateway) SendDeviceCommand(_ context.Context, _ gatewayapi.DeviceCommandRequest) (gatewayapi.CommandExecutionResult, error) {
	return g.commandResult, g.commandErr
}
func (g *stubGateway) ToggleControl(_ context.Context, _ gatewayapi.ToggleControlRequest) (gatewayapi.CommandExecutionResult, error) {
	return gatewayapi.CommandExecutionResult{}, nil
}
func (g *stubGateway) RunActionControl(_ context.Context, _ gatewayapi.ActionControlRequest) (gatewayapi.CommandExecutionResult, error) {
	return gatewayapi.CommandExecutionResult{}, nil
}
func (g *stubGateway) ListEvents(_ context.Context, _ gatewayapi.EventFilter) ([]models.Event, error) {
	return nil, nil
}
func (g *stubGateway) ListAudits(_ context.Context, _ gatewayapi.AuditFilter) ([]models.AuditRecord, error) {
	return nil, nil
}

// stubPluginChecker implements pluginChecker for tests.
type stubPluginChecker struct {
	running bool
}

func (m *stubPluginChecker) IsRunning(_ string) bool { return m.running }

// newStreamTestServer builds a Server with injected stub dependencies.
func newStreamTestServer(gw gatewayapi.Service, pluginRunning bool) *Server {
	return &Server{
		gateway: gw,
		plugins: &stubPluginChecker{running: pluginRunning},
	}
}

// streamDevice returns a DeviceView that has the "stream" capability.
func streamDevice(pluginID string) models.DeviceView {
	return models.DeviceView{
		Device: models.Device{
			ID:           "dev-1",
			PluginID:     pluginID,
			Capabilities: []string{"stream"},
		},
	}
}

// noStreamDevice returns a DeviceView without the "stream" capability.
func noStreamDevice() models.DeviceView {
	return models.DeviceView{
		Device: models.Device{
			ID:           "dev-1",
			PluginID:     "hikvision",
			Capabilities: []string{"state"},
		},
	}
}

func TestHandleStreamOffer_NoStreamCapability(t *testing.T) {
	gw := &stubGateway{device: noStreamDevice()}
	s := newStreamTestServer(gw, true)

	body := bytes.NewBufferString(`{"sdp":"offer"}`)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dev-1/stream/offer", body)
	r.SetPathValue("id", "dev-1")
	w := httptest.NewRecorder()

	s.handleStreamOffer(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "device does not support streaming" {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func TestHandleStreamOffer_PluginNotRunning(t *testing.T) {
	gw := &stubGateway{device: streamDevice("hikvision")}
	s := newStreamTestServer(gw, false)

	body := bytes.NewBufferString(`{"sdp":"offer"}`)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dev-1/stream/offer", body)
	r.SetPathValue("id", "dev-1")
	w := httptest.NewRecorder()

	s.handleStreamOffer(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "plugin is not running" {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func TestHandleStreamOffer_HappyPath(t *testing.T) {
	gw := &stubGateway{
		device: streamDevice("hikvision"),
		commandResult: gatewayapi.CommandExecutionResult{
			Result: models.CommandResponse{
				Accepted: true,
				Payload: map[string]any{
					"session_id": "sess-abc",
					"sdp":        "answer-sdp",
				},
			},
		},
	}
	s := newStreamTestServer(gw, true)

	body := bytes.NewBufferString(`{"sdp":"offer-sdp"}`)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dev-1/stream/offer", body)
	r.SetPathValue("id", "dev-1")
	w := httptest.NewRecorder()

	s.handleStreamOffer(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["session_id"] != "sess-abc" {
		t.Fatalf("expected session_id=sess-abc, got %v", resp["session_id"])
	}
	if resp["sdp"] != "answer-sdp" {
		t.Fatalf("expected sdp=answer-sdp, got %v", resp["sdp"])
	}
}

func TestHandleStreamClose_HappyPath(t *testing.T) {
	gw := &stubGateway{
		device:        streamDevice("hikvision"),
		commandResult: gatewayapi.CommandExecutionResult{Result: models.CommandResponse{Accepted: true}},
	}
	s := newStreamTestServer(gw, true)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/dev-1/stream/sess-abc", nil)
	r.SetPathValue("id", "dev-1")
	r.SetPathValue("session_id", "sess-abc")
	w := httptest.NewRecorder()

	s.handleStreamClose(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleStreamICE_HappyPath(t *testing.T) {
	gw := &stubGateway{
		device:        streamDevice("hikvision"),
		commandResult: gatewayapi.CommandExecutionResult{Result: models.CommandResponse{Accepted: true}},
	}
	s := newStreamTestServer(gw, true)

	body := bytes.NewBufferString(`{"session_id":"sess-abc","candidate":"candidate:..."}`)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dev-1/stream/ice", body)
	r.SetPathValue("id", "dev-1")
	w := httptest.NewRecorder()

	s.handleStreamICE(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleStreamClose_NoStreamCapability(t *testing.T) {
	gw := &stubGateway{device: noStreamDevice()}
	s := newStreamTestServer(gw, true)

	r := httptest.NewRequest(http.MethodDelete, "/api/v1/devices/dev-1/stream/sess-abc", nil)
	r.SetPathValue("id", "dev-1")
	r.SetPathValue("session_id", "sess-abc")
	w := httptest.NewRecorder()

	s.handleStreamClose(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestHandleStreamICE_PluginNotRunning(t *testing.T) {
	gw := &stubGateway{device: streamDevice("hikvision")}
	s := newStreamTestServer(gw, false)

	body := bytes.NewBufferString(`{"session_id":"sess-abc","candidate":"candidate:..."}`)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/devices/dev-1/stream/ice", body)
	r.SetPathValue("id", "dev-1")
	w := httptest.NewRecorder()

	s.handleStreamICE(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
