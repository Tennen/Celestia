package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/chentianyu/celestia/internal/pluginutil"
	lanclient "github.com/chentianyu/celestia/plugins/hikvision/internal/client"
)

const (
	defaultPollIntervalSeconds      = 30
	minPollIntervalSeconds          = 5
	defaultMaxStreamSessions        = 4
	minMaxStreamSessions            = 1
	defaultStreamIdleTimeoutSeconds = 60
	minStreamIdleTimeoutSeconds     = 10
)

func parseConfig(cfg map[string]any) (Config, error) {
	if cfg == nil {
		return Config{}, errors.New("config is required")
	}

	mode := parseRuntimeMode(cfg)
	poll := pluginutil.Int(cfg["poll_interval_seconds"], defaultPollIntervalSeconds)
	if poll < minPollIntervalSeconds {
		poll = minPollIntervalSeconds
	}

	cloudCfg, err := parseCloudConfig(cfg)
	if err != nil {
		return Config{}, err
	}

	sdkLibDefault := strings.TrimSpace(pluginutil.String(cfg["sdk_lib_dir"], ""))
	if sdkLibDefault == "" {
		sdkLibDefault = defaultSDKLibDir()
	}

	entryMaps, err := readEntryMaps(cfg)
	if err != nil {
		return Config{}, err
	}
	if len(entryMaps) == 0 {
		return Config{}, errors.New("entries is required")
	}

	entries := make([]EntryConfig, 0, len(entryMaps))
	for idx, entryMap := range entryMaps {
		entry, err := parseEntryConfig(entryMap, idx, mode, sdkLibDefault)
		if err != nil {
			return Config{}, err
		}
		entries = append(entries, entry)
	}

	assignEntryIDs(mode, entries)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].EntryID < entries[j].EntryID
	})

	return Config{
		Mode:                mode,
		PollIntervalSeconds: poll,
		Cloud:               cloudCfg,
		Entries:             entries,
	}, nil
}

func parseRuntimeMode(cfg map[string]any) RuntimeMode {
	switch strings.ToLower(strings.TrimSpace(pluginutil.String(cfg["mode"], ""))) {
	case string(RuntimeModeCloud):
		return RuntimeModeCloud
	case string(RuntimeModeLAN):
		return RuntimeModeLAN
	default:
		return RuntimeModeLAN
	}
}

func parseCloudConfig(cfg map[string]any) (CloudConfig, error) {
	raw, _ := cfg["cloud"].(map[string]any)
	cloudCfg := CloudConfig{
		Username:         pluginutil.String(raw["username"], ""),
		Password:         pluginutil.String(raw["password"], ""),
		APIURL:           pluginutil.String(raw["api_url"], ""),
		SessionID:        pluginutil.String(raw["session_id"], ""),
		RefreshSessionID: pluginutil.String(raw["refresh_session_id"], ""),
		UserName:         pluginutil.String(raw["user_name"], ""),
	}.Sanitized()

	if cloudCfg.HasCredentials() || cloudCfg.HasSession() || strings.TrimSpace(cloudCfg.APIURL) != "" {
		return cloudCfg, nil
	}
	return CloudConfig{}, nil
}

