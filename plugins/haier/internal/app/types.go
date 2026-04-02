package app

import (
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/haier/internal/client"
)

type Config struct {
	Accounts            []client.AccountConfig `json:"accounts"`
	PollIntervalSeconds int                    `json:"poll_interval_seconds"`
}

type accountRuntime struct {
	Config      client.AccountConfig
	Client      *client.UWSClient
	Appliances  map[string]*applianceRuntime
	LastSync    time.Time
	LastError   string
	LastRefresh time.Time
	WSS         *client.WssListener
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
