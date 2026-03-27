package cloud

import (
	"fmt"
	"strings"
	"time"
)

const (
	accountBaseURL       = "https://account.xiaomi.com"
	legacyCallbackURL    = "https://sts.api.io.mi.com/sts"
	legacySID            = "xiaomiio"
	legacySDKVersion     = "3.8.6"
	legacyChannel        = "MI_APP_STORE"
	legacyProtocolHeader = "PROTOCAL-HTTP2"
	legacyUserAgentFmt   = "Android-7.1.1-1.0.0-ONEPLUS A3010-136-%s APP/xiaomi.smarthome APPV/62830"
)

type orderedParam struct {
	Key   string
	Value string
}

func legacyUserAgent(deviceID string) string {
	id := strings.TrimSpace(deviceID)
	if id == "" {
		id = "CELESTIA00000000"
	}
	return fmt.Sprintf(legacyUserAgentFmt, id)
}

func normalizeLocale(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "en_US"
	}
	return value
}

func normalizeTimezone(value string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	_, offset := time.Now().Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("GMT%s%02d:%02d", sign, offset/3600, (offset%3600)/60)
}
