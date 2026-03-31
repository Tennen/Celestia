package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
)

// StreamSession holds the state for a single active WebRTC/RTSP relay session.
type StreamSession struct {
	ID           string
	EntryID      string
	DeviceID     string
	PC           *webrtc.PeerConnection
	RTSPClient   *gortsplib.Client
	LastActivity time.Time
	cancel       context.CancelFunc
}

// RTSPRelay manages all active stream sessions for the plugin.
type RTSPRelay struct {
	mu          sync.Mutex
	sessions    map[string]*StreamSession
	maxSessions int
	idleTimeout time.Duration
	emitEvent   func(models.Event)
	stopCleanup context.CancelFunc
}

// NewRTSPRelay creates a new RTSPRelay and starts the idle cleanup loop.
func NewRTSPRelay(maxSessions int, idleTimeout time.Duration, emit func(models.Event)) *RTSPRelay {
	ctx, cancel := context.WithCancel(context.Background())
	r := &RTSPRelay{
		sessions:    make(map[string]*StreamSession),
		maxSessions: maxSessions,
		idleTimeout: idleTimeout,
		emitEvent:   emit,
		stopCleanup: cancel,
	}
	go r.startCleanupLoop(ctx)
	return r
}

// Offer opens an RTSP connection to the camera, creates a WebRTC PeerConnection,
// and returns a session ID and SDP answer. Credentials are never included in return values.
func (r *RTSPRelay) Offer(ctx context.Context, entryID string, deviceID string, cfg CameraConfig, sdpOffer string) (sessionID string, sdpAnswer string, err error) {
	r.mu.Lock()
	if len(r.sessions) >= r.maxSessions {
		r.mu.Unlock()
		return "", "", fmt.Errorf("session limit reached: max %d concurrent streams", r.maxSessions)
	}
	r.mu.Unlock()

	// Construct RTSP URL with credentials (never returned to caller)
	rtspPath := strings.ReplaceAll(cfg.RTSPPath, "{channel}", fmt.Sprintf("%d", cfg.Channel))
	rtspURL := fmt.Sprintf("rtsp://%s:%s@%s:%d%s", cfg.Username, cfg.Password, cfg.Host, cfg.RTSPPort, rtspPath)

	parsedURL, err := base.ParseURL(rtspURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid RTSP configuration for %s:%d: %w", cfg.Host, cfg.RTSPPort, err)
	}

	// Open RTSP client
	client := &gortsplib.Client{}
	if err := client.Start(parsedURL.Scheme, parsedURL.Host); err != nil {
		return "", "", fmt.Errorf("RTSP unreachable at %s:%d: %w", cfg.Host, cfg.RTSPPort, err)
	}

	// Describe the stream
	desc, _, err := client.Describe(parsedURL)
	if err != nil {
		client.Close()
		return "", "", fmt.Errorf("RTSP DESCRIBE failed at %s:%d: %w", cfg.Host, cfg.RTSPPort, err)
	}

	// Find video and audio formats
	videoMedia, videoFmt, audioMedia, audioFmt := findRTSPFormats(desc)
	if videoMedia == nil {
		client.Close()
		return "", "", fmt.Errorf("no supported video track (H.264/H.265) at %s:%d", cfg.Host, cfg.RTSPPort)
	}

	// Setup video media
	if _, err := client.Setup(desc.BaseURL, videoMedia, 0, 0); err != nil {
		client.Close()
		return "", "", fmt.Errorf("RTSP SETUP failed at %s:%d: %w", cfg.Host, cfg.RTSPPort, err)
	}

	// Setup audio media if present
	if audioMedia != nil {
		if _, err := client.Setup(desc.BaseURL, audioMedia, 0, 0); err != nil {
			// Audio is optional; continue without it
			audioMedia = nil
			audioFmt = nil
		}
	}

	// Create WebRTC PeerConnection with default codecs registered.
	// webrtc.NewPeerConnection uses a shared default MediaEngine that has no
	// codecs registered; we must build our own API with RegisterDefaultCodecs
	// so that SetRemoteDescription can match the browser's offered codecs.
	me := &webrtc.MediaEngine{}
	if err := me.RegisterDefaultCodecs(); err != nil {
		client.Close()
		return "", "", fmt.Errorf("failed to register default codecs: %w", err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(me))
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		client.Close()
		return "", "", fmt.Errorf("failed to create PeerConnection: %w", err)
	}

	// Add video track
	videoTrack, err := addWebRTCVideoTrack(pc, videoFmt)
	if err != nil {
		_ = pc.Close()
		client.Close()
		return "", "", fmt.Errorf("failed to add video track: %w", err)
	}

	// Add audio track if present
	var audioTrack *webrtc.TrackLocalStaticRTP
	if audioMedia != nil && audioFmt != nil {
		audioTrack, _ = addWebRTCAudioTrack(pc, audioFmt)
		// Audio failure is non-fatal
	}

	// Set remote description from SDP offer first (required before CreateAnswer)
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdpOffer,
	}); err != nil {
		_ = pc.Close()
		client.Close()
		return "", "", fmt.Errorf("failed to set remote description: %w", err)
	}

	// Produce SDP answer with 10s timeout.
	// GatheringCompletePromise must be called before SetLocalDescription.
	answerCtx, answerCancel := context.WithTimeout(ctx, 10*time.Second)
	defer answerCancel()

	answer, err := produceSDP(answerCtx, pc)
	if err != nil {
		_ = pc.Close()
		client.Close()
		return "", "", fmt.Errorf("failed to produce SDP answer: %w", err)
	}

	sessionID = uuid.NewString()
	sessCtx, sessCancel := context.WithCancel(context.Background())

	session := &StreamSession{
		ID:           sessionID,
		EntryID:      entryID,
		DeviceID:     deviceID,
		PC:           pc,
		RTSPClient:   client,
		LastActivity: time.Now(),
		cancel:       sessCancel,
	}

	// Register ICE candidate and connection state callbacks before PLAY
	// so no candidates are missed during the RTSP handshake.
	r.registerICECallbacks(session)

	// Start RTSP PLAY and RTP forwarding
	if _, err := client.Play(nil); err != nil {
		sessCancel()
		_ = pc.Close()
		client.Close()
		return "", "", fmt.Errorf("RTSP PLAY failed at %s:%d: %w", cfg.Host, cfg.RTSPPort, err)
	}

	go r.forwardRTP(sessCtx, session, client, videoMedia, videoFmt, videoTrack, audioMedia, audioFmt, audioTrack)

	r.mu.Lock()
	r.sessions[sessionID] = session
	r.mu.Unlock()

	return sessionID, answer.SDP, nil
}

