//go:build linux && hikvision_sdk

package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	globalSDKEnv = &sdkEnvironment{}

	ptzDirectionToCmd = map[string]uint32{
		"up":         21,
		"down":       22,
		"left":       23,
		"right":      24,
		"up_left":    25,
		"up_right":   26,
		"down_left":  27,
		"down_right": 28,
		"zoom_in":    11,
		"zoom_out":   12,
		"focus_near": 13,
		"focus_far":  14,
	}
)

type sdkEnvironment struct {
	mu          sync.Mutex
	refCount    int
	loadedDir   string
	initialized bool
}

type playbackSession struct {
	ID         string
	Handle     int
	Start      time.Time
	End        time.Time
	Paused     bool
	LastError  string
	CreatedAt  time.Time
	LastAccess time.Time
}

type hcNetClient struct {
	mu        sync.Mutex
	cfg       CameraConfig
	connected bool
	userID    int
	channel   int
	session   *playbackSession
}

func newHCNetClient() cameraClient {
	return &hcNetClient{userID: -1}
}

func (c *hcNetClient) Connect(_ context.Context, cfg CameraConfig) (cameraStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return c.statusLocked(), nil
	}
	if err := globalSDKEnv.acquire(cfg.SDKLibDir); err != nil {
		return cameraStatus{Connected: false, Playback: map[string]any{}}, err
	}

	loginResult, err := sdkLogin(cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	if err != nil {
		globalSDKEnv.release()
		return cameraStatus{Connected: false, Playback: map[string]any{}}, err
	}

	channel := cfg.Channel
	if channel <= 0 {
		channel = loginResult.StartChannel
	}
	if channel <= 0 {
		channel = defaultChannel
	}
	cfg.Channel = channel

	c.cfg = cfg
	c.userID = loginResult.UserID
	c.channel = channel
	c.connected = true
	c.session = nil
	return c.statusLocked(), nil
}

func (c *hcNetClient) Disconnect(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.session != nil {
		sdkPlaybackStop(c.session.Handle)
		c.session = nil
	}
	if c.userID >= 0 {
		sdkLogout(c.userID)
		c.userID = -1
	}
	if c.connected {
		globalSDKEnv.release()
	}
	c.connected = false
	c.channel = 0
	return nil
}

func (c *hcNetClient) Status(_ context.Context) (cameraStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.statusLocked(), nil
}

