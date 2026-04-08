package app

import "errors"

// handleStreamRTSPURL returns the RTSP URL for the camera entry.
// Credentials are included so the core relay can open the RTSP connection.
// This payload is consumed server-side and never forwarded to the browser.
func (p *Plugin) handleStreamRTSPURL(runtime *entryRuntime) (map[string]any, string, error) {
	rtspURL := buildRTSPURL(runtime.Config, runtime.CloudDevice)
	if rtspURL == "" {
		return nil, "", errors.New("rtsp stream is not configured for this camera")
	}
	return map[string]any{
		"rtsp_url": rtspURL,
	}, "rtsp url ready", nil
}
