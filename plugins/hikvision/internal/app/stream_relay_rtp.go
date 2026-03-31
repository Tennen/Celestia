package app

import (
	"context"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

func now() time.Time { return time.Now().UTC() }

// addWebRTCVideoTrack adds a video track to the PeerConnection based on the RTSP format.
// Returns the created track for RTP forwarding.
func addWebRTCVideoTrack(pc *webrtc.PeerConnection, videoFmt format.Format) (*webrtc.TrackLocalStaticRTP, error) {
	var mimeType string
	var clockRate uint32

	switch f := videoFmt.(type) {
	case *format.H264:
		mimeType = webrtc.MimeTypeH264
		clockRate = uint32(f.ClockRate())
	case *format.H265:
		mimeType = webrtc.MimeTypeH265
		clockRate = uint32(f.ClockRate())
	default:
		mimeType = webrtc.MimeTypeH264
		clockRate = 90000
	}

	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: mimeType, ClockRate: clockRate},
		"video",
		"rtsp-relay",
	)
	if err != nil {
		return nil, err
	}

	if _, err := pc.AddTrack(track); err != nil {
		return nil, err
	}
	return track, nil
}

// addWebRTCAudioTrack adds an audio track to the PeerConnection based on the RTSP format.
func addWebRTCAudioTrack(pc *webrtc.PeerConnection, audioFmt format.Format) (*webrtc.TrackLocalStaticRTP, error) {
	var mimeType string
	var clockRate uint32
	var channels uint16

	switch f := audioFmt.(type) {
	case *format.G711:
		if f.MULaw {
			mimeType = "audio/PCMU"
		} else {
			mimeType = "audio/PCMA"
		}
		clockRate = uint32(f.ClockRate())
		channels = uint16(f.ChannelCount)
	case *format.MPEG4Audio:
		mimeType = webrtc.MimeTypeOpus
		clockRate = 48000
		channels = 2
	default:
		mimeType = webrtc.MimeTypeOpus
		clockRate = 48000
		channels = 2
	}

	track, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{
			MimeType:  mimeType,
			ClockRate: clockRate,
			Channels:  channels,
		},
		"audio",
		"rtsp-relay",
	)
	if err != nil {
		return nil, err
	}

	if _, err := pc.AddTrack(track); err != nil {
		return nil, err
	}
	return track, nil
}

// forwardRTP reads RTP packets from the RTSP client and writes them to WebRTC tracks.
// It runs until the session context is cancelled or the RTSP client closes.
func (r *RTSPRelay) forwardRTP(
	ctx context.Context,
	session *StreamSession,
	client *gortsplib.Client,
	videoMedia *description.Media,
	videoFmt format.Format,
	videoTrack *webrtc.TrackLocalStaticRTP,
	audioMedia *description.Media,
	audioFmt format.Format,
	audioTrack *webrtc.TrackLocalStaticRTP,
) {
	// Register video RTP callback
	client.OnPacketRTP(videoMedia, videoFmt, func(pkt *rtp.Packet) {
		select {
		case <-ctx.Done():
			return
		default:
		}
		session.LastActivity = session.LastActivity // touch without lock for perf
		raw, err := pkt.Marshal()
		if err != nil {
			return
		}
		_, _ = videoTrack.Write(raw)
	})

	// Register audio RTP callback if present
	if audioMedia != nil && audioFmt != nil && audioTrack != nil {
		client.OnPacketRTP(audioMedia, audioFmt, func(pkt *rtp.Packet) {
			select {
			case <-ctx.Done():
				return
			default:
			}
			raw, err := pkt.Marshal()
			if err != nil {
				return
			}
			_, _ = audioTrack.Write(raw)
		})
	}

	// Wait for context cancellation (session close or plugin stop)
	<-ctx.Done()

	// Emit stream_disconnected if session was still active when context was cancelled
	r.mu.Lock()
	_, stillActive := r.sessions[session.ID]
	if stillActive {
		delete(r.sessions, session.ID)
	}
	r.mu.Unlock()

	if stillActive {
		r.emitEvent(models.Event{
			ID:       uuid.NewString(),
			Type:     models.EventDeviceOccurred,
			PluginID: pluginID,
			DeviceID: session.DeviceID,
			TS:       now(),
			Payload: map[string]any{
				"event_type": "stream_disconnected",
				"session_id": session.ID,
			},
		})
	}
}
