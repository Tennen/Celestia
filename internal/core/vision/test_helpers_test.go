package vision

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage/sqlite"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func newVisionTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.New(filepath.Join(t.TempDir(), "vision.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func newVisionTestService(t *testing.T) (*Service, *registry.Service, *state.Service, *sqlite.Store) {
	t.Helper()
	store := newVisionTestStore(t)
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	bus := eventbus.New()
	service := New(store, registrySvc, stateSvc, bus)
	t.Cleanup(service.Close)
	return service, registrySvc, stateSvc, store
}

func visionTestCamera() models.Device {
	return models.Device{
		ID:             "hikvision:camera:entry-1",
		PluginID:       "hikvision",
		VendorDeviceID: "192.0.2.10:8000:ch1",
		Kind:           models.DeviceKindCameraLike,
		Name:           "Patio Camera",
		Online:         true,
		Capabilities:   []string{"stream"},
		Metadata: map[string]any{
			"entry_id": "entry-1",
		},
	}
}

func newVisionWSTestServer(t *testing.T, handler func(context.Context, *websocket.Conn)) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Fatalf("websocket.Accept() error = %v", err)
		}
		defer conn.Close(websocket.StatusNormalClosure, "test done")
		handler(r.Context(), conn)
	}))
	t.Cleanup(server.Close)
	return server
}

func wsURLFromHTTP(serverURL string) string {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return "ws" + strings.TrimPrefix(serverURL, "http")
	}
	parsed.Scheme = strings.Replace(parsed.Scheme, "http", "ws", 1)
	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/api/v1/capabilities/" + models.VisionCapabilityID
	}
	return parsed.String()
}

func sendTestEnvelope(t *testing.T, ctx context.Context, conn *websocket.Conn, messageType, requestID string, payload any) {
	t.Helper()
	raw := wsEnvelope{Type: messageType, RequestID: requestID}
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json marshal %s: %v", messageType, err)
		}
		raw.Payload = body
	}
	if err := wsjson.Write(ctx, conn, raw); err != nil {
		t.Fatalf("wsjson.Write(%s) error = %v", messageType, err)
	}
}

func readTestEnvelope(t *testing.T, ctx context.Context, conn *websocket.Conn) wsEnvelope {
	t.Helper()
	var envelope wsEnvelope
	if err := wsjson.Read(ctx, conn, &envelope); err != nil {
		t.Fatalf("wsjson.Read() error = %v", err)
	}
	return envelope
}

func waitFor[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for test signal")
		var zero T
		return zero
	}
}
