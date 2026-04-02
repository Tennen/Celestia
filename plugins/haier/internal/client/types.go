package client

import "strings"

// AccountConfig holds the credentials for one Haier UWS account.
// Only clientId and refreshToken are persisted; accessToken is kept in memory only.
type AccountConfig struct {
	Name         string `json:"name,omitempty"`
	ClientID     string `json:"clientId"`
	RefreshToken string `json:"refresh_token"`
}

func (a AccountConfig) NormalizedName() string {
	if name := strings.TrimSpace(a.Name); name != "" {
		return name
	}
	if id := strings.TrimSpace(a.ClientID); id != "" {
		if len(id) > 8 {
			return id[:8]
		}
		return id
	}
	return "haier"
}

// HasCredentials returns true when both clientId and refreshToken are non-empty.
func (a AccountConfig) HasCredentials() bool {
	return strings.TrimSpace(a.ClientID) != "" && strings.TrimSpace(a.RefreshToken) != ""
}
