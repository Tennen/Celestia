package cloud

import (
	"encoding/json"
	"strconv"
	"strings"
)

const DefaultAPIURL = "apiieu.ezvizlife.com"

type Config struct {
	Username         string `json:"username,omitempty"`
	Password         string `json:"password,omitempty"`
	APIURL           string `json:"api_url,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	RefreshSessionID string `json:"refresh_session_id,omitempty"`
	UserName         string `json:"user_name,omitempty"`
}

func (c Config) Sanitized() Config {
	c.Username = strings.TrimSpace(c.Username)
	c.Password = strings.TrimSpace(c.Password)
	c.APIURL = strings.TrimSpace(c.APIURL)
	c.SessionID = strings.TrimSpace(c.SessionID)
	c.RefreshSessionID = strings.TrimSpace(c.RefreshSessionID)
	c.UserName = strings.TrimSpace(c.UserName)
	if c.APIURL == "" {
		c.APIURL = DefaultAPIURL
	}
	return c
}

func (c Config) HasCredentials() bool {
	return strings.TrimSpace(c.Username) != "" && strings.TrimSpace(c.Password) != ""
}

func (c Config) HasSession() bool {
	return strings.TrimSpace(c.APIURL) != "" &&
		strings.TrimSpace(c.SessionID) != "" &&
		strings.TrimSpace(c.RefreshSessionID) != ""
}

func (c Config) HasAuth() bool {
	return c.HasCredentials() || c.HasSession()
}

type DeviceInfo struct {
	Serial            string
	Name              string
	Version           string
	DeviceCategory    string
	DeviceSubCategory string
	Online            bool
	LocalIP           string
	LocalRTSPPort     int
	SupportExt        map[string]any
	Raw               map[string]any
}

func (d DeviceInfo) PTZSupported() bool {
	return supportExtEnabled(d.SupportExt,
		"SupportPtz",
		"SupportPtzNew",
		"SupportPtzcmdViaP2pv3",
		"SupportPtzLeftRight",
		"SupportPtzTopBottom",
		"154",
		"605",
		"169",
		"31",
		"30",
	)
}

func supportExtEnabled(values map[string]any, keys ...string) bool {
	if len(values) == 0 {
		return false
	}
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			if typed {
				return true
			}
		case float64:
			if typed != 0 {
				return true
			}
		case int:
			if typed != 0 {
				return true
			}
		case string:
			trimmed := strings.TrimSpace(strings.ToLower(typed))
			if trimmed != "" && trimmed != "0" && trimmed != "false" && trimmed != "off" {
				return true
			}
		case map[string]any:
			return true
		}
	}
	return false
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return 0
}

func truthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case string:
		trimmed := strings.TrimSpace(strings.ToLower(typed))
		return trimmed != "" && trimmed != "0" && trimmed != "false" && trimmed != "off"
	default:
		return false
	}
}
