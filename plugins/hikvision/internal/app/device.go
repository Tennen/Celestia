package app

import (
	"fmt"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func buildDevice(cfg CameraConfig) models.Device {
	metadata := map[string]any{
		"host":        cfg.Host,
		"port":        cfg.Port,
		"channel":     cfg.Channel,
		"rtsp_port":   cfg.RTSPPort,
		"rtsp_path":   cfg.RTSPPath,
		"sdk_lib_dir": cfg.SDKLibDir,
		"entry_id":    cfg.EntryID,
		"controls":    buildControlSpecs(),
	}
	return models.Device{
		ID:             cfg.DeviceID,
		PluginID:       pluginID,
		VendorDeviceID: fmt.Sprintf("%s:%d:ch%d", cfg.Host, cfg.Port, cfg.Channel),
		Kind:           models.DeviceKindCameraLike,
		Name:           cfg.Name,
		Online:         false,
		Capabilities:   []string{"stream", "ptz", "playback", "recordings"},
		Metadata:       metadata,
	}
}

func buildControlSpecs() []models.DeviceControlSpec {
	return []models.DeviceControlSpec{
		actionControl("ptz-up", "PTZ Up", "up"),
		actionControl("ptz-down", "PTZ Down", "down"),
		actionControl("ptz-left", "PTZ Left", "left"),
		actionControl("ptz-right", "PTZ Right", "right"),
		actionControl("ptz-zoom-in", "Zoom In", "zoom_in"),
		actionControl("ptz-zoom-out", "Zoom Out", "zoom_out"),
	}
}

func actionControl(id, label, direction string) models.DeviceControlSpec {
	return models.DeviceControlSpec{
		ID:    id,
		Kind:  models.DeviceControlKindAction,
		Label: label,
		Command: &models.DeviceControlCommand{
			Action: "ptz_move",
			Params: map[string]any{"direction": direction},
		},
	}
}

func buildState(cfg CameraConfig, status cameraStatus, lastError string) models.DeviceStateSnapshot {
	state := map[string]any{
		"connected":   status.Connected,
		"host":        cfg.Host,
		"port":        cfg.Port,
		"channel":     cfg.Channel,
		"rtsp_url":    status.RTSPURL,
		"sdk_lib_dir": cfg.SDKLibDir,
	}
	if status.Playback != nil {
		state["playback"] = cloneMap(status.Playback)
	} else {
		state["playback"] = map[string]any{}
	}
	if lastError != "" {
		state["last_error"] = lastError
	}
	return models.DeviceStateSnapshot{
		DeviceID: cfg.DeviceID,
		PluginID: pluginID,
		TS:       time.Now().UTC(),
		State:    state,
	}
}