// produceSDP creates an SDP answer and waits for ICE gathering to complete or times out.
// GatheringCompletePromise must be called before SetLocalDescription.
func produceSDP(ctx context.Context, pc *webrtc.PeerConnection) (webrtc.SessionDescription, error) {
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}

	// Register the gathering-complete channel BEFORE setting local description,
	// otherwise the signal may fire before we start listening.
	gatherDone := webrtc.GatheringCompletePromise(pc)

	if err := pc.SetLocalDescription(answer); err != nil {
		return webrtc.SessionDescription{}, err
	}

	select {
	case <-gatherDone:
		return *pc.LocalDescription(), nil
	case <-ctx.Done():
		return webrtc.SessionDescription{}, errors.New("SDP answer timed out after 10s")
	}
}

// Close closes the PeerConnection and RTSP client for the given session.
func (r *RTSPRelay) Close(sessionID string) error {
	r.mu.Lock()
	session, ok := r.sessions[sessionID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("session not found: %s", sessionID)
	}
	delete(r.sessions, sessionID)
	r.mu.Unlock()

	closeSession(session)
	return nil
}

// AddICECandidate delivers a remote ICE candidate to the PeerConnection.
func (r *RTSPRelay) AddICECandidate(sessionID string, candidate string) error {
	r.mu.Lock()
	session, ok := r.sessions[sessionID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.LastActivity = time.Now()
	return session.PC.AddICECandidate(webrtc.ICECandidateInit{Candidate: candidate})
}

// CloseAll closes every active session (called on plugin Stop).
func (r *RTSPRelay) CloseAll() {
	r.stopCleanup()

	r.mu.Lock()
	sessions := make([]*StreamSession, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, s)
	}
	r.sessions = make(map[string]*StreamSession)
	r.mu.Unlock()

	for _, s := range sessions {
		closeSession(s)
	}
}

