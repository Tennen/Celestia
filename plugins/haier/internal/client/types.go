package client

import (
	"fmt"
	"strings"
	"time"
)

// AccountConfig holds the credentials and preferences for one Haier account.
// Shared types that are exclusively used by the client live here; plugin-level
// runtime types (accountRuntime, applianceRuntime, Config) remain in internal/app.
type AccountConfig struct {
	Name         string `json:"name,omitempty"`
	Email        string `json:"email"`
	Password     string `json:"password,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	MobileID     string `json:"mobile_id,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
}

func (a AccountConfig) NormalizedName() string {
	if name := strings.TrimSpace(a.Name); name != "" {
		return name
	}
	email := strings.TrimSpace(a.Email)
	if email == "" {
		return "haier"
	}
	if idx := strings.Index(email, "@"); idx > 0 {
		return email[:idx]
	}
	return email
}

func (a AccountConfig) NormalizedTimezone() string {
	if tz := strings.TrimSpace(a.Timezone); tz != "" {
		return tz
	}
	if loc := time.Now().Location(); loc != nil {
		if name := strings.TrimSpace(loc.String()); name != "" {
			return name
		}
	}
	return "Europe/Berlin"
}

func (a AccountConfig) NormalizedMobileID() string {
	if mobileID := strings.TrimSpace(a.MobileID); mobileID != "" {
		return mobileID
	}
	name := strings.ToLower(strings.ReplaceAll(a.NormalizedName(), " ", "-"))
	name = strings.ReplaceAll(name, "@", "-")
	if name == "" {
		name = "haier"
	}
	return fmt.Sprintf("celestia-%s", name)
}

func (a AccountConfig) HasCredentials() bool {
	email := strings.TrimSpace(a.Email)
	return strings.TrimSpace(a.RefreshToken) != "" || (email != "" && strings.TrimSpace(a.Password) != "")
}