func (c *hcNetClient) PTZMove(ctx context.Context, direction string, speed int, durationMS int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	userID, channel, err := c.requireConnectedLocked()
	if err != nil {
		return err
	}
	cmd, ok := ptzDirectionToCmd[strings.ToLower(strings.TrimSpace(direction))]
	if !ok {
		return fmt.Errorf("unsupported PTZ direction: %s", direction)
	}
	speed = clampInt(speed, 1, 7)
	if durationMS < 50 {
		durationMS = defaultPTZStepMS
	}

	if err := sdkPTZ(userID, channel, cmd, false, speed); err != nil {
		return err
	}
	timer := time.NewTimer(time.Duration(durationMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	if err := sdkPTZ(userID, channel, cmd, true, speed); err != nil {
		return err
	}
	return ctx.Err()
}

func (c *hcNetClient) PTZStop(_ context.Context, direction string, speed int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	userID, channel, err := c.requireConnectedLocked()
	if err != nil {
		return err
	}
	cmd, ok := ptzDirectionToCmd[strings.ToLower(strings.TrimSpace(direction))]
	if !ok {
		return fmt.Errorf("unsupported PTZ direction: %s", direction)
	}
	speed = clampInt(speed, 1, 7)
	return sdkPTZ(userID, channel, cmd, true, speed)
}

func (c *hcNetClient) PlaybackOpen(_ context.Context, start, end time.Time) (map[string]any, error) {
	if !end.After(start) {
		return nil, errors.New("end must be later than start")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	userID, channel, err := c.requireConnectedLocked()
	if err != nil {
		return nil, err
	}
	if c.session != nil {
		sdkPlaybackStop(c.session.Handle)
		c.session = nil
	}

	handle, err := sdkPlaybackOpen(userID, channel, start, end)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	c.session = &playbackSession{
		ID:         uuid.NewString(),
		Handle:     handle,
		Start:      start,
		End:        end,
		Paused:     false,
		CreatedAt:  now,
		LastAccess: now,
	}
	return c.playbackPayloadLocked(c.session), nil
}

func (c *hcNetClient) PlaybackControl(_ context.Context, sessionID string, action string, seekPercent *float64) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, _, err := c.requireConnectedLocked()
	if err != nil {
		return nil, err
	}
	session, err := c.requireSessionLocked(sessionID)
	if err != nil {
		return nil, err
	}

	command := sdkPlayStart
	value := uint32(0)
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "play":
		command = sdkPlayStart
		session.Paused = false
	case "pause":
		command = sdkPlayPause
		session.Paused = true
	case "seek":
		command = sdkPlaySetPos
		if seekPercent == nil {
			return nil, errors.New("seek_percent is required for seek")
		}
		value = uint32(clampInt(int(*seekPercent+0.5), 0, 100))
	default:
		return nil, fmt.Errorf("unsupported playback action %q", action)
	}

	if _, err := sdkPlaybackControl(session.Handle, command, value); err != nil {
		session.LastError = err.Error()
		return nil, err
	}
	session.LastError = ""
	session.LastAccess = time.Now().UTC()
	return c.playbackPayloadLocked(session), nil
}

func (c *hcNetClient) PlaybackClose(_ context.Context, sessionID string) (map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return map[string]any{
			"entry_id":   c.cfg.EntryID,
			"session_id": sessionID,
			"status":     "closed",
		}, nil
	}
	session, err := c.requireSessionLocked(sessionID)
	if err != nil {
		return nil, err
	}
	sdkPlaybackStop(session.Handle)
	c.session = nil
	return map[string]any{
		"entry_id":   c.cfg.EntryID,
		"session_id": sessionID,
		"status":     "closed",
	}, nil
}

func (c *hcNetClient) ListRecordings(ctx context.Context, day time.Time, _ int) ([]map[string]any, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	userID, channel, err := c.requireConnectedLocked()
	if err != nil {
		return nil, err
	}

	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local)
	end := start.Add(24*time.Hour - time.Second)
	findHandle, err := sdkFindOpen(userID, channel, start, end)
	if err != nil {
		return nil, err
	}
	defer sdkFindClose(findHandle)

	records := make([]map[string]any, 0, 32)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		ret, data, err := sdkFindNext(findHandle)
		if err != nil {
			return nil, err
		}
		switch ret {
		case sdkFindSuccess:
			if !data.End.After(data.Start) {
				continue
			}
			records = append(records, map[string]any{
				"id":               fmt.Sprintf("%s_%s_%d", data.Start.Format(time.RFC3339), data.End.Format(time.RFC3339), len(records)),
				"start":            data.Start.Format(time.RFC3339),
				"end":              data.End.Format(time.RFC3339),
				"duration_seconds": int(data.End.Sub(data.Start).Seconds()),
				"file_name":        data.FileName,
				"file_size":        data.FileSize,
				"file_type":        data.FileType,
				"locked":           data.Locked,
				"file_index":       data.FileIndex,
				"stream_type":      data.StreamType,
			})
		case sdkFindNoMore, sdkFindNoFile:
			sort.SliceStable(records, func(i, j int) bool {
				a := stringParam(records[i], "start")
				b := stringParam(records[j], "start")
				return a < b
			})
			return records, nil
		case sdkFindFinding:
			time.Sleep(50 * time.Millisecond)
		default:
			return nil, fmt.Errorf("unexpected NET_DVR_FindNextFile_V40 result: %d", ret)
		}
	}
}

func (c *hcNetClient) requireConnectedLocked() (int, int, error) {
	if !c.connected || c.userID < 0 {
		return 0, 0, errors.New("camera is not connected")
	}
	return c.userID, c.channel, nil
}

func (c *hcNetClient) requireSessionLocked(sessionID string) (*playbackSession, error) {
	if c.session == nil {
		return nil, errors.New("no active playback session")
	}
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("session_id is required")
	}
	if c.session.ID != strings.TrimSpace(sessionID) {
		return nil, fmt.Errorf("playback session %q not found", sessionID)
	}
	return c.session, nil
}

