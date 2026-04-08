package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

func (s *Session) getPagelist(ctx context.Context, offset int) (map[string]any, error) {
	query := url.Values{
		"groupId": []string{"-1"},
		"limit":   []string{"30"},
		"offset":  []string{stringValue(offset)},
		"filter":  []string{defaultPagelistFilter},
	}
	resp, err := s.doJSONRequest(ctx, http.MethodGet, pagelistPath, query, nil, "", true)
	if err != nil {
		return nil, err
	}
	if !responseOK(resp) {
		return nil, responseError("ezviz pagelist request failed", resp)
	}
	payload := structToMap(resp)
	page := mapValue(payload["page"])
	if truthy(page["hasNext"]) {
		next, err := s.getPagelist(ctx, offset+30)
		if err != nil {
			return nil, err
		}
		payload = mergeMap(payload, next)
	}
	return payload, nil
}

func buildDeviceIndex(payload map[string]any) map[string]DeviceInfo {
	deviceInfos := sliceValue(payload["deviceInfos"])
	if len(deviceInfos) == 0 {
		return map[string]DeviceInfo{}
	}

	result := make(map[string]DeviceInfo, len(deviceInfos))
	cloudResources := mapValue(payload["CLOUD"])
	for _, item := range deviceInfos {
		device := mapValue(item)
		serial := stringValue(device["deviceSerial"])
		if serial == "" {
			continue
		}
		resourceID := findResourceID(cloudResources, serial)
		raw := map[string]any{
			"CLOUD":         map[string]any{resourceID: mapValue(cloudResources[resourceID])},
			"VTM":           map[string]any{resourceID: mapValue(mapValue(payload["VTM"])[resourceID])},
			"P2P":           mapValue(payload["P2P"])[serial],
			"CONNECTION":    mapValue(payload["CONNECTION"])[serial],
			"KMS":           mapValue(payload["KMS"])[serial],
			"STATUS":        mapValue(payload["STATUS"])[serial],
			"TIME_PLAN":     mapValue(payload["TIME_PLAN"])[serial],
			"CHANNEL":       map[string]any{resourceID: mapValue(mapValue(payload["CHANNEL"])[resourceID])},
			"QOS":           mapValue(payload["QOS"])[serial],
			"NODISTURB":     mapValue(payload["NODISTURB"])[serial],
			"FEATURE":       mapValue(payload["FEATURE"])[serial],
			"UPGRADE":       mapValue(payload["UPGRADE"])[serial],
			"FEATURE_INFO":  mapValue(payload["FEATURE_INFO"])[serial],
			"SWITCH":        mapValue(payload["SWITCH"])[serial],
			"CUSTOM_TAG":    mapValue(payload["CUSTOM_TAG"])[serial],
			"VIDEO_QUALITY": map[string]any{resourceID: mapValue(mapValue(payload["VIDEO_QUALITY"])[resourceID])},
			"resourceInfos": filterResourceInfos(payload["resourceInfos"], serial),
			"WIFI":          mapValue(payload["WIFI"])[serial],
			"deviceInfos":   device,
		}

		supportExt := mapValue(device["supportExt"])
		if len(supportExt) == 0 {
			if rawSupportExt := stringValue(device["supportExt"]); rawSupportExt != "" {
				var decoded map[string]any
				if err := json.Unmarshal([]byte(rawSupportExt), &decoded); err == nil {
					supportExt = decoded
					device["supportExt"] = decoded
				}
			}
		}
		optionals := mapValue(mapValue(raw["STATUS"])["optionals"])

		info := DeviceInfo{
			Serial:            serial,
			Name:              firstNonEmpty(stringValue(device["name"]), serial),
			Version:           stringValue(device["version"]),
			DeviceCategory:    stringValue(device["deviceCategory"]),
			DeviceSubCategory: stringValue(device["deviceSubCategory"]),
			Online:            deviceOnline(device, mapValue(raw["STATUS"])),
			LocalIP:           localIP(raw),
			LocalRTSPPort:     localRTSPPort(raw),
			SupportExt:        supportExt,
			Raw:               raw,
		}
		if len(supportExt) == 0 && len(optionals) > 0 {
			info.SupportExt = optionals
		}
		result[serial] = info
	}
	return result
}

func (s *Session) doJSON(req *http.Request) (apiResponse, error) {
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return apiResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiResponse{}, err
	}
	var decoded apiResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return apiResponse{}, errors.New("ezviz response was not valid JSON")
	}
	if decoded.Meta == nil {
		decoded.Meta = map[string]any{}
	}
	if decoded.Meta["status"] == nil {
		decoded.Meta["status"] = resp.StatusCode
	}
	return decoded, nil
}

func structToMap(value any) map[string]any {
	raw, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func mergeMap(base, next map[string]any) map[string]any {
	if len(base) == 0 {
		return next
	}
	if len(next) == 0 {
		return base
	}
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	for key, value := range next {
		current, ok := out[key]
		if !ok {
			out[key] = value
			continue
		}
		switch currentTyped := current.(type) {
		case map[string]any:
			out[key] = mergeMap(currentTyped, mapValue(value))
		case []any:
			out[key] = append(currentTyped, sliceValue(value)...)
		default:
			out[key] = value
		}
	}
	return out
}

func findResourceID(values map[string]any, serial string) string {
	for key, raw := range values {
		if stringValue(mapValue(raw)["deviceSerial"]) == serial {
			return key
		}
	}
	return "NONE"
}

func filterResourceInfos(value any, serial string) []any {
	items := sliceValue(value)
	out := make([]any, 0, len(items))
	for _, item := range items {
		typed := mapValue(item)
		if stringValue(typed["deviceSerial"]) == serial {
			out = append(out, typed)
		}
	}
	return out
}

func localIP(raw map[string]any) string {
	wifi := mapValue(raw["WIFI"])
	if ip := stringValue(wifi["address"]); ip != "" && ip != "0.0.0.0" {
		return ip
	}
	connection := mapValue(raw["CONNECTION"])
	if ip := stringValue(connection["localIp"]); ip != "" && ip != "0.0.0.0" {
		return ip
	}
	return ""
}

func localRTSPPort(raw map[string]any) int {
	connection := mapValue(raw["CONNECTION"])
	if port := intValue(connection["localRtspPort"]); port > 0 {
		return port
	}
	return 554
}

func deviceOnline(deviceInfo, status map[string]any) bool {
	if truthy(deviceInfo["status"]) {
		return true
	}
	return truthy(status["globalStatus"])
}

func mapValue(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	default:
		return map[string]any{}
	}
}

func sliceValue(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	default:
		return nil
	}
}
