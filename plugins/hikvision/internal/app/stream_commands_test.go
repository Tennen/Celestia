package app

import (
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func newTestPlugin(_ int) *Plugin {
	return &Plugin{
		entries:     map[string]*entryRuntime{},
		deviceIndex: map[string]string{},
		events:      make(chan models.Event, 16),
	}
}

func newTestRuntime() *entryRuntime {
	return &entryRuntime{
		Config: EntryConfig{
			EntryID:  "test-entry",
			DeviceID: "hikvision:camera:test-entry",
			Host:     "192.0.2.1",
			RTSPPort: 554,
			RTSPPath: "/h264/ch1/main/av_stream",
			Username: "admin",
			Password: "password",
			Channel:  1,
		},
		Device: models.Device{
			ID: "hikvision:camera:test-entry",
		},
	}
}

// TestHandleStreamRTSPURL verifies that handleStreamRTSPURL returns a well-formed URL.
func TestHandleStreamRTSPURL(t *testing.T) {
	p := newTestPlugin(4)
	runtime := newTestRuntime()

	payload, _, err := p.handleStreamRTSPURL(runtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	url, ok := payload["rtsp_url"].(string)
	if !ok || url == "" {
		t.Fatal("expected non-empty rtsp_url in payload")
	}
	if url != "rtsp://admin:password@192.0.2.1:554/h264/ch1/main/av_stream" {
		t.Errorf("unexpected rtsp_url: %s", url)
	}
}

func TestHandleStreamRTSPURL_UsesExplicitRTSPURL(t *testing.T) {
	p := newTestPlugin(4)
	runtime := newTestRuntime()
	runtime.Config.RTSPURL = "rtsp://viewer:secret@example.invalid:8554/live"

	payload, _, err := p.handleStreamRTSPURL(runtime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	url, ok := payload["rtsp_url"].(string)
	if !ok {
		t.Fatalf("payload rtsp_url = %#v, want string", payload["rtsp_url"])
	}
	if url != runtime.Config.RTSPURL {
		t.Fatalf("rtsp_url = %q, want %q", url, runtime.Config.RTSPURL)
	}
}
