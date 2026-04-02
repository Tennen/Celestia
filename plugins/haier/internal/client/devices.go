package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (c *HaierClient) LoadAppliances(ctx context.Context) ([]map[string]any, error) {
	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodGet, haierAPIBase+"/commands/v1/appliance", nil, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	result := []map[string]any{}
	if appliances, ok := payload["appliances"].([]any); ok {
		for _, raw := range appliances {
			if item, ok := raw.(map[string]any); ok {
				result = append(result, item)
			}
		}
		return result, nil
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		if appliances, ok := nested["appliances"].([]any); ok {
			for _, raw := range appliances {
				if item, ok := raw.(map[string]any); ok {
					result = append(result, item)
				}
			}
		}
	}
	return result, nil
}

func (c *HaierClient) LoadCommands(ctx context.Context, appliance map[string]any) (map[string]any, error) {
	params := url.Values{}
	params.Set("applianceType", StringFromAny(appliance["applianceTypeName"]))
	params.Set("applianceModelId", StringFromAny(appliance["applianceModelId"]))
	params.Set("macAddress", StringFromAny(appliance["macAddress"]))
	params.Set("os", haierOS)
	params.Set("appVersion", haierAppVersion)
	params.Set("code", StringFromAny(appliance["code"]))
	if firmwareID := StringFromAny(appliance["eepromId"]); firmwareID != "" {
		params.Set("firmwareId", firmwareID)
	}
	if firmwareVersion := StringFromAny(appliance["fwVersion"]); firmwareVersion != "" {
		params.Set("fwVersion", firmwareVersion)
	}
	if series := StringFromAny(appliance["series"]); series != "" {
		params.Set("series", series)
	}

	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodGet, haierAPIBase+"/commands/v1/retrieve?"+params.Encode(), nil, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	if resultCode := StringFromAny(payload["resultCode"]); resultCode != "" && resultCode != "0" {
		return nil, fmt.Errorf("command metadata request failed: resultCode=%s", resultCode)
	}
	return payload, nil
}

func (c *HaierClient) LoadAttributes(ctx context.Context, appliance map[string]any) (map[string]any, error) {
	params := url.Values{}
	params.Set("macAddress", StringFromAny(appliance["macAddress"]))
	params.Set("applianceType", StringFromAny(appliance["applianceTypeName"]))
	params.Set("category", "CYCLE")
	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodGet, haierAPIBase+"/commands/v1/context?"+params.Encode(), nil, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	return payload, nil
}

func (c *HaierClient) LoadStatistics(ctx context.Context, appliance map[string]any) (map[string]any, error) {
	params := url.Values{}
	params.Set("macAddress", StringFromAny(appliance["macAddress"]))
	params.Set("applianceType", StringFromAny(appliance["applianceTypeName"]))
	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodGet, haierAPIBase+"/commands/v1/statistics?"+params.Encode(), nil, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	return payload, nil
}

func (c *HaierClient) LoadMaintenance(ctx context.Context, appliance map[string]any) (map[string]any, error) {
	params := url.Values{}
	params.Set("macAddress", StringFromAny(appliance["macAddress"]))
	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodGet, haierAPIBase+"/commands/v1/maintenance-cycle?"+params.Encode(), nil, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	return payload, nil
}

func (c *HaierClient) SendCommand(ctx context.Context, appliance map[string]any, command string, parameters map[string]any, ancillaryParameters map[string]any, programName string) (map[string]any, error) {
	now := time.Now().UTC()
	body := map[string]any{
		"macAddress":       StringFromAny(appliance["macAddress"]),
		"timestamp":        now.Format("2006-01-02T15:04:05.000Z"),
		"commandName":      command,
		"transactionId":    fmt.Sprintf("%s_%s", StringFromAny(appliance["macAddress"]), now.Format("2006-01-02T15:04:05.000Z")),
		"applianceOptions": applianceOptions(appliance),
		"device": map[string]any{
			"appVersion":  haierAppVersion,
			"mobileId":    c.cfg.NormalizedMobileID(),
			"mobileOs":    haierOS,
			"osVersion":   haierOSVersion,
			"deviceModel": haierDeviceModel,
		},
		"attributes": map[string]any{
			"channel":     "mobileApp",
			"origin":      "standardProgram",
			"energyLabel": "0",
		},
		"ancillaryParameters": ancillaryParameters,
		"parameters":          parameters,
		"applianceType":       StringFromAny(appliance["applianceTypeName"]),
	}
	if programName != "" && command == "startProgram" {
		body["programName"] = strings.ToUpper(programName)
	}
	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodPost, haierAPIBase+"/commands/v1/send", body, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	if resultCode := resultCodeFrom(payload); resultCode != "" && resultCode != "0" {
		return payload, fmt.Errorf("command failed: resultCode=%s", resultCode)
	}
	return payload, nil
}
