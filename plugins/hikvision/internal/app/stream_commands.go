package app

import (
	"fmt"
	"strings"
)

// handleStreamRTSPURL returns the RTSP URL for the camera entry.
// Credentials are included so the core relay can open the RTSP connection.
// This payload is consumed server-side and never forwarded to the browser.
func (p *Plugin) handleStreamRTSPURL(runtime *entryRuntime) (map[string]any, string, error) {
	cfg := runtime.Config
	rtspPath := strings.ReplaceAll(cfg.RTSPPath, "{channel}", fmt.Sprintf("%d", cfg.Channel))
	rtspURL := fmt.Sprintf("rtsp://%s:%s@%s:%d%s",
		cfg.Username, cfg.Password, cfg.Host, cfg.RTSPPort, rtspPath)
	return map[string]any{
		"rtsp_url": rtspURL,
	}, "rtsp url ready", nil
}
