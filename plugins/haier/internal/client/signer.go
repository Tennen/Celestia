package client

import (
	"crypto/sha256"
	"fmt"
)

const (
	uwsAppID  = "MB-SHEZJAPPWXXCX-0000"
	uwsAppKey = "79ce99cc7f9804663939676031b8a427"
)

// Sign computes the UWS request signature.
// urlPath: request path without domain or query params, e.g. "/uds/v1/protected/deviceinfos"
// bodyStr: JSON-encoded request body; empty string for GET requests
// timestamp: millisecond Unix timestamp as string
// Returns a 64-character lowercase hex SHA256 digest.
func Sign(urlPath, bodyStr, timestamp string) string {
	input := urlPath + bodyStr + uwsAppID + uwsAppKey + timestamp
	sum := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", sum)
}
