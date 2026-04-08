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
	if p.config.Mode == RuntimeModeCloud {
		return p.executeCloudCommand(ctx, runtime, req)
	}
	return p.executeLANCommand(ctx, runtime, req)
}

func (p *Plugin) executeLANCommand(ctx context.Context, runtime *entryRuntime, req models.CommandRequest) (map[string]any, string, error) {
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
	case "stream_rtsp_url":
		return p.handleStreamRTSPURL(runtime)
	default:
		return nil, "", fmt.Errorf("unsupported action %q", req.Action)
	}
}

func (p *Plugin) executeCloudCommand(ctx context.Context, runtime *entryRuntime, req models.CommandRequest) (map[string]any, string, error) {
	params := req.Params
	switch req.Action {
	case "ptz_move":
		direction := strings.ToLower(stringParam(params, "direction"))
		if direction == "" {
			return nil, "", errors.New("direction is required")
		}
		return p.cloudPTZMove(ctx, runtime, direction, intParam(params, "speed", runtime.Config.PTZDefaultSpeed), intParam(params, "duration_ms", runtime.Config.PTZStepMS))
	case "ptz_stop":
		direction := strings.ToLower(stringParam(params, "direction"))
		if direction == "" {
			return nil, "", errors.New("direction is required")
		}
		return p.cloudPTZStop(ctx, runtime, direction, intParam(params, "speed", runtime.Config.PTZDefaultSpeed))
	case "ptz_up":
		return p.shortPTZ(ctx, runtime, "up")
	case "ptz_down":
		return p.shortPTZ(ctx, runtime, "down")
	case "ptz_left":
		return p.shortPTZ(ctx, runtime, "left")
	case "ptz_right":
		return p.shortPTZ(ctx, runtime, "right")
	case "stream_rtsp_url":
		return p.handleStreamRTSPURL(runtime)
	case "playback_open", "playback_control", "playback_play", "playback_pause", "playback_seek", "playback_close", "list_recordings", "ptz_zoom_in", "ptz_zoom_out":
		return nil, "", fmt.Errorf("%s is not supported in hikvision cloud mode", req.Action)
	default:
		return nil, "", fmt.Errorf("unsupported action %q", req.Action)
	}
}

func (p *Plugin) shortPTZ(ctx context.Context, runtime *entryRuntime, direction string) (map[string]any, string, error) {
	if p.config.Mode == RuntimeModeCloud {
		return p.cloudPTZMove(ctx, runtime, direction, runtime.Config.PTZDefaultSpeed, runtime.Config.PTZStepMS)
	}
	if err := runtime.Client.PTZMove(ctx, direction, runtime.Config.PTZDefaultSpeed, runtime.Config.PTZStepMS); err != nil {
		return nil, "", err
	}
	return nil, "ptz move accepted", nil
}

func (p *Plugin) cloudPTZMove(ctx context.Context, runtime *entryRuntime, direction string, speed, durationMS int) (map[string]any, string, error) {
	command, err := cloudPTZCommand(direction)
	if err != nil {
		return nil, "", err
	}
	if err := p.ensureCloudPTZAvailable(runtime); err != nil {
		return nil, "", err
	}
	if speed < 1 {
		speed = runtime.Config.PTZDefaultSpeed
	}
	if speed > 10 {
		speed = 10
	}
	if durationMS < 50 {
		durationMS = runtime.Config.PTZStepMS
	}
	if err := p.cloud.PTZMove(ctx, runtime.Config.DeviceSerial, command, speed, time.Duration(durationMS)*time.Millisecond); err != nil {
		return nil, "", err
	}
	return nil, "ptz move accepted", nil
}

func (p *Plugin) cloudPTZStop(ctx context.Context, runtime *entryRuntime, direction string, speed int) (map[string]any, string, error) {
	command, err := cloudPTZCommand(direction)
	if err != nil {
		return nil, "", err
	}
	if err := p.ensureCloudPTZAvailable(runtime); err != nil {
		return nil, "", err
	}
	if speed < 1 {
		speed = runtime.Config.PTZDefaultSpeed
	}
	if speed > 10 {
		speed = 10
	}
	if err := p.cloud.PTZStop(ctx, runtime.Config.DeviceSerial, command, speed); err != nil {
		return nil, "", err
	}
	return nil, "ptz stop accepted", nil
}

func (p *Plugin) ensureCloudPTZAvailable(runtime *entryRuntime) error {
	if runtime == nil {
		return errors.New("device runtime is unavailable")
	}
	if runtime.ControlBlocked != "" {
		return errors.New(runtime.ControlBlocked)
	}
	if runtime.Config.DeviceSerial == "" {
		return errors.New("device_serial is required for cloud PTZ")
	}
	if p.cloud == nil {
		return errors.New("ezviz cloud auth is not configured")
	}
	return nil
}

func cloudPTZCommand(direction string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "up":
		return "UP", nil
	case "down":
		return "DOWN", nil
	case "left":
		return "LEFT", nil
	case "right":
		return "RIGHT", nil
	default:
		return "", fmt.Errorf("unsupported PTZ direction %q in cloud mode", direction)
	}
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
