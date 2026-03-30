package app

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/chentianyu/celestia/internal/pluginutil"
)

const (
	defaultPollIntervalSeconds = 30
	minPollIntervalSeconds     = 5
	defaultBackendBaseURL      = "http://127.0.0.1:8099"
	defaultSDKPort             = 8000
	defaultChannel             = 1
	defaultRTSPPort            = 554
	defaultRTSPPath            = "/Streaming/Channels/{channel}01"
	defaultPTZSpeed            = 4
	defaultPTZStepMS           = 400
)

type Config struct {
	PollIntervalSeconds int
	Entries             []CameraConfig
}

type CameraConfig struct {
	Name              string
	EntryID           string
	DeviceID          string
	Host              string
	Port              int
	Username          string
	Password          string
	Channel           int
	RTSPPort          int
	RTSPPath          string
	PTZDefaultSpeed   int
	PTZStepMS         int
	BackendBaseURL    string
	SDKLibDirOverride string
}

func parseConfig(cfg map[string]any) (Config, error) {
	if cfg == nil {
		return Config{}, errors.New("config is required")
	}
	poll := pluginutil.Int(cfg["poll_interval_seconds"], defaultPollIntervalSeconds)
	if poll < minPollIntervalSeconds {
		poll = minPollIntervalSeconds
	}

	backendDefault := strings.TrimSpace(pluginutil.String(cfg["backend_base_url"], ""))
	if backendDefault == "" {
		backendDefault = strings.TrimSpace(os.Getenv("CELESTIA_HIKVISION_BACKEND_BASE_URL"))
	}
	if backendDefault == "" {
		backendDefault = defaultBackendBaseURL
	}

	entryMaps, err := readEntryMaps(cfg)
	if err != nil {
		return Config{}, err
	}
	if len(entryMaps) == 0 {
		return Config{}, errors.New("entries is required")
	}

	entries := make([]CameraConfig, 0, len(entryMaps))
	for idx, entryMap := range entryMaps {
		entry, err := parseEntryConfig(entryMap, idx, backendDefault)
		if err != nil {
			return Config{}, err
		}
		entries = append(entries, entry)
	}

	assignEntryIDs(entries)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].EntryID < entries[j].EntryID
	})

	return Config{PollIntervalSeconds: poll, Entries: entries}, nil
}

func readEntryMaps(cfg map[string]any) ([]map[string]any, error) {
	entries, ok := cfg["entries"]
	if ok {
		out := mapSlice(entries)
		if len(out) == 0 {
			return nil, errors.New("entries must be an array of objects")
		}
		return out, nil
	}
	if strings.TrimSpace(pluginutil.String(cfg["host"], "")) == "" {
		return nil, nil
	}
	return []map[string]any{cfg}, nil
}

func parseEntryConfig(raw map[string]any, idx int, backendDefault string) (CameraConfig, error) {
	host := strings.TrimSpace(pluginutil.String(raw["host"], ""))
	username := strings.TrimSpace(pluginutil.String(raw["username"], ""))
	password := pluginutil.String(raw["password"], "")
	if host == "" {
		return CameraConfig{}, fmt.Errorf("entries[%d].host is required", idx)
	}
	if username == "" {
		return CameraConfig{}, fmt.Errorf("entries[%d].username is required", idx)
	}
	if password == "" {
		return CameraConfig{}, fmt.Errorf("entries[%d].password is required", idx)
	}
	port := pluginutil.Int(raw["port"], defaultSDKPort)
	if port <= 0 {
		port = defaultSDKPort
	}
	channel := pluginutil.Int(raw["channel"], defaultChannel)
	if channel <= 0 {
		channel = defaultChannel
	}
	rtspPort := pluginutil.Int(raw["rtsp_port"], defaultRTSPPort)
	if rtspPort <= 0 {
		rtspPort = defaultRTSPPort
	}
	rtspPath := strings.TrimSpace(pluginutil.String(raw["rtsp_path"], defaultRTSPPath))
	if rtspPath == "" {
		rtspPath = defaultRTSPPath
	}
	ptzSpeed := pluginutil.Int(raw["ptz_default_speed"], defaultPTZSpeed)
	if ptzSpeed < 1 || ptzSpeed > 7 {
		ptzSpeed = defaultPTZSpeed
	}
	ptzStepMS := pluginutil.Int(raw["ptz_step_ms"], defaultPTZStepMS)
	if ptzStepMS < 50 {
		ptzStepMS = defaultPTZStepMS
	}

	backendBaseURL := strings.TrimSpace(pluginutil.String(raw["backend_base_url"], ""))
	if backendBaseURL == "" {
		backendBaseURL = backendDefault
	}
	if backendBaseURL == "" {
		return CameraConfig{}, fmt.Errorf("entries[%d].backend_base_url is required", idx)
	}

	name := strings.TrimSpace(pluginutil.String(raw["name"], ""))
	if name == "" {
		name = fmt.Sprintf("%s-ch%d", host, channel)
	}

	entry := CameraConfig{
		Name:              name,
		Host:              host,
		Port:              port,
		Username:          username,
		Password:          password,
		Channel:           channel,
		RTSPPort:          rtspPort,
		RTSPPath:          rtspPath,
		PTZDefaultSpeed:   ptzSpeed,
		PTZStepMS:         ptzStepMS,
		BackendBaseURL:    backendBaseURL,
		SDKLibDirOverride: strings.TrimSpace(pluginutil.String(raw["sdk_lib_dir_override"], "")),
	}
	return entry, nil
}

func assignEntryIDs(entries []CameraConfig) {
	usedEntries := map[string]int{}
	usedDevices := map[string]int{}
	for idx := range entries {
		base := sanitizeID(entries[idx].Name)
		if base == "" {
			base = fmt.Sprintf("camera-%d", idx+1)
		}
		entryID := uniqueID(base, usedEntries)
		entries[idx].EntryID = entryID
		deviceBase := fmt.Sprintf("hikvision:camera:%s", entryID)
		entries[idx].DeviceID = uniqueID(deviceBase, usedDevices)
	}
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
	output := strings.Trim(builder.String(), "-")
	return output
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