func defaultSDKLibDir() string {
	if value := strings.TrimSpace(os.Getenv("CELESTIA_HIKVISION_SDK_LIB_DIR")); value != "" {
		return value
	}
	candidates := []string{
		"/opt/celestia/sdk/lib/arm64",
		filepath.Join("plugins", "hikvision", "sdk", "lib", "arm64"),
		filepath.Join("sdk", "lib", "arm64"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return "/opt/celestia/sdk/lib/arm64"
}

func readEntryMaps(cfg map[string]any) ([]map[string]any, error) {
	if entries, ok := cfg["entries"]; ok {
		out := mapSlice(entries)
		if len(out) == 0 {
			return nil, errors.New("entries must be an array of objects")
		}
		return out, nil
	}
	if strings.TrimSpace(pluginutil.String(cfg["host"], "")) == "" &&
		strings.TrimSpace(pluginutil.String(cfg["device_serial"], "")) == "" &&
		strings.TrimSpace(pluginutil.String(cfg["rtsp_url"], "")) == "" {
		return nil, nil
	}
	return []map[string]any{cfg}, nil
}

func parseEntryConfig(raw map[string]any, idx int, mode RuntimeMode, sdkLibDefault string) (EntryConfig, error) {
	host := strings.TrimSpace(pluginutil.String(raw["host"], ""))
	rtspHost := strings.TrimSpace(pluginutil.String(raw["rtsp_host"], ""))
	username := strings.TrimSpace(pluginutil.String(raw["username"], ""))
	password := pluginutil.String(raw["password"], "")
	rtspUsername := strings.TrimSpace(pluginutil.String(raw["rtsp_username"], ""))
	rtspPassword := pluginutil.String(raw["rtsp_password"], "")
	deviceSerial := strings.TrimSpace(pluginutil.String(raw["device_serial"], ""))
	rtspURL := strings.TrimSpace(pluginutil.String(raw["rtsp_url"], ""))

	port := pluginutil.Int(raw["port"], lanclient.DefaultSDKPort)
	if port <= 0 {
		port = lanclient.DefaultSDKPort
	}
	channel := pluginutil.Int(raw["channel"], lanclient.DefaultChannel)
	if channel <= 0 {
		channel = lanclient.DefaultChannel
	}
	rtspPort := pluginutil.Int(raw["rtsp_port"], lanclient.DefaultRTSPPort)
	if rtspPort <= 0 {
		rtspPort = lanclient.DefaultRTSPPort
	}
	rtspPath := strings.TrimSpace(pluginutil.String(raw["rtsp_path"], lanclient.DefaultRTSPPath))
	if rtspPath == "" {
		rtspPath = lanclient.DefaultRTSPPath
	}

	ptzSpeed := pluginutil.Int(raw["ptz_default_speed"], lanclient.DefaultPTZSpeed)
	if ptzSpeed < 1 || ptzSpeed > 10 {
		ptzSpeed = lanclient.DefaultPTZSpeed
	}
	ptzStepMS := pluginutil.Int(raw["ptz_step_ms"], lanclient.DefaultPTZStepMS)
	if ptzStepMS < 50 {
		ptzStepMS = lanclient.DefaultPTZStepMS
	}

	sdkLibDir := strings.TrimSpace(pluginutil.String(raw["sdk_lib_dir"], ""))
	sdkLibDirOverride := strings.TrimSpace(pluginutil.String(raw["sdk_lib_dir_override"], ""))
	switch {
	case sdkLibDirOverride != "":
		sdkLibDir = sdkLibDirOverride
	case sdkLibDir == "":
		sdkLibDir = sdkLibDefault
	}

	name := strings.TrimSpace(pluginutil.String(raw["name"], ""))
	if name == "" {
		switch {
		case deviceSerial != "":
			name = deviceSerial
		case host != "":
			name = fmt.Sprintf("%s-ch%d", host, channel)
		default:
			name = fmt.Sprintf("camera-%d", idx+1)
		}
	}

	maxStreamSessions := pluginutil.Int(raw["max_stream_sessions"], defaultMaxStreamSessions)
	if maxStreamSessions == 0 {
		maxStreamSessions = defaultMaxStreamSessions
	}
	if maxStreamSessions < minMaxStreamSessions {
		maxStreamSessions = minMaxStreamSessions
	}

	streamIdleTimeout := pluginutil.Int(raw["stream_idle_timeout_seconds"], defaultStreamIdleTimeoutSeconds)
	if streamIdleTimeout == 0 {
		streamIdleTimeout = defaultStreamIdleTimeoutSeconds
	}
	if streamIdleTimeout < minStreamIdleTimeoutSeconds {
		streamIdleTimeout = minStreamIdleTimeoutSeconds
	}

	entry := EntryConfig{
		Name:                     name,
		DeviceSerial:             deviceSerial,
		Host:                     host,
		Port:                     port,
		Username:                 username,
		Password:                 password,
		Channel:                  channel,
		RTSPURL:                  rtspURL,
		RTSPHost:                 firstNonEmpty(rtspHost, host),
		RTSPPort:                 rtspPort,
		RTSPPath:                 rtspPath,
		RTSPUsername:             firstNonEmpty(rtspUsername, username),
		RTSPPassword:             firstNonEmpty(rtspPassword, password),
		PTZDefaultSpeed:          ptzSpeed,
		PTZStepMS:                ptzStepMS,
		SDKLibDir:                sdkLibDir,
		MaxStreamSessions:        maxStreamSessions,
		StreamIdleTimeoutSeconds: streamIdleTimeout,
		WebRTCNATIP:              strings.TrimSpace(pluginutil.String(raw["webrtc_nat_ip"], "")),
		WebRTCInterface:          strings.TrimSpace(pluginutil.String(raw["webrtc_interface"], "")),
	}

	switch mode {
	case RuntimeModeLAN:
		if host == "" {
			return EntryConfig{}, fmt.Errorf("entries[%d].host is required in lan mode", idx)
		}
		if username == "" {
			return EntryConfig{}, fmt.Errorf("entries[%d].username is required in lan mode", idx)
		}
		if password == "" {
			return EntryConfig{}, fmt.Errorf("entries[%d].password is required in lan mode", idx)
		}
		if strings.TrimSpace(sdkLibDir) == "" {
			return EntryConfig{}, fmt.Errorf("entries[%d].sdk_lib_dir is required in lan mode", idx)
		}
	case RuntimeModeCloud:
		if deviceSerial == "" && rtspURL == "" && host == "" && rtspHost == "" {
			return EntryConfig{}, fmt.Errorf("entries[%d] requires device_serial or RTSP settings in cloud mode", idx)
		}
	default:
		return EntryConfig{}, fmt.Errorf("unsupported hikvision mode %q", mode)
	}
	return entry, nil
}

func assignEntryIDs(mode RuntimeMode, entries []EntryConfig) {
	usedEntries := map[string]int{}
	usedDevices := map[string]int{}
	for idx := range entries {
		base := entryIdentityBase(mode, entries[idx])
		if base == "" {
			base = fmt.Sprintf("camera-%d", idx+1)
		}
		entryID := uniqueID(base, usedEntries)
		entries[idx].EntryID = entryID
		deviceBase := fmt.Sprintf("hikvision:camera:%s", entryID)
		entries[idx].DeviceID = uniqueID(deviceBase, usedDevices)
	}
}

func entryIdentityBase(mode RuntimeMode, entry EntryConfig) string {
	if mode == RuntimeModeCloud && strings.TrimSpace(entry.DeviceSerial) != "" {
		return sanitizeID(entry.DeviceSerial)
	}
	base := sanitizeID(fmt.Sprintf("%s-%d-ch%d", entry.Host, entry.Port, entry.Channel))
	if base != "" {
		return base
	}
	return sanitizeID(entry.Name)
}

func uniqueID(base string, used map[string]int) string {
	count := used[base]
	if count == 0 {
		used[base] = 1
		return base
	}
	used[base] = count + 1
	return fmt.Sprintf("%s-%d", base, count+1)
}

func sanitizeID(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	if input == "" {
		return ""
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if lastDash {
			continue
		}
		builder.WriteRune('-')
		lastDash = true
	}
	return strings.Trim(builder.String(), "-")
}

func mapSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			entry, ok := item.(map[string]any)
			if ok {
				out = append(out, entry)
			}
		}
		return out
	default:
		return nil
	}
}
