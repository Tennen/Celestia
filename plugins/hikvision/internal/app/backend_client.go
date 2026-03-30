package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type backendClient struct {
	baseURL    string
	httpClient *http.Client
}

type backendError struct {
	Status  int
	Message string
}

func (e *backendError) Error() string {
	if e == nil {
		return "backend error"
	}
	if e.Status > 0 {
		return fmt.Sprintf("backend request failed (%d): %s", e.Status, e.Message)
	}
	return e.Message
}

type backendStatus struct {
	OK        bool           `json:"ok"`
	EntryID   string         `json:"entry_id"`
	Connected bool           `json:"connected"`
	Host      string         `json:"host"`
	Channel   int            `json:"channel"`
	RTSPURL   string         `json:"rtsp_url"`
	Playback  map[string]any `json:"playback"`
}

type backendRecordings struct {
	OK         bool             `json:"ok"`
	EntryID    string           `json:"entry_id"`
	Date       string           `json:"date"`
	Count      int              `json:"count"`
	Recordings []map[string]any `json:"recordings"`
}

func newBackendClient(baseURL string) *backendClient {
	cleaned := strings.TrimSpace(strings.TrimRight(baseURL, "/"))
	return &backendClient{
		baseURL: cleaned,
		httpClient: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (c *backendClient) Connect(ctx context.Context, cfg CameraConfig) (backendStatus, error) {
	payload := map[string]any{
		"host":              cfg.Host,
		"port":              cfg.Port,
		"username":          cfg.Username,
		"password":          cfg.Password,
		"channel":           cfg.Channel,
		"rtsp_port":         cfg.RTSPPort,
		"rtsp_path":         cfg.RTSPPath,
		"ptz_default_speed": cfg.PTZDefaultSpeed,
		"ptz_step_ms":       cfg.PTZStepMS,
	}
	if cfg.SDKLibDirOverride != "" {
		payload["lib_dir_override"] = cfg.SDKLibDirOverride
	}
	var status backendStatus
	err := c.requestJSON(ctx, http.MethodPost, "/entries/"+cfg.EntryID+"/connect", payload, &status)
	return status, err
}

func (c *backendClient) Disconnect(ctx context.Context, entryID string) error {
	return c.requestJSON(ctx, http.MethodDelete, "/entries/"+entryID, nil, nil)
}

func (c *backendClient) Status(ctx context.Context, entryID string) (backendStatus, error) {
	var status backendStatus
	err := c.requestJSON(ctx, http.MethodGet, "/entries/"+entryID+"/status", nil, &status)
	return status, err
}

func (c *backendClient) PTZMove(ctx context.Context, entryID, direction string, speed, durationMS int) error {
	payload := map[string]any{
		"direction":   direction,
		"speed":       speed,
		"duration_ms": durationMS,
	}
	return c.requestJSON(ctx, http.MethodPost, "/entries/"+entryID+"/ptz/move", payload, nil)
}

func (c *backendClient) PTZStop(ctx context.Context, entryID, direction string, speed int) error {
	payload := map[string]any{
		"direction": direction,
		"speed":     speed,
	}
	return c.requestJSON(ctx, http.MethodPost, "/entries/"+entryID+"/ptz/stop", payload, nil)
}

func (c *backendClient) PlaybackOpen(ctx context.Context, entryID, start, end string) (map[string]any, error) {
	payload := map[string]any{"start": start, "end": end}
	out := map[string]any{}
	err := c.requestJSON(ctx, http.MethodPost, "/entries/"+entryID+"/playback/session", payload, &out)
	return out, err
}

func (c *backendClient) PlaybackControl(
	ctx context.Context,
	entryID string,
	sessionID string,
	action string,
	seekPercent *float64,
) (map[string]any, error) {
	payload := map[string]any{"action": action}
	if seekPercent != nil {
		payload["seek_percent"] = *seekPercent
	}
	out := map[string]any{}
	apiPath := "/entries/" + entryID + "/playback/" + sessionID + "/control"
	err := c.requestJSON(ctx, http.MethodPost, apiPath, payload, &out)
	return out, err
}

func (c *backendClient) PlaybackClose(ctx context.Context, entryID, sessionID string) (map[string]any, error) {
	out := map[string]any{}
	apiPath := "/entries/" + entryID + "/playback/" + sessionID
	err := c.requestJSON(ctx, http.MethodDelete, apiPath, nil, &out)
	return out, err
}

func (c *backendClient) ListRecordings(
	ctx context.Context,
	entryID string,
	date string,
	slotMinutes int,
) (backendRecordings, error) {
	query := url.Values{}
	query.Set("date", date)
	query.Set("slot_minutes", strconv.Itoa(slotMinutes))
	apiPath := "/entries/" + entryID + "/recordings?" + query.Encode()
	var out backendRecordings
	err := c.requestJSON(ctx, http.MethodGet, apiPath, nil, &out)
	return out, err
}

func (c *backendClient) requestJSON(ctx context.Context, method, apiPath string, payload any, out any) error {
	if strings.TrimSpace(c.baseURL) == "" {
		return &backendError{Message: "backend_base_url is empty"}
	}
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse backend base url: %w", err)
	}
	requestPath := apiPath
	if index := strings.Index(requestPath, "?"); index >= 0 {
		endpoint.RawQuery = requestPath[index+1:]
		requestPath = requestPath[:index]
	}
	endpoint.Path = path.Join(endpoint.Path, strings.TrimPrefix(requestPath, "/"))

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &backendError{Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return readBackendError(resp)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	return nil
}

func readBackendError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = resp.Status
	}
	asJSON := map[string]any{}
	if err := json.Unmarshal(body, &asJSON); err == nil {
		if detail, ok := asJSON["detail"].(string); ok && strings.TrimSpace(detail) != "" {
			message = strings.TrimSpace(detail)
		}
	}
	return &backendError{Status: resp.StatusCode, Message: message}
}
