// Package stream owns the WebRTC/RTSP relay that runs inside the core (gateway)
// process. Keeping it here means the relay binds to the host network directly,
// so ICE candidates are reachable by browsers without any UDP port-mapping
// through a container runtime (Colima, Docker Desktop, etc.).
//
// The relay is intentionally decoupled from the plugin system: it receives a
// plain RTSP URL (with credentials) from the plugin via a "stream_rtsp_url"
// command and handles all WebRTC negotiation itself.
package stream

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph264"
	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/google/uuid"
	pionice "github.com/pion/ice/v2"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

const defaultIdleTimeout = 60 * time.Second

// Session holds the state for one active WebRTC/RTSP relay session.
type Session struct {
	ID           string
	DeviceID     string
	PC           *webrtc.PeerConnection
	rtspClient   *gortsplib.Client
	lastActivity time.Time
	cancel       context.CancelFunc
}

// Relay manages WebRTC/RTSP relay sessions inside the core process.
type Relay struct {
	mu          sync.Mutex
	sessions    map[string]*Session
	maxSessions int
	idleTimeout time.Duration
	natIP       string // host LAN IP announced in ICE candidates
	tcpPort     int    // TCP port for ICE-TCP (0 = use UDP instead)
	nextPort    uint16 // UDP port counter (50000–59999), unused when tcpPort>0
	tcpMux      pionice.TCPMux
	stopCleanup context.CancelFunc
}

// New creates a Relay and starts the idle-session cleanup loop.
// natIP is the LAN IP of the host machine to announce in ICE candidates.
// tcpPort, when >0, enables ICE-TCP on that port (required when UDP port
// mapping is unavailable, e.g. Colima on macOS).
func New(maxSessions int, idleTimeout time.Duration, natIP string, tcpPort int) *Relay {
	if maxSessions <= 0 {
		maxSessions = 4
	}
	if idleTimeout <= 0 {
		idleTimeout = defaultIdleTimeout
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &Relay{
		sessions:    make(map[string]*Session),
		maxSessions: maxSessions,
		idleTimeout: idleTimeout,
		natIP:       strings.TrimSpace(natIP),
		tcpPort:     tcpPort,
		nextPort:    50000,
		stopCleanup: cancel,
	}
	if tcpPort > 0 {
		tcpListener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4zero, Port: tcpPort})
		if err != nil {
			log.Printf("[stream] WARNING: failed to listen on TCP %d for ICE-TCP: %v", tcpPort, err)
		} else {
			r.tcpMux = pionice.NewTCPMuxDefault(pionice.TCPMuxParams{Listener: tcpListener})
			log.Printf("[stream] ICE-TCP enabled on port %d", tcpPort)
		}
	}
	go r.cleanupLoop(ctx)
	return r
}

// NatIP returns the configured NAT IP (for diagnostics).
func (r *Relay) NatIP() string { return r.natIP }

// allocPort returns the next UDP port in the 50000–59999 range.
func (r *Relay) allocPort() uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.nextPort
	r.nextPort++
	if r.nextPort > 59999 {
		r.nextPort = 50000
	}
	return p
}

