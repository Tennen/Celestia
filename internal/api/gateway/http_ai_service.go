package gateway

import (
	"context"
	"net/http"
	"net/url"
)

func (s *HTTPService) ListAIDevices(ctx context.Context, filter DeviceFilter) ([]AIDevice, error) {
	var out []AIDevice
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
	if err := s.get(ctx, "/api/ai/v1/devices", query, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPService) ExecuteAICommand(ctx context.Context, req AICommandRequest) (AICommandResult, error) {
	var out AICommandResult
	payload := map[string]any{
		"target":      req.Target,
		"device_id":   req.DeviceID,
		"device_name": req.DeviceName,
		"command":     req.Command,
		"action":      req.Action,
		"params":      req.Params,
	}
	if err := s.request(ctx, http.MethodPost, "/api/ai/v1/commands", nil, payload, &out, req.Actor); err != nil {
		return AICommandResult{}, err
	}
	return out, nil
}
