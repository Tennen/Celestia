package app

import (
	"context"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type AccountConfig struct {
	Name             string `json:"name,omitempty"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Region           string `json:"region"`
	Timezone         string `json:"timezone,omitempty"`
	SessionID        string `json:"session_id,omitempty"`
	SessionUserID    string `json:"session_user_id,omitempty"`
	SessionCreatedAt string `json:"session_created_at,omitempty"`
	SessionExpiresAt string `json:"session_expires_at,omitempty"`
	SessionBaseURL   string `json:"session_base_url,omitempty"`
}

type CompatConfig struct {
	PassportBaseURL string `json:"passport_base_url,omitempty"`
	ChinaBaseURL    string `json:"china_base_url,omitempty"`
	APIVersion      string `json:"api_version,omitempty"`
	ClientHeader    string `json:"client_header,omitempty"`
	UserAgent       string `json:"user_agent,omitempty"`
	Locale          string `json:"locale,omitempty"`
	AcceptLanguage  string `json:"accept_language,omitempty"`
	Platform        string `json:"platform,omitempty"`
	OSVersion       string `json:"os_version,omitempty"`
	ModelName       string `json:"model_name,omitempty"`
	PhoneBrand      string `json:"phone_brand,omitempty"`
	Source          string `json:"source,omitempty"`
	HourMode        string `json:"hour_mode,omitempty"`
}

type Config struct {
	Accounts            []AccountConfig `json:"accounts"`
	PollIntervalSeconds int             `json:"poll_interval_seconds"`
	Compat              CompatConfig    `json:"compat,omitempty"`
}

type deviceSnapshot struct {
	AccountName string
	Client      *Client
	Info        petkitDeviceInfo
	Device      models.Device
	State       models.DeviceStateSnapshot
	Detail      map[string]any
	Records     map[string]any
	LatestEvent *deviceOccurredEvent
}

type accountRuntime struct {
	cfg      AccountConfig
	client   *Client
	devices  map[string]deviceSnapshot
	lastErr  error
	lastSync time.Time
	mqtt     *mqttListener
}

type Plugin struct {
	mu        sync.RWMutex
	config    Config
	runtimes  map[string]*accountRuntime
	devices   map[string]models.Device
	states    map[string]models.DeviceStateSnapshot
	eventKeys map[string]string
	events    chan models.Event
	cancel    context.CancelFunc
	started   bool
}

type deviceOccurredEvent struct {
	Key     string
	TS      time.Time
	Payload map[string]any
}

func New() *Plugin {
	return &Plugin{
		runtimes:  map[string]*accountRuntime{},
		devices:   map[string]models.Device{},
		states:    map[string]models.DeviceStateSnapshot{},
		eventKeys: map[string]string{},
		events:    make(chan models.Event, 128),
	}
}

func (p *Plugin) Manifest() models.PluginManifest {
	return models.PluginManifest{
		ID:      "petkit",
		Name:    "Petkit Plugin",
		Version: "1.0.0",
		Vendor:  "petkit",
		Capabilities: []string{
			"discover",
			"state",
			"command",
			"events",
			"cloud_login",
			"cloud_session",
			"feeder_control",
			"litter_control",
			"fountain_ble_relay",
		},
		ConfigSchema: map[string]any{
			"type": "object",
			"default": map[string]any{
				"accounts": []map[string]any{
					{
						"name":     "primary",
						"username": "<petkit-username>",
						"password": "<petkit-password>",
						"region":   "US",
						"timezone": "Asia/Shanghai",
					},
				},
				"poll_interval_seconds": 30,
				"compat": map[string]any{
					"passport_base_url": "https://passport.petkt.com/",
					"china_base_url":    "https://api.petkit.cn/6/",
					"api_version":       "12.4.9",
					"client_header":     "android(15.1;23127PN0CG)",
					"user_agent":        "okhttp/3.14.19",
					"locale":            "en-US",
					"accept_language":   "en-US;q=1, it-US;q=0.9",
					"platform":          "android",
					"os_version":        "15.1",
					"model_name":        "23127PN0CG",
					"phone_brand":       "Xiaomi",
					"source":            "app.petkit-android",
					"hour_mode":         "24",
				},
			},
			"properties": map[string]any{
				"poll_interval_seconds": map[string]any{
					"type":    "number",
					"default": 30,
				},
				"accounts": map[string]any{
					"type":        "array",
					"description": "Petkit cloud accounts with username, password, region and timezone.",
				},
				"compat": map[string]any{
					"type":        "object",
					"description": "Optional Petkit cloud compatibility overrides. Leave empty to use the built-in defaults that track the current upstream app behavior.",
				},
			},
		},
		DeviceKinds: []models.DeviceKind{
			models.DeviceKindPetFeeder,
			models.DeviceKindPetLitterBox,
			models.DeviceKindPetFountain,
		},
	}
}

func (p *Plugin) ValidateConfig(_ context.Context, cfg map[string]any) error {
	_, err := parseConfig(cfg)
	return err
}

func (p *Plugin) Setup(_ context.Context, cfg map[string]any) error {
	config, err := parseConfig(cfg)
	if err != nil {
		return err
	}
	runtimes := make(map[string]*accountRuntime, len(config.Accounts))
	for _, account := range config.Accounts {
		runtimes[accountKey(account)] = &accountRuntime{
			cfg:     account,
			client:  NewClient(account, config.Compat),
			devices: map[string]deviceSnapshot{},
		}
	}

	p.mu.Lock()
	p.config = config
	p.runtimes = runtimes
	p.devices = map[string]models.Device{}
	p.states = map[string]models.DeviceStateSnapshot{}
	p.eventKeys = map[string]string{}
	p.mu.Unlock()
	return nil
}
