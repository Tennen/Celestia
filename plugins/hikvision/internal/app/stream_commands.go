package app

import (
	"context"
	"errors"
)

// handleStreamOffer extracts the SDP offer from params, calls the relay Offer method,
// and returns the session_id and SDP answer.
func (p *Plugin) handleStreamOffer(ctx context.Context, runtime *entryRuntime, params map[string]any) (map[string]any, string, error) {
	sdp := stringParam(params, "sdp")
	if sdp == "" {
		return nil, "", errors.New("sdp is required")
	}

	sessionID, sdpAnswer, err := p.relay.Offer(ctx, runtime.Config.EntryID, runtime.Device.ID, runtime.Config, sdp)
	if err != nil {
		return nil, "", err
	}

	return map[string]any{
		"session_id": sessionID,
		"sdp":        sdpAnswer,
	}, "stream session opened", nil
}

// handleStreamClose extracts the session_id from params and closes the relay session.
func (p *Plugin) handleStreamClose(params map[string]any) (map[string]any, string, error) {
	sessionID := stringParam(params, "session_id")
	if sessionID == "" {
		return nil, "", errors.New("session_id is required")
	}

	if err := p.relay.Close(sessionID); err != nil {
		return nil, "", err
	}

	return map[string]any{"closed": true}, "stream session closed", nil
}

// handleStreamICE extracts session_id and candidate from params and delivers the ICE candidate.
func (p *Plugin) handleStreamICE(params map[string]any) (map[string]any, string, error) {
	sessionID := stringParam(params, "session_id")
	if sessionID == "" {
		return nil, "", errors.New("session_id is required")
	}

	candidate := stringParam(params, "candidate")
	if candidate == "" {
		return nil, "", errors.New("candidate is required")
	}

	if err := p.relay.AddICECandidate(sessionID, candidate); err != nil {
		return nil, "", err
	}

	return map[string]any{"accepted": true}, "ICE candidate accepted", nil
}
