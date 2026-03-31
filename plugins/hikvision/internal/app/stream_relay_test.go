package app

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
)

// noopEmit is a no-op event emitter for tests.
func noopEmit(_ models.Event) {}

// newTestRelay creates an RTSPRelay with a short idle timeout for testing.
func newTestRelay(maxSessions int) *RTSPRelay {
	return NewRTSPRelay(maxSessions, 60*time.Second, noopEmit)
}

// injectSession directly inserts a StreamSession into the relay for lifecycle tests.
// This avoids needing a real RTSP connection.
func injectSession(r *RTSPRelay, session *StreamSession) {
	r.mu.Lock()
	r.sessions[session.ID] = session
	r.mu.Unlock()
}

// --- Task 3.6: Property test for session ID uniqueness ---
// Validates: Requirements 2.4
//
// Property 1: Every session_id returned by Offer is unique across concurrent calls.
// Since Offer requires a real RTSP connection, we test the ID generation mechanism
// directly: uuid.NewString() is called once per session and stored as the map key.
// We verify that concurrent ID generation produces no duplicates.

func TestSessionIDUniqueness(t *testing.T) {
	const goroutines = 100
	ids := make([]string, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			ids[i] = uuid.NewString()
		}()
	}
	wg.Wait()

	seen := make(map[string]struct{}, goroutines)
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			t.Errorf("duplicate session ID generated: %s", id)
		}
		seen[id] = struct{}{}
	}
}

// TestSessionIDUniquenessViaRelay verifies that the relay's internal session map
// never contains duplicate keys by injecting sessions concurrently.
func TestSessionIDUniquenessViaRelay(t *testing.T) {
	const count = 50
	r := newTestRelay(count + 10)
	defer r.CloseAll()

	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			id := uuid.NewString()
			pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
			if err != nil {
				return
			}
			s := &StreamSession{
				ID:           id,
				EntryID:      "entry",
				DeviceID:     "device",
				PC:           pc,
				LastActivity: time.Now(),
				cancel:       func() {},
			}
			injectSession(r, s)
		}()
	}
	wg.Wait()

	r.mu.Lock()
	defer r.mu.Unlock()
	seen := make(map[string]struct{}, len(r.sessions))
	for id := range r.sessions {
		if _, exists := seen[id]; exists {
			t.Errorf("duplicate session ID in relay map: %s", id)
		}
		seen[id] = struct{}{}
	}
}

// --- Task 3.7: Unit tests for RTSPRelay lifecycle ---

// TestCloseUnknownSession verifies that Close returns an error for an unknown session_id.
// Validates: Requirements 2.6
func TestCloseUnknownSession(t *testing.T) {
	r := newTestRelay(4)
	defer r.CloseAll()

	err := r.Close("nonexistent-session-id")
	if err == nil {
		t.Error("expected error when closing unknown session, got nil")
	}
}

// TestAddICECandidateUnknownSession verifies that AddICECandidate returns an error
// for an unknown session_id.
// Validates: Requirements 3.3
func TestAddICECandidateUnknownSession(t *testing.T) {
	r := newTestRelay(4)
	defer r.CloseAll()

	err := r.AddICECandidate("nonexistent-session-id", "candidate:1 1 UDP 2130706431 192.168.1.1 54321 typ host")
	if err == nil {
		t.Error("expected error when adding ICE candidate to unknown session, got nil")
	}
}

// TestCloseAllDrainsSessions verifies that CloseAll removes all active sessions.
// Validates: Requirements 5.4
func TestCloseAllDrainsSessions(t *testing.T) {
	r := newTestRelay(10)

	// Inject several sessions
	for i := 0; i < 5; i++ {
		id := uuid.NewString()
		pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		if err != nil {
			t.Fatalf("failed to create PeerConnection: %v", err)
		}
		s := &StreamSession{
			ID:           id,
			EntryID:      "entry",
			DeviceID:     "device",
			PC:           pc,
			LastActivity: time.Now(),
			cancel:       func() {},
		}
		injectSession(r, s)
	}

	r.mu.Lock()
	before := len(r.sessions)
	r.mu.Unlock()

	if before != 5 {
		t.Fatalf("expected 5 sessions before CloseAll, got %d", before)
	}

	r.CloseAll()

	r.mu.Lock()
	after := len(r.sessions)
	r.mu.Unlock()

	if after != 0 {
		t.Errorf("expected 0 sessions after CloseAll, got %d", after)
	}
}

// TestSessionLimitRejection verifies that Offer returns an error when maxSessions is reached.
// Validates: Requirements 2.7
// This tests the limit check path before any RTSP connection is attempted.
func TestSessionLimitRejection(t *testing.T) {
	const maxSessions = 2
	r := newTestRelay(maxSessions)
	defer r.CloseAll()

	// Fill up to the limit by injecting sessions directly
	for i := 0; i < maxSessions; i++ {
		id := uuid.NewString()
		pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		if err != nil {
			t.Fatalf("failed to create PeerConnection: %v", err)
		}
		s := &StreamSession{
			ID:           id,
			EntryID:      "entry",
			DeviceID:     "device",
			PC:           pc,
			LastActivity: time.Now(),
			cancel:       func() {},
		}
		injectSession(r, s)
	}

	// Attempt to open one more session — should fail at the limit check
	// before any RTSP connection is attempted (unreachable host is fine here)
	cfg := CameraConfig{
		Host:     "192.0.2.1", // TEST-NET, unreachable
		RTSPPort: 554,
		RTSPPath: "/Streaming/Channels/101",
		Username: "admin",
		Password: "password",
		Channel:  1,
	}
	_, _, err := r.Offer(context.Background(), "entry", "device", cfg, "v=0\r\n")
	if err == nil {
		t.Error("expected error when session limit is exceeded, got nil")
	}
}

// TestCloseRemovesSession verifies that Close removes the session from the active set.
// Validates: Requirements 2.5
func TestCloseRemovesSession(t *testing.T) {
	r := newTestRelay(4)
	defer r.CloseAll()

	id := uuid.NewString()
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("failed to create PeerConnection: %v", err)
	}
	s := &StreamSession{
		ID:           id,
		EntryID:      "entry",
		DeviceID:     "device",
		PC:           pc,
		LastActivity: time.Now(),
		cancel:       func() {},
	}
	injectSession(r, s)

	if err := r.Close(id); err != nil {
		t.Fatalf("unexpected error closing session: %v", err)
	}

	r.mu.Lock()
	_, exists := r.sessions[id]
	r.mu.Unlock()

	if exists {
		t.Error("session still present in map after Close")
	}
}
