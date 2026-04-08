package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	lanclient "github.com/chentianyu/celestia/plugins/hikvision/internal/client"
	ezvizcloud "github.com/chentianyu/celestia/plugins/hikvision/internal/cloud"
)

func buildDevice(mode RuntimeMode, entry EntryConfig, info *ezvizcloud.DeviceInfo, controlBlocked string) models.Device {
	rtspURL := buildRTSPURL(entry, info)
	metadata := map[string]any{
		"host":                entry.Host,
		"port":                entry.Port,
		"channel":             entry.Channel,
		"rtsp_port":           entry.RTSPPort,
		"rtsp_path":           entry.RTSPPath,
		"entry_id":            entry.EntryID,
		"mode":                mode,
		"device_serial":       entry.DeviceSerial,
		"stream_configured":   rtspURL != "",
		"control_blocked":     controlBlocked,
		"controls":            buildControlSpecs(mode, entry, info, controlBlocked),
		"max_stream_sessions": entry.MaxStreamSessions,
	}
	if mode == RuntimeModeLAN {
		metadata["sdk_lib_dir"] = entry.SDKLibDir
	}
	if info != nil {
		metadata["cloud_name"] = info.Name
		metadata["cloud_version"] = info.Version
		metadata["local_ip"] = info.LocalIP
		metadata["local_rtsp_port"] = info.LocalRTSPPort
		metadata["ptz_supported"] = info.PTZSupported()
	}

	capabilities := []string{}
	if rtspURL != "" {
		capabilities = append(capabilities, "stream")
	}
	switch mode {
	case RuntimeModeLAN:
		capabilities = append(capabilities, "ptz", "playback", "recordings")
	case RuntimeModeCloud:
		capabilities = append(capabilities, "ptz")
	}

	name := firstNonEmpty(entry.Name, entry.DeviceSerial, entry.Host, entry.EntryID)
	if info != nil {
		name = firstNonEmpty(entry.Name, info.Name, entry.DeviceSerial, entry.Host, entry.EntryID)
	}
	vendorDeviceID := firstNonEmpty(entry.DeviceSerial, fmt.Sprintf("%s:%d:ch%d", entry.Host, entry.Port, entry.Channel))
	if vendorDeviceID == "" {
		vendorDeviceID = entry.EntryID
	}
	online := false
	if info != nil {
		online = info.Online
	}
	return models.Device{
		ID:             entry.DeviceID,
		PluginID:       pluginID,
		VendorDeviceID: vendorDeviceID,
		Kind:           models.DeviceKindCameraLike,
		Name:           name,
		Online:         online,
		Capabilities:   capabilities,
		Metadata:       metadata,
	}
}

func buildControlSpecs(mode RuntimeMode, entry EntryConfig, info *ezvizcloud.DeviceInfo, controlBlocked string) []models.DeviceControlSpec {
	switch mode {
	case RuntimeModeLAN:
		return []models.DeviceControlSpec{
			actionControl("ptz-up", "PTZ Up", "up", ""),
			actionControl("ptz-down", "PTZ Down", "down", ""),
			actionControl("ptz-left", "PTZ Left", "left", ""),
			actionControl("ptz-right", "PTZ Right", "right", ""),
			actionControl("ptz-zoom-in", "Zoom In", "zoom_in", ""),
			actionControl("ptz-zoom-out", "Zoom Out", "zoom_out", ""),
		}
	case RuntimeModeCloud:
		disabledReason := controlBlocked
		if disabledReason == "" && info != nil && !info.PTZSupported() {
			disabledReason = "PTZ is not exposed by the Ezviz cloud for this camera"
		}
		return []models.DeviceControlSpec{
			actionControl("ptz-up", "PTZ Up", "up", disabledReason),
			actionControl("ptz-down", "PTZ Down", "down", disabledReason),
			actionControl("ptz-left", "PTZ Left", "left", disabledReason),
			actionControl("ptz-right", "PTZ Right", "right", disabledReason),
		}
	default:
		return nil
	}
}

func actionControl(id, label, direction, disabledReason string) models.DeviceControlSpec {
	return models.DeviceControlSpec{
		ID:             id,
		Kind:           models.DeviceControlKindAction,
		Label:          label,
		Disabled:       disabledReason != "",
		DisabledReason: disabledReason,
		Command: &models.DeviceControlCommand{
			Action: "ptz_move",
			Params: map[string]any{"direction": direction},
		},
	}
}

func buildLocalState(entry EntryConfig, status lanclient.CameraStatus, lastError string) models.DeviceStateSnapshot {
	state := map[string]any{
		"connected":   status.Connected,
		"mode":        RuntimeModeLAN,
		"host":        entry.Host,
		"port":        entry.Port,
		"channel":     entry.Channel,
		"rtsp_url":    status.RTSPURL,
		"sdk_lib_dir": entry.SDKLibDir,
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
		DeviceID: entry.DeviceID,
		PluginID: pluginID,
		TS:       time.Now().UTC(),
		State:    state,
	}
}

func buildCloudState(entry EntryConfig, info *ezvizcloud.DeviceInfo, cloudConfigured bool, controlBlocked, lastError string) models.DeviceStateSnapshot {
	rtspURL := buildRTSPURL(entry, info)
	state := map[string]any{
		"connected":           false,
		"mode":                RuntimeModeCloud,
		"device_serial":       entry.DeviceSerial,
		"cloud_authenticated": cloudConfigured,
		"ptz_enabled":         controlBlocked == "",
		"rtsp_url":            rtspURL,
	}
	if info != nil {
		state["connected"] = info.Online
		state["cloud_name"] = info.Name
		state["cloud_version"] = info.Version
		state["device_category"] = info.DeviceCategory
		state["device_sub_category"] = info.DeviceSubCategory
		state["local_ip"] = info.LocalIP
		state["local_rtsp_port"] = info.LocalRTSPPort
		state["ptz_supported"] = info.PTZSupported()
	}
	if controlBlocked != "" {
		state["ptz_disabled_reason"] = controlBlocked
	}
	if lastError != "" {
		state["last_error"] = lastError
	}
	return models.DeviceStateSnapshot{
		DeviceID: entry.DeviceID,
		PluginID: pluginID,
		TS:       time.Now().UTC(),
		State:    state,
	}
}

func buildRTSPURL(entry EntryConfig, info *ezvizcloud.DeviceInfo) string {
	if entry.RTSPURL != "" {
		return entry.RTSPURL
	}
	host := firstNonEmpty(entry.RTSPHost, entry.Host)
	if host == "" && info != nil {
		host = info.LocalIP
	}
	username := firstNonEmpty(entry.RTSPUsername, entry.Username)
	password := firstNonEmpty(entry.RTSPPassword, entry.Password)
	if host == "" || username == "" || password == "" {
		return ""
	}
	port := entry.RTSPPort
	if port <= 0 && info != nil && info.LocalRTSPPort > 0 {
		port = info.LocalRTSPPort
	}
	if port <= 0 {
		port = lanclient.DefaultRTSPPort
	}
	path := entry.RTSPPath
	if path == "" {
		path = lanclient.DefaultRTSPPath
	}
	path = strings.ReplaceAll(path, "{channel}", fmt.Sprintf("%d", entry.Channel))
	return fmt.Sprintf("rtsp://%s:%s@%s:%d%s", username, password, host, port, path)
}
