package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type AccountConfig struct {
	Name         string `json:"name,omitempty"`
	Email        string `json:"email"`
	Password     string `json:"password,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	MobileID     string `json:"mobile_id,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
}

type Config struct {
	Accounts            []AccountConfig `json:"accounts"`
	PollIntervalSeconds int             `json:"poll_interval_seconds"`
}

func (a AccountConfig) normalizedName() string {
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

func (a AccountConfig) normalizedTimezone() string {
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

func (a AccountConfig) normalizedMobileID() string {
	if mobileID := strings.TrimSpace(a.MobileID); mobileID != "" {
		return mobileID
	}
	name := strings.ToLower(strings.ReplaceAll(a.normalizedName(), " ", "-"))
	name = strings.ReplaceAll(name, "@", "-")
	if name == "" {
		name = "haier"
	}
	return fmt.Sprintf("celestia-%s", name)
}

func (a AccountConfig) hasCredentials() bool {
	email := strings.TrimSpace(a.Email)
	return strings.TrimSpace(a.RefreshToken) != "" || (email != "" && strings.TrimSpace(a.Password) != "")
}

type accountRuntime struct {
	Config      AccountConfig
	Client      *haierClient
	Appliances  map[string]*applianceRuntime
	LastSync    time.Time
	LastError   string
	LoggedIn    bool
	LastRefresh time.Time
	WSS         *wssListener
}

type applianceRuntime struct {
	Device         models.Device
	ApplianceInfo  map[string]any
	CommandData    map[string]any
	CapabilitySet  map[string]bool
	CommandNames   map[string]string
	CurrentState   map[string]any
	LastSnapshotTS time.Time
}
