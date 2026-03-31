package app

import (
	"context"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

// newTestPlugin creates a minimal Plugin with a relay for command handler tests.
func newTestPlugin(maxSessions int) *Plugin {
	relay := NewRTSPRelay(maxSessions, 60*time.Second, noopEmit)
	return &Plugin{
		entries:     map[string]*entryRuntime{},
		deviceIndex: map[string]string{},
		events:      make(chan models.Event, 16),
		relay:       relay,
	}
}

// newTestRuntime creates a minimal entryRuntime for command handler tests.
func newTestRuntime() *entryRuntime {
	return &entryRuntime{
		Config: CameraConfig{
			EntryID:  "test-entry",
			DeviceID: "hikvision:camera:test-entry",
			Host:     "192.0.2.1",
			RTSPPort: 554,
			RTSPPath: "/Streaming/Channels/101",
			Username: "admin",
			Password: "password",
			Channel:  1,
		},
		Device: models.Device{
			ID: "hikvision:camera:test-entry",
		},
	}
}

// TestHandleStreamOffer_MissingSDP verifies that handleStreamOffer returns an error
// when the sdp param is absent.
// Validates: Requirements 5.1
func TestHandleStreamOffer_MissingSDP(t *testing.T) {
	p := newTestPlugin(4)
	defer p.relay.CloseAll()

	_, _, err := p.handleStreamOffer(context.Background(), newTestRuntime(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing sdp param, got nil")
	}
}

// TestHandleStreamOffer_EmptySDP verifies that handleStreamOffer returns an error
// when the sdp param is an empty string.
// Validates: Requirements 5.1
func TestHandleStreamOffer_EmptySDP(t *testing.T) {
	p := newTestPlugin(4)
	defer p.relay.CloseAll()

	_, _, err := p.handleStreamOffer(context.Background(), newTestRuntime(), map[string]any{"sdp": ""})
	if err == nil {
		t.Error("expected error for empty sdp param, got nil")
	}
}

// TestHandleStreamClose_MissingSessionID verifies that handleStreamClose returns an error
// when the session_id param is absent.
// Validates: Requirements 5.2
func TestHandleStreamClose_MissingSessionID(t *testing.T) {
	p := newTestPlugin(4)
	defer p.relay.CloseAll()

	_, _, err := p.handleStreamClose(map[string]any{})
	if err == nil {
		t.Error("expected error for missing session_id param, got nil")
	}
}

// TestHandleStreamICE_MissingSessionID verifies that handleStreamICE returns an error
// when the session_id param is absent.
// Validates: Requirements 5.3
func TestHandleStreamICE_MissingSessionID(t *testing.T) {
	p := newTestPlugin(4)
	defer p.relay.CloseAll()

	_, _, err := p.handleStreamICE(map[string]any{"candidate": "candidate:1 1 UDP 2130706431 192.168.1.1 54321 typ host"})
	if err == nil {
		t.Error("expected error for missing session_id param, got nil")
	}
}

// TestHandleStreamICE_MissingCandidate verifies that handleStreamICE returns an error
// when the candidate param is absent.
// Validates: Requirements 5.3
func TestHandleStreamICE_MissingCandidate(t *testing.T) {
	p := newTestPlugin(4)
	defer p.relay.CloseAll()

	_, _, err := p.handleStreamICE(map[string]any{"session_id": "some-session-id"})
	if err == nil {
		t.Error("expected error for missing candidate param, got nil")
	}
}
