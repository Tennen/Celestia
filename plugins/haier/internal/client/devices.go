package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// LoadAppliances fetches the device list from the UWS platform.
// GET https://uws.haier.net/uds/v1/protected/deviceinfos
func (c *UWSClient) LoadAppliances(ctx context.Context) ([]map[string]any, error) {
	var resp struct {
		RetCode        string           `json:"retCode"`
		RetInfo        string           `json:"retInfo"`
		DeviceInfos    []map[string]any `json:"deviceinfos"`
		DeviceInfoList []map[string]any `json:"deviceInfoList"`
	}
	if err := c.requestJSON(ctx, http.MethodGet, "/uds/v1/protected/deviceinfos", nil, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != "00000" {
		return nil, fmt.Errorf("LoadAppliances failed: retCode=%s retInfo=%s", resp.RetCode, resp.RetInfo)
	}
	devices := resp.DeviceInfos
	if devices == nil {
		devices = resp.DeviceInfoList
	}
	if devices == nil {
		return []map[string]any{}, nil
	}
	return devices, nil
}

// LoadDigitalModels fetches the digital model state for one or more devices.
// POST https://uws.haier.net/shadow/v1/devdigitalmodels
// Returns a map of deviceId → attribute key/value pairs.
func (c *UWSClient) LoadDigitalModels(ctx context.Context, deviceIDs []string) (map[string]map[string]string, error) {
	models, err := c.LoadDigitalModelDetails(ctx, deviceIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[string]map[string]string, len(models))
	for deviceID, model := range models {
		result[deviceID] = model.Values()
	}
	return result, nil
}

// LoadDigitalModelDetails fetches the full digital model for one or more devices.
// The returned structure preserves attribute descriptions and enum options from Haier cloud.
func (c *UWSClient) LoadDigitalModelDetails(ctx context.Context, deviceIDs []string) (map[string]DigitalModel, error) {
	type deviceRef struct {
		DeviceID string `json:"deviceId"`
	}
	items := make([]deviceRef, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		items = append(items, deviceRef{DeviceID: id})
	}
	body := map[string]any{
		"deviceInfoList": items,
	}

	var resp struct {
		RetCode                string            `json:"retCode"`
		RetInfo                string            `json:"retInfo"`
		DetailInfo             map[string]string `json:"detailInfo"`
		DeviceDigitalModelList []struct {
			DeviceID   string `json:"deviceId"`
			Attributes []struct {
				Name  string `json:"name"`
				Value any    `json:"value"`
			} `json:"attributes"`
		} `json:"deviceDigitalModelList"`
	}
	if err := c.requestJSON(ctx, http.MethodPost, "/shadow/v1/devdigitalmodels", body, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != "" && resp.RetCode != "00000" {
		return nil, fmt.Errorf("LoadDigitalModels failed: retCode=%s retInfo=%s", resp.RetCode, resp.RetInfo)
	}

	result := make(map[string]DigitalModel, len(deviceIDs))
	for deviceID, raw := range resp.DetailInfo {
		model, err := decodeDigitalModel(raw)
		if err != nil {
			return nil, fmt.Errorf("decode digital model for %s: %w", deviceID, err)
		}
		result[deviceID] = model
	}
	if len(result) > 0 {
		return result, nil
	}

	for _, entry := range resp.DeviceDigitalModelList {
		if entry.DeviceID == "" {
			continue
		}
		model := DigitalModel{Attributes: make([]DigitalModelAttribute, 0, len(entry.Attributes))}
		for _, attr := range entry.Attributes {
			if attr.Name == "" {
				continue
			}
			model.Attributes = append(model.Attributes, DigitalModelAttribute{
				Name:  attr.Name,
				Value: StringFromAny(attr.Value),
			})
		}
		result[entry.DeviceID] = model
	}
	return result, nil
}

func decodeDigitalModel(raw string) (DigitalModel, error) {
	if raw == "" {
		return DigitalModel{}, nil
	}
	var payload struct {
		Attributes []struct {
			Name       string `json:"name"`
			Desc       string `json:"desc"`
			Value      any    `json:"value"`
			Readable   bool   `json:"readable"`
			Writable   bool   `json:"writable"`
			ValueRange struct {
				DataList []struct {
					Data any    `json:"data"`
					Desc string `json:"desc"`
				} `json:"dataList"`
			} `json:"valueRange"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return DigitalModel{}, err
	}
	model := DigitalModel{Attributes: make([]DigitalModelAttribute, 0, len(payload.Attributes))}
	for _, attr := range payload.Attributes {
		if attr.Name == "" {
			continue
		}
		item := DigitalModelAttribute{
			Name:        attr.Name,
			Description: strings.TrimSpace(attr.Desc),
			Value:       StringFromAny(attr.Value),
			Readable:    attr.Readable,
			Writable:    attr.Writable,
			Options:     make([]DigitalModelValueOption, 0, len(attr.ValueRange.DataList)),
		}
		for _, option := range attr.ValueRange.DataList {
			value := StringFromAny(option.Data)
			label := strings.TrimSpace(option.Desc)
			if value == "" && label == "" {
				continue
			}
			item.Options = append(item.Options, DigitalModelValueOption{
				Value: value,
				Label: firstNonEmpty(label, value),
			})
		}
		model.Attributes = append(model.Attributes, item)
	}
	return model, nil
}