// Offer opens an RTSP connection to rtspURL, creates a WebRTC PeerConnection,
// and returns a session ID and SDP answer.
// rtspURL must include credentials (rtsp://user:pass@host:port/path).
// The URL is consumed locally and never forwarded to the browser.
func (r *Relay) Offer(ctx context.Context, deviceID, rtspURL, sdpOffer string) (sessionID, sdpAnswer string, err error) {
	r.mu.Lock()
	if len(r.sessions) >= r.maxSessions {
		r.mu.Unlock()
		return "", "", fmt.Errorf("session limit reached (%d)", r.maxSessions)
	}
	r.mu.Unlock()

	parsedURL, err := base.ParseURL(rtspURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid RTSP URL: %w", err)
	}

	client := &gortsplib.Client{}
	if err := client.Start(parsedURL.Scheme, parsedURL.Host); err != nil {
		return "", "", fmt.Errorf("RTSP connect failed: %w", err)
	}

	desc, _, err := client.Describe(parsedURL)
	if err != nil {
		client.Close()
		return "", "", fmt.Errorf("RTSP DESCRIBE failed: %w", err)
	}

	videoMedia, videoFmt, audioMedia, audioFmt := findFormats(desc)
	if videoMedia == nil {
		client.Close()
		return "", "", errors.New("no H264 video track in RTSP stream")
	}

	if _, err := client.Setup(desc.BaseURL, videoMedia, 0, 0); err != nil {
		client.Close()
		return "", "", fmt.Errorf("RTSP SETUP failed: %w", err)
	}
	if audioMedia != nil {
		if _, err := client.Setup(desc.BaseURL, audioMedia, 0, 0); err != nil {
			audioMedia = nil
			audioFmt = nil
		}
	}

	me := &webrtc.MediaEngine{}
	if err := me.RegisterDefaultCodecs(); err != nil {
		client.Close()
		return "", "", fmt.Errorf("codec registration: %w", err)
	}

	se := webrtc.SettingEngine{}
	if r.tcpMux != nil {
		// ICE-TCP mode: use the shared TCP listener, announce host LAN IP.
		// This works reliably through Colima/Docker TCP port mapping.
		se.SetICETCPMux(r.tcpMux)
		if r.natIP != "" {
			se.SetNAT1To1IPs([]string{r.natIP}, webrtc.ICECandidateTypeHost)
		}
		// Disable UDP gathering so only TCP candidates are generated.
		se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeTCP4})
	} else if r.natIP != "" {
		// UDP NAT mode: bind on 0.0.0.0 at a specific port.
		port := r.allocPort()
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: int(port)})
		if err != nil {
			client.Close()
			return "", "", fmt.Errorf("UDP listen on port %d: %w", port, err)
		}
		udpMux := pionice.NewUDPMuxDefault(pionice.UDPMuxParams{UDPConn: conn})
		se.SetICEUDPMux(udpMux)
		se.SetNAT1To1IPs([]string{r.natIP}, webrtc.ICECandidateTypeHost)
	} else {
		// No NAT IP, no TCP port: enumerate local interfaces directly.
		se.SetICEMulticastDNSMode(pionice.MulticastDNSModeQueryOnly)
		se.SetIPFilter(isLANCandidate)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(me), webrtc.WithSettingEngine(se))
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		client.Close()
		return "", "", fmt.Errorf("PeerConnection: %w", err)
	}

	// Normalize SDP line endings to CRLF (RFC 4566).
	offer := strings.ReplaceAll(sdpOffer, "\r\n", "\n")
	offer = strings.ReplaceAll(offer, "\n", "\r\n")
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
		_ = pc.Close()
		client.Close()
		return "", "", fmt.Errorf("SetRemoteDescription: %w", err)
	}

	videoTrack, err := addVideoTrack(pc, videoFmt)
	if err != nil {
		_ = pc.Close()
		client.Close()
		return "", "", fmt.Errorf("add video track: %w", err)
	}

	var audioTrack *webrtc.TrackLocalStaticRTP
	if audioMedia != nil && audioFmt != nil {
		if t, e := addAudioTrack(pc, audioFmt); e == nil {
			audioTrack = t
		} else {
			audioMedia = nil
			audioFmt = nil
		}
	}

	sessionID = uuid.NewString()
	sessCtx, sessCancel := context.WithCancel(context.Background())
	session := &Session{
		ID:           sessionID,
		DeviceID:     deviceID,
		PC:           pc,
		rtspClient:   client,
		lastActivity: time.Now(),
		cancel:       sessCancel,
	}

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("[stream] session=%s ICE state -> %s", sessionID[:8], state.String())
		if state == webrtc.ICEConnectionStateFailed || state == webrtc.ICEConnectionStateClosed {
			r.mu.Lock()
			if _, ok := r.sessions[sessionID]; ok {
				delete(r.sessions, sessionID)
				closeSession(session)
			}
			r.mu.Unlock()
		}
	})

	answerCtx, answerCancel := context.WithTimeout(ctx, 10*time.Second)
	defer answerCancel()

	answer, err := produceSDP(answerCtx, pc)
	if err != nil {
		sessCancel()
		_ = pc.Close()
		client.Close()
		return "", "", fmt.Errorf("SDP answer: %w", err)
	}

	if _, err := client.Play(nil); err != nil {
		sessCancel()
		_ = pc.Close()
		client.Close()
		return "", "", fmt.Errorf("RTSP PLAY: %w", err)
	}

	log.Printf("[stream] session=%s RTSP PLAY started nat_ip=%q", sessionID[:8], r.natIP)
	go forwardRTP(sessCtx, session, client, videoMedia, videoFmt, videoTrack, audioMedia, audioFmt, audioTrack)

	r.mu.Lock()
	r.sessions[sessionID] = session
	r.mu.Unlock()

	return sessionID, answer.SDP, nil
}