func (c *hcNetClient) statusLocked() cameraStatus {
	status := cameraStatus{
		Connected: c.connected && c.userID >= 0,
		RTSPURL:   "",
		Playback:  map[string]any{},
	}
	if !status.Connected {
		return status
	}
	status.RTSPURL = buildRTSPURL(c.cfg)
	if c.session != nil {
		status.Playback = c.playbackPayloadLocked(c.session)
	}
	return status
}

func (c *hcNetClient) playbackPayloadLocked(session *playbackSession) map[string]any {
	progress := 0
	if pos, err := sdkPlaybackControl(session.Handle, sdkPlayGetPos, 0); err == nil {
		progress = pos
	} else if strings.TrimSpace(session.LastError) == "" {
		session.LastError = err.Error()
	}
	return map[string]any{
		"session_id":  session.ID,
		"entry_id":    c.cfg.EntryID,
		"start":       session.Start.Format(time.RFC3339),
		"end":         session.End.Format(time.RFC3339),
		"status":      "running",
		"paused":      session.Paused,
		"last_error":  strings.TrimSpace(session.LastError),
		"created_at":  session.CreatedAt.Format(time.RFC3339),
		"last_access": session.LastAccess.Format(time.RFC3339),
		"progress":    progress,
	}
}

func buildRTSPURL(cfg CameraConfig) string {
	path := strings.ReplaceAll(cfg.RTSPPath, "{channel}", strconv.Itoa(cfg.Channel))
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	userInfo := url.UserPassword(cfg.Username, cfg.Password).String()
	return fmt.Sprintf("rtsp://%s@%s:%d%s", userInfo, cfg.Host, cfg.RTSPPort, path)
}

func (e *sdkEnvironment) acquire(rawSDKDir string) error {
	if runtime.GOOS != "linux" || runtime.GOARCH != "arm64" {
		return fmt.Errorf("hikvision sdk requires linux/arm64, current platform is %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	sdkDir := strings.TrimSpace(rawSDKDir)
	if sdkDir == "" {
		return errors.New("sdk_lib_dir is required")
	}
	absDir, err := filepath.Abs(sdkDir)
	if err != nil {
		return fmt.Errorf("resolve sdk_lib_dir: %w", err)
	}
	libPath := filepath.Join(absDir, "libhcnetsdk.so")
	cryptoPath := filepath.Join(absDir, "libcrypto.so.1.1")
	sslPath := filepath.Join(absDir, "libssl.so.1.1")

	required := []string{libPath, cryptoPath, sslPath}
	for _, path := range required {
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return fmt.Errorf("required sdk file missing: %s", path)
		}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.loadedDir != "" && e.loadedDir != absDir {
		return fmt.Errorf("sdk already loaded from %s; mixed sdk_lib_dir values are not supported", e.loadedDir)
	}
	if e.loadedDir == "" {
		if err := sdkLoad(libPath); err != nil {
			return err
		}
		e.loadedDir = absDir
	}
	if !e.initialized {
		appendLDLibraryPath(absDir)
		logDir := filepath.Join("/tmp", "celestia-hikvision-sdklog")
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return fmt.Errorf("prepare sdk log dir: %w", err)
		}
		if err := sdkInit(absDir, cryptoPath, sslPath, logDir); err != nil {
			return err
		}
		e.initialized = true
	}
	e.refCount++
	return nil
}

func (e *sdkEnvironment) release() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.refCount > 0 {
		e.refCount--
	}
	if e.refCount == 0 && e.initialized {
		sdkCleanup()
		e.initialized = false
	}
}

func appendLDLibraryPath(sdkDir string) {
	parts := []string{}
	current := strings.TrimSpace(os.Getenv("LD_LIBRARY_PATH"))
	if current != "" {
		parts = append(parts, strings.Split(current, ":")...)
	}
	additions := []string{sdkDir, filepath.Join(sdkDir, "HCNetSDKCom")}
	for _, value := range additions {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		found := false
		for _, item := range parts {
			if item == value {
				found = true
				break
			}
		}
		if !found {
			parts = append(parts, value)
		}
	}
	_ = os.Setenv("LD_LIBRARY_PATH", strings.Join(parts, ":"))
}
