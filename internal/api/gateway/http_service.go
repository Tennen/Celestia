package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type HTTPService struct {
	baseURL string
	client  *http.Client
}

func NewHTTPService(baseURL string, timeout time.Duration) *HTTPService {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		trimmed = "http://127.0.0.1:8080"
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPService{
		baseURL: trimmed,
		client:  &http.Client{Timeout: timeout},
	}
}

func (s *HTTPService) Health(ctx context.Context) (HealthStatus, error) {
	var out HealthStatus
	if err := s.get(ctx, "/api/v1/health", nil, &out); err != nil {
		return HealthStatus{}, err
	}
	return out, nil
}

func (s *HTTPService) Dashboard(ctx context.Context) (models.DashboardSummary, error) {
	var out models.DashboardSummary
	if err := s.get(ctx, "/api/v1/dashboard", nil, &out); err != nil {
		return models.DashboardSummary{}, err
	}
	return out, nil
}

func (s *HTTPService) ListCatalogPlugins(ctx context.Context) ([]models.CatalogPlugin, error) {
	var out []models.CatalogPlugin
	if err := s.get(ctx, "/api/v1/catalog/plugins", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPService) ListPlugins(ctx context.Context) ([]models.PluginRuntimeView, error) {
	var out []models.PluginRuntimeView
	if err := s.get(ctx, "/api/v1/plugins", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPService) InstallPlugin(ctx context.Context, req InstallPluginRequest) (models.PluginInstallRecord, error) {
	var out models.PluginInstallRecord
	if err := s.request(ctx, http.MethodPost, "/api/v1/plugins", nil, req, &out, ""); err != nil {
		return models.PluginInstallRecord{}, err
	}
	return out, nil
}

func (s *HTTPService) UpdatePluginConfig(ctx context.Context, req UpdatePluginConfigRequest) (models.PluginInstallRecord, error) {
	var out models.PluginInstallRecord
	payload := map[string]any{"config": req.Config}
	path := fmt.Sprintf("/api/v1/plugins/%s/config", url.PathEscape(req.PluginID))
	if err := s.request(ctx, http.MethodPut, path, nil, payload, &out, ""); err != nil {
		return models.PluginInstallRecord{}, err
	}
	return out, nil
}

func (s *HTTPService) EnablePlugin(ctx context.Context, pluginID string) error {
	path := fmt.Sprintf("/api/v1/plugins/%s/enable", url.PathEscape(pluginID))
	return s.request(ctx, http.MethodPost, path, nil, nil, nil, "")
}

func (s *HTTPService) DisablePlugin(ctx context.Context, pluginID string) error {
	path := fmt.Sprintf("/api/v1/plugins/%s/disable", url.PathEscape(pluginID))
	return s.request(ctx, http.MethodPost, path, nil, nil, nil, "")
}

func (s *HTTPService) DiscoverPlugin(ctx context.Context, pluginID string) error {
	path := fmt.Sprintf("/api/v1/plugins/%s/discover", url.PathEscape(pluginID))
	return s.request(ctx, http.MethodPost, path, nil, nil, nil, "")
}

func (s *HTTPService) DeletePlugin(ctx context.Context, pluginID string) error {
	path := fmt.Sprintf("/api/v1/plugins/%s", url.PathEscape(pluginID))
	return s.request(ctx, http.MethodDelete, path, nil, nil, nil, "")
}

func (s *HTTPService) GetPluginLogs(ctx context.Context, pluginID string) (PluginLogsView, error) {
	var out PluginLogsView
	path := fmt.Sprintf("/api/v1/plugins/%s/logs", url.PathEscape(pluginID))
	if err := s.get(ctx, path, nil, &out); err != nil {
		return PluginLogsView{}, err
	}
	return out, nil
}

func (s *HTTPService) ListAutomations(ctx context.Context) ([]models.Automation, error) {
	var out []models.Automation
	if err := s.get(ctx, "/api/v1/automations", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPService) SaveAutomation(ctx context.Context, automation models.Automation) (models.Automation, error) {
	var out models.Automation
	path := "/api/v1/automations"
	method := http.MethodPost
	if strings.TrimSpace(automation.ID) != "" {
		method = http.MethodPut
		path = fmt.Sprintf("/api/v1/automations/%s", url.PathEscape(automation.ID))
	}
	if err := s.request(ctx, method, path, nil, automation, &out, ""); err != nil {
		return models.Automation{}, err
	}
	return out, nil
}

func (s *HTTPService) DeleteAutomation(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/automations/%s", url.PathEscape(id))
	return s.request(ctx, http.MethodDelete, path, nil, nil, nil, "")
}

func (s *HTTPService) ListDevices(ctx context.Context, filter DeviceFilter) ([]models.DeviceView, error) {
	var out []models.DeviceView
	query := url.Values{}
	if filter.PluginID != "" {
		query.Set("plugin_id", filter.PluginID)
	}
	if filter.Kind != "" {
		query.Set("kind", filter.Kind)
	}
	if filter.Query != "" {
		query.Set("q", filter.Query)
	}
	if err := s.get(ctx, "/api/v1/devices", query, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPService) GetDevice(ctx context.Context, deviceID string) (models.DeviceView, error) {
	var out models.DeviceView
	path := fmt.Sprintf("/api/v1/devices/%s", url.PathEscape(deviceID))
	if err := s.get(ctx, path, nil, &out); err != nil {
		return models.DeviceView{}, err
	}
	return out, nil
}

func (s *HTTPService) UpdateDevicePreference(ctx context.Context, req UpdateDevicePreferenceRequest) (models.DevicePreference, error) {
	var out models.DevicePreference
	path := fmt.Sprintf("/api/v1/devices/%s/preference", url.PathEscape(req.DeviceID))
	payload := map[string]any{"alias": req.Alias}
	if err := s.request(ctx, http.MethodPut, path, nil, payload, &out, ""); err != nil {
		return models.DevicePreference{}, err
	}
	return out, nil
}

func (s *HTTPService) UpdateControlPreference(ctx context.Context, req UpdateControlPreferenceRequest) (models.DeviceControlPreference, error) {
	var out models.DeviceControlPreference
	path := fmt.Sprintf("/api/v1/devices/%s/controls/%s", url.PathEscape(req.DeviceID), url.PathEscape(req.ControlID))
	payload := map[string]any{"alias": req.Alias}
	if req.Visible != nil {
		payload["visible"] = *req.Visible
	}
	if err := s.request(ctx, http.MethodPut, path, nil, payload, &out, ""); err != nil {
		return models.DeviceControlPreference{}, err
	}
	return out, nil
}

func (s *HTTPService) SendDeviceCommand(ctx context.Context, req DeviceCommandRequest) (CommandExecutionResult, error) {
	var out CommandExecutionResult
	path := fmt.Sprintf("/api/v1/devices/%s/commands", url.PathEscape(req.DeviceID))
	payload := map[string]any{"action": req.Action, "params": req.Params}
	if err := s.request(ctx, http.MethodPost, path, nil, payload, &out, req.Actor); err != nil {
		return CommandExecutionResult{}, err
	}
	return out, nil
}

func (s *HTTPService) ToggleControl(ctx context.Context, req ToggleControlRequest) (CommandExecutionResult, error) {
	var out CommandExecutionResult
	mode := "off"
	if req.On {
		mode = "on"
	}
	path := fmt.Sprintf("/api/v1/toggle/%s/%s", url.PathEscape(req.CompoundControlID), mode)
	if err := s.request(ctx, http.MethodPost, path, nil, nil, &out, req.Actor); err != nil {
		return CommandExecutionResult{}, err
	}
	return out, nil
}

func (s *HTTPService) RunActionControl(ctx context.Context, req ActionControlRequest) (CommandExecutionResult, error) {
	var out CommandExecutionResult
	path := fmt.Sprintf("/api/v1/action/%s", url.PathEscape(req.CompoundControlID))
	if err := s.request(ctx, http.MethodPost, path, nil, nil, &out, req.Actor); err != nil {
		return CommandExecutionResult{}, err
	}
	return out, nil
}

func (s *HTTPService) ListEvents(ctx context.Context, filter EventFilter) ([]models.Event, error) {
	var out []models.Event
	query := url.Values{}
	if filter.PluginID != "" {
		query.Set("plugin_id", filter.PluginID)
	}
	if filter.DeviceID != "" {
		query.Set("device_id", filter.DeviceID)
	}
	if filter.FromTS != nil {
		query.Set("from_ts", filter.FromTS.UTC().Format(time.RFC3339Nano))
	}
	if filter.ToTS != nil {
		query.Set("to_ts", filter.ToTS.UTC().Format(time.RFC3339Nano))
	}
	if filter.BeforeTS != nil {
		query.Set("before_ts", filter.BeforeTS.UTC().Format(time.RFC3339Nano))
	}
	if filter.BeforeID != "" {
		query.Set("before_id", filter.BeforeID)
	}
	if filter.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	if err := s.get(ctx, "/api/v1/events", query, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPService) ListAudits(ctx context.Context, filter AuditFilter) ([]models.AuditRecord, error) {
	var out []models.AuditRecord
	query := url.Values{}
	if filter.DeviceID != "" {
		query.Set("device_id", filter.DeviceID)
	}
	if filter.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", filter.Limit))
	}
	if err := s.get(ctx, "/api/v1/audits", query, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPService) get(ctx context.Context, path string, query url.Values, out any) error {
	return s.request(ctx, http.MethodGet, path, query, nil, out, "")
}

func (s *HTTPService) request(ctx context.Context, method, path string, query url.Values, body any, out any, actor string) error {
	fullURL := s.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return statusError(http.StatusBadRequest, err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return statusError(http.StatusInternalServerError, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		req.Header.Set("X-Actor", actor)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return statusError(http.StatusBadGateway, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return s.decodeError(resp)
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return statusError(http.StatusBadGateway, err)
	}
	return nil
}

func (s *HTTPService) decodeError(resp *http.Response) error {
	raw, _ := io.ReadAll(resp.Body)
	if len(raw) == 0 {
		return statusErrorf(resp.StatusCode, "http %d", resp.StatusCode)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err == nil {
		if resp.StatusCode == http.StatusForbidden {
			reason, _ := payload["reason"].(string)
			allowed, hasAllowed := payload["allowed"].(bool)
			if hasAllowed && !allowed {
				return &StatusError{StatusCode: resp.StatusCode, Err: &PolicyDeniedError{Decision: models.PolicyDecision{Allowed: false, Reason: reason}}}
			}
		}
		if field, ok := payload["field"].(string); ok {
			rawMatches, hasMatches := payload["matches"]
			if hasMatches {
				encodedMatches, _ := json.Marshal(rawMatches)
				var matches []AIResolveMatch
				if err := json.Unmarshal(encodedMatches, &matches); err == nil {
					value, _ := payload["value"].(string)
					return &StatusError{
						StatusCode: resp.StatusCode,
						Err: &AmbiguousReferenceError{
							Field:   field,
							Value:   value,
							Matches: matches,
						},
					}
				}
			}
		}
		if text, ok := payload["error"].(string); ok && strings.TrimSpace(text) != "" {
			return statusError(resp.StatusCode, fmt.Errorf(text))
		}
	}
	return statusError(resp.StatusCode, fmt.Errorf(strings.TrimSpace(string(raw))))
}