// Close tears down the session.
func (r *Relay) Close(sessionID string) error {
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

// CloseAll closes every active session (called on gateway shutdown).
func (r *Relay) CloseAll() {
	r.stopCleanup()
	r.mu.Lock()
	sessions := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		sessions = append(sessions, s)
	}
	r.sessions = make(map[string]*Session)
	r.mu.Unlock()
	for _, s := range sessions {
		closeSession(s)
	}
	if r.tcpMux != nil {
		_ = r.tcpMux.Close()
	}
}

func (r *Relay) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			r.mu.Lock()
			for id, s := range r.sessions {
				if now.Sub(s.lastActivity) > r.idleTimeout {
					delete(r.sessions, id)
					go closeSession(s)
				}
			}
			r.mu.Unlock()
		}
	}
}

func produceSDP(ctx context.Context, pc *webrtc.PeerConnection) (webrtc.SessionDescription, error) {
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return webrtc.SessionDescription{}, err
	}
	gatherDone := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		return webrtc.SessionDescription{}, err
	}
	select {
	case <-gatherDone:
		return *pc.LocalDescription(), nil
	case <-ctx.Done():
		return webrtc.SessionDescription{}, errors.New("SDP answer timed out")
	}
}

func closeSession(s *Session) {
	if s.cancel != nil {
		s.cancel()
	}
	if s.PC != nil {
		_ = s.PC.Close()
	}
	if s.rtspClient != nil {
		s.rtspClient.Close()
	}
}

func forwardRTP(
	ctx context.Context,
	session *Session,
	client *gortsplib.Client,
	videoMedia *description.Media, videoFmt format.Format, videoTrack *webrtc.TrackLocalStaticRTP,
	audioMedia *description.Media, audioFmt format.Format, audioTrack *webrtc.TrackLocalStaticRTP,
) {
	var videoCount uint64

	h264Fmt, isH264 := videoFmt.(*format.H264)

	if isH264 {
		// Decode RTP → NALUs, inject SPS/PPS before IDR, re-encode → RTP.
		rtpDec, err := h264Fmt.CreateDecoder()
		if err != nil {
			log.Printf("[stream] session=%s H264 decoder create error: %v", session.ID[:8], err)
			isH264 = false
		}

		rtpEnc := &rtph264.Encoder{
			PayloadType:       h264Fmt.PayloadType(),
			PacketizationMode: h264Fmt.PacketizationMode,
		}
		if err := rtpEnc.Init(); err != nil {
			log.Printf("[stream] session=%s H264 encoder init error: %v", session.ID[:8], err)
			isH264 = false
		}

		if isH264 {
			client.OnPacketRTP(videoMedia, videoFmt, func(pkt *rtp.Packet) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				session.lastActivity = time.Now()

				nalus, err := rtpDec.Decode(pkt)
				if err != nil || len(nalus) == 0 {
					return
				}

				// Check if this access unit contains an IDR (keyframe).
				hasIDR := false
				for _, nalu := range nalus {
					if len(nalu) > 0 && h264.NALUType(nalu[0]&0x1f) == h264.NALUTypeIDR {
						hasIDR = true
						break
					}
				}

				// Prepend SPS+PPS before every IDR so the browser decoder can
				// initialize without needing out-of-band parameter sets.
				if hasIDR {
					var withParams [][]byte
					if len(h264Fmt.SPS) > 0 {
						withParams = append(withParams, h264Fmt.SPS)
					}
					if len(h264Fmt.PPS) > 0 {
						withParams = append(withParams, h264Fmt.PPS)
					}
					nalus = append(withParams, nalus...)
				}

				pkts, err := rtpEnc.Encode(nalus)
				if err != nil {
					return
				}

				for _, p := range pkts {
					p.Timestamp = pkt.Timestamp
					if raw, err := p.Marshal(); err == nil {
						if _, werr := videoTrack.Write(raw); werr != nil && videoCount == 0 {
							log.Printf("[stream] session=%s first video write error: %v", session.ID[:8], werr)
						}
						videoCount++
						if videoCount == 1 || videoCount%300 == 0 {
							log.Printf("[stream] session=%s video RTP count=%d pt=%d hasIDR=%v",
								session.ID[:8], videoCount, p.PayloadType, hasIDR)
						}
					}
				}
			})
		}
	}

	if !isH264 {
		// Fallback: direct RTP relay without re-packetization.
		client.OnPacketRTP(videoMedia, videoFmt, func(pkt *rtp.Packet) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			session.lastActivity = time.Now()
			if raw, err := pkt.Marshal(); err == nil {
				if _, werr := videoTrack.Write(raw); werr != nil && videoCount == 0 {
					log.Printf("[stream] session=%s first video write error: %v", session.ID[:8], werr)
				}
				videoCount++
				if videoCount == 1 || videoCount%300 == 0 {
					log.Printf("[stream] session=%s video RTP count=%d pt=%d", session.ID[:8], videoCount, pkt.PayloadType)
				}
			}
		})
	}

	if audioMedia != nil && audioFmt != nil && audioTrack != nil {
		client.OnPacketRTP(audioMedia, audioFmt, func(pkt *rtp.Packet) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if raw, err := pkt.Marshal(); err == nil {
				_, _ = audioTrack.Write(raw)
			}
		})
	}

	log.Printf("[stream] session=%s forwardRTP started, calling client.Wait()", session.ID[:8])
	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		err := client.Wait()
		log.Printf("[stream] session=%s client.Wait() returned: %v (video=%d)", session.ID[:8], err, videoCount)
	}()

	select {
	case <-ctx.Done():
		client.Close()
		<-waitDone
	case <-waitDone:
	}
}