// startCleanupLoop ticks at ≤15s intervals and closes idle sessions.
func (r *RTSPRelay) startCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.cleanupIdleSessions()
		}
	}
}

func (r *RTSPRelay) cleanupIdleSessions() {
	now := time.Now()
	r.mu.Lock()
	var toClose []*StreamSession
	for id, s := range r.sessions {
		if now.Sub(s.LastActivity) > r.idleTimeout {
			toClose = append(toClose, s)
			delete(r.sessions, id)
		}
	}
	r.mu.Unlock()

	for _, s := range toClose {
		closeSession(s)
		r.emitEvent(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceOccurred,
			PluginID: pluginID,
			DeviceID: s.DeviceID,
			TS:       time.Now().UTC(),
			Payload: map[string]any{
				"event_type": "stream_timeout",
				"session_id": s.ID,
			},
		})
	}
}

// registerICECallbacks wires up ICE candidate emission and connection state handling.
func (r *RTSPRelay) registerICECallbacks(session *StreamSession) {
	session.PC.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		r.emitEvent(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceOccurred,
			PluginID: pluginID,
			DeviceID: session.DeviceID,
			TS:       time.Now().UTC(),
			Payload: map[string]any{
				"event_type": "ice_candidate",
				"session_id": session.ID,
				"candidate":  c.ToJSON().Candidate,
			},
		})
	})

	session.PC.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateClosed {
			r.mu.Lock()
			_, exists := r.sessions[session.ID]
			if exists {
				delete(r.sessions, session.ID)
			}
			r.mu.Unlock()

			if exists {
				closeSession(session)
				r.emitEvent(models.Event{
					ID:       uuid.NewString(),
					Type:     models.EventDeviceOccurred,
					PluginID: pluginID,
					DeviceID: session.DeviceID,
					TS:       time.Now().UTC(),
					Payload: map[string]any{
						"event_type": "stream_disconnected",
						"session_id": session.ID,
					},
				})
			}
		}
	})
}

// closeSession tears down a session's PeerConnection and RTSP client.
func closeSession(s *StreamSession) {
	if s.cancel != nil {
		s.cancel()
	}
	if s.PC != nil {
		_ = s.PC.Close()
	}
	if s.RTSPClient != nil {
		s.RTSPClient.Close()
	}
}

// findRTSPFormats scans the RTSP session description for H.264/H.265 video
// and G.711/MPEG4-Audio audio formats.
func findRTSPFormats(desc *description.Session) (
	videoMedia *description.Media, videoFmt format.Format,
	audioMedia *description.Media, audioFmt format.Format,
) {
	// Try H.264 first
	var h264 *format.H264
	if m := desc.FindFormat(&h264); m != nil {
		videoMedia = m
		videoFmt = h264
	}

	// Try H.265 if no H.264
	if videoMedia == nil {
		var h265 *format.H265
		if m := desc.FindFormat(&h265); m != nil {
			videoMedia = m
			videoFmt = h265
		}
	}

	// Try G.711 audio
	var g711 *format.G711
	if m := desc.FindFormat(&g711); m != nil {
		audioMedia = m
		audioFmt = g711
	}

	// Try MPEG4-Audio if no G.711
	if audioMedia == nil {
		var mpeg4a *format.MPEG4Audio
		if m := desc.FindFormat(&mpeg4a); m != nil {
			audioMedia = m
			audioFmt = mpeg4a
		}
	}

	return
}
