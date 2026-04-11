package stream

import (
	"context"
	"errors"

	"github.com/pion/webrtc/v3"
)

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