func findFormats(desc *description.Session) (
	videoMedia *description.Media, videoFmt format.Format,
	audioMedia *description.Media, audioFmt format.Format,
) {
	var h264 *format.H264
	if m := desc.FindFormat(&h264); m != nil {
		videoMedia, videoFmt = m, h264
	}
	var g711 *format.G711
	if m := desc.FindFormat(&g711); m != nil {
		audioMedia, audioFmt = m, g711
	}
	return
}

func addVideoTrack(pc *webrtc.PeerConnection, videoFmt format.Format) (*webrtc.TrackLocalStaticRTP, error) {
	mimeType := webrtc.MimeTypeH264
	clockRate := uint32(90000)
	if f, ok := videoFmt.(*format.H264); ok {
		clockRate = uint32(f.ClockRate())
	}
	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: mimeType, ClockRate: clockRate},
		"video", "rtsp-relay",
	)
	if err != nil {
		return nil, err
	}
	if _, err := pc.AddTrack(track); err != nil {
		return nil, err
	}
	return track, nil
}

func addAudioTrack(pc *webrtc.PeerConnection, audioFmt format.Format) (*webrtc.TrackLocalStaticRTP, error) {
	f, ok := audioFmt.(*format.G711)
	if !ok {
		return nil, fmt.Errorf("unsupported audio format: %T", audioFmt)
	}
	mimeType := "audio/PCMU"
	if !f.MULaw {
		mimeType = "audio/PCMA"
	}
	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: mimeType, ClockRate: uint32(f.ClockRate()), Channels: uint16(f.ChannelCount)},
		"audio", "rtsp-relay",
	)
	if err != nil {
		return nil, err
	}
	if _, err := pc.AddTrack(track); err != nil {
		return nil, err
	}
	return track, nil
}

// isLANCandidate returns true for IPv4 addresses plausibly reachable by a
// browser on the same LAN. Rejects loopback, link-local, and /30-or-smaller
// subnets (VPN tunnels, USB tethering).
func isLANCandidate(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	if ip4[0] == 127 || (ip4[0] == 169 && ip4[1] == 254) {
		return false
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return true
	}
	for _, iface := range ifaces {
		if iface.Flags&(net.FlagLoopback|net.FlagPointToPoint) != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || !ipNet.Contains(ip4) {
				continue
			}
			ones, _ := ipNet.Mask.Size()
			if ones >= 30 {
				return false
			}
		}
	}
	return true
}
