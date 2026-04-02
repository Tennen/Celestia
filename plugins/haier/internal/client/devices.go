package client

import (
	"context"
	"fmt"
	"net/http"
)

// LoadAppliances fetches the device list from the UWS platform.
// GET https://uws.haier.net/uds/v1/protected/deviceinfos
func (c *UWSClient) LoadAppliances(ctx context.Context) ([]map[string]any, error) {
	var resp struct {
		RetCode        string           `json:"retCode"`
		RetInfo        string           `json:"retInfo"`
		DeviceInfoList []map[string]any `json:"deviceInfoList"`
	}
	if err := c.requestJSON(ctx, http.MethodGet, "/uds/v1/protected/deviceinfos", nil, &resp); err != nil {
		return nil, err
	}
	if resp.RetCode != "00000" {
		return nil, fmt.Errorf("LoadAppliances failed: retCode=%s retInfo=%s", resp.RetCode, resp.RetInfo)
	}
	if resp.DeviceInfoList == nil {
		return []map[string]any{}, nil
	}
	return resp.DeviceInfoList, nil
}

// LoadDigitalModels fetches the digital model state for one or more devices.
// POST https://uws.haier.net/shadow/v1/devdigitalmodels
// Returns a map of deviceId → attribute key/value pairs.
func (c *UWSClient) LoadDigitalModels(ctx context.Context, deviceIDs []string) (map[string]map[string]string, error) {
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
		RetCode              string `json:"retCode"`
		RetInfo              string `json:"retInfo"`
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

	result := make(map[string]map[string]string, len(resp.DeviceDigitalModelList))
	for _, entry := range resp.DeviceDigitalModelList {
		if entry.DeviceID == "" {
			continue
		}
		attrs := make(map[string]string, len(entry.Attributes))
		for _, attr := range entry.Attributes {
			if attr.Name == "" {
				continue
			}
			attrs[attr.Name] = StringFromAny(attr.Value)
		}
		result[entry.DeviceID] = attrs
	}
	return result, nil
}
