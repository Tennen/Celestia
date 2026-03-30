package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (p *Plugin) executeCommand(ctx context.Context, runtime *entryRuntime, req models.CommandRequest) (map[string]any, string, error) {
	params := req.Params
	switch req.Action {
	case "ptz_move":
		direction := strings.ToLower(stringParam(params, "direction"))
		if direction == "" {
			return nil, "", errors.New("direction is required")
		}
		speed := intParam(params, "speed", runtime.Config.PTZDefaultSpeed)
		duration := intParam(params, "duration_ms", runtime.Config.PTZStepMS)
		if err := runtime.Client.PTZMove(ctx, direction, speed, duration); err != nil {
			return nil, "", err
		}
		return nil, "ptz move accepted", nil
	case "ptz_stop":
		direction := strings.ToLower(stringParam(params, "direction"))
		if direction == "" {
			return nil, "", errors.New("direction is required")
		}
		speed := intParam(params, "speed", runtime.Config.PTZDefaultSpeed)
		if err := runtime.Client.PTZStop(ctx, direction, speed); err != nil {
			return nil, "", err
		}
		return nil, "ptz stop accepted", nil
	case "ptz_up":
		return p.shortPTZ(ctx, runtime, "up")
	case "ptz_down":
		return p.shortPTZ(ctx, runtime, "down")
	case "ptz_left":
		return p.shortPTZ(ctx, runtime, "left")
	case "ptz_right":
		return p.shortPTZ(ctx, runtime, "right")
	case "ptz_zoom_in":
		return p.shortPTZ(ctx, runtime, "zoom_in")
	case "ptz_zoom_out":
		return p.shortPTZ(ctx, runtime, "zoom_out")
	case "playback_open":
		start := stringParam(params, "start")
		end := stringParam(params, "end")
		if start == "" || end == "" {
			return nil, "", errors.New("start and end are required")
		}
		startAt, err := parseISOTime(start)
		if err != nil {
			return nil, "", fmt.Errorf("invalid start: %w", err)
		}
		endAt, err := parseISOTime(end)
		if err != nil {
			return nil, "", fmt.Errorf("invalid end: %w", err)
		}
		result, err := runtime.Client.PlaybackOpen(ctx, startAt, endAt)
		if err != nil {
			return nil, "", err
		}
		message := "playback session opened"
		if sessionID, ok := result["session_id"].(string); ok && strings.TrimSpace(sessionID) != "" {
			message = "playback session " + strings.TrimSpace(sessionID) + " opened"
		}
		return result, message, nil
	case "playback_control":
		sessionID := stringParam(params, "session_id")
		action := strings.ToLower(stringParam(params, "action"))
		if sessionID == "" {
			return nil, "", errors.New("session_id is required")
		}
		if action == "" {
			return nil, "", errors.New("action is required")
		}
		return p.playbackControl(ctx, runtime, sessionID, action, params)
	case "playback_play":
		sessionID := stringParam(params, "session_id")
		if sessionID == "" {
			return nil, "", errors.New("session_id is required")
		}
		return p.playbackControl(ctx, runtime, sessionID, "play", params)
	case "playback_pause":
		sessionID := stringParam(params, "session_id")
		if sessionID == "" {
			return nil, "", errors.New("session_id is required")
		}
		return p.playbackControl(ctx, runtime, sessionID, "pause", params)
	case "playback_seek":
		sessionID := stringParam(params, "session_id")
		if sessionID == "" {
			return nil, "", errors.New("session_id is required")
		}
		return p.playbackControl(ctx, runtime, sessionID, "seek", params)
	case "playback_close":
		sessionID := stringParam(params, "session_id")
		if sessionID == "" {
			return nil, "", errors.New("session_id is required")
		}
		result, err := runtime.Client.PlaybackClose(ctx, sessionID)
		if err != nil {
			return nil, "", err
		}
		return result, "playback session closed", nil
	case "list_recordings":
		dateValue := stringParam(params, "date")
		if dateValue == "" {
			return nil, "", errors.New("date is required (YYYY-MM-DD)")
		}
		slotMinutes := intParam(params, "slot_minutes", 60)
		if slotMinutes < 5 || slotMinutes > 60 {
			slotMinutes = 60
		}
		day, err := time.Parse("2006-01-02", dateValue)
		if err != nil {
			return nil, "", errors.New("date format must be YYYY-MM-DD")
		}
		recordings, err := runtime.Client.ListRecordings(ctx, day, slotMinutes)
		if err != nil {
			return nil, "", err
		}
		payload := map[string]any{
			"entry_id":   runtime.Config.EntryID,
			"date":       day.Format("2006-01-02"),
			"count":      len(recordings),
			"recordings": recordings,
		}
		return payload, fmt.Sprintf("recordings listed: %d", len(recordings)), nil
	default:
		return nil, "", fmt.Errorf("unsupported action %q", req.Action)
	}
}

func (p *Plugin) shortPTZ(ctx context.Context, runtime *entryRuntime, direction string) (map[string]any, string, error) {
	if err := runtime.Client.PTZMove(ctx, direction, runtime.Config.PTZDefaultSpeed, runtime.Config.PTZStepMS); err != nil {
		return nil, "", err
	}
	return nil, "ptz move accepted", nil
}

func (p *Plugin) playbackControl(
	ctx context.Context,
	runtime *entryRuntime,
	sessionID string,
	action string,
	params map[string]any,
) (map[string]any, string, error) {
	allowed := map[string]bool{"play": true, "pause": true, "seek": true}
	if !allowed[action] {
		return nil, "", fmt.Errorf("unsupported playback action %q", action)
	}
	var seekPercent *float64
	if action == "seek" {
		value, ok := floatParam(params, "seek_percent")
		if !ok {
			return nil, "", errors.New("seek_percent is required for seek")
		}
		seekPercent = &value
	}
	result, err := runtime.Client.PlaybackControl(ctx, sessionID, action, seekPercent)
	if err != nil {
		return nil, "", err
	}
	return result, "playback control accepted", nil
}
