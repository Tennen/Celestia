package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/coreapi"
	"github.com/chentianyu/celestia/plugins/petkit/internal/client"
)

func parseConfig(cfg map[string]any) (Config, error) {
	config := Config{
		PollIntervalSeconds: 30,
		Compat:              defaultCompatConfig(),
	}
	if raw, ok := cfg["poll_interval_seconds"].(float64); ok && int(raw) > 0 {
		config.PollIntervalSeconds = int(raw)
	}
	if compat, ok := cfg["compat"].(map[string]any); ok {
		config.Compat = parseCompatConfig(compat)
	}
	rawAccounts, ok := cfg["accounts"].([]any)
	if !ok || len(rawAccounts) == 0 {
		return Config{}, errors.New("accounts is required")
	}
	for i, raw := range rawAccounts {
		entry, _ := raw.(map[string]any)
		account := AccountConfig{
			Name:             stringValue(entry["name"], ""),
			Username:         strings.TrimSpace(stringValue(entry["username"], "")),
			Password:         strings.TrimSpace(stringValue(entry["password"], "")),
			Region:           strings.ToLower(strings.TrimSpace(stringValue(entry["region"], ""))),
			Timezone:         strings.TrimSpace(stringValue(entry["timezone"], "")),
			SessionID:        strings.TrimSpace(stringValue(entry["session_id"], "")),
			SessionUserID:    strings.TrimSpace(stringValue(entry["session_user_id"], "")),
			SessionCreatedAt: strings.TrimSpace(stringValue(entry["session_created_at"], "")),
			SessionExpiresAt: strings.TrimSpace(stringValue(entry["session_expires_at"], "")),
			SessionBaseURL:   strings.TrimSpace(stringValue(entry["session_base_url"], "")),
		}
		if account.Username == "" {
			return Config{}, fmt.Errorf("accounts[%d].username is required", i)
		}
		if account.Password == "" {
			return Config{}, fmt.Errorf("accounts[%d].password is required", i)
		}
		if account.Region == "" {
			return Config{}, fmt.Errorf("accounts[%d].region is required", i)
		}
		if account.Timezone == "" {
			if account.Region == "cn" {
				account.Timezone = "Asia/Shanghai"
			} else {
				account.Timezone = "UTC"
			}
		}
		if account.Name == "" {
			account.Name = account.Username
		}
		config.Accounts = append(config.Accounts, account)
	}
	return config, nil
}

func defaultCompatConfig() CompatConfig {
	return client.DefaultCompatConfig()
}

func legacyCompatConfig() CompatConfig {
	return CompatConfig{
		PassportBaseURL: "https://passport.petkt.com/",
		ChinaBaseURL:    "https://api.petkit.cn/6/",
		APIVersion:      "13.2.1",
		ClientHeader:    "android(16.1;23127PN0CG)",
		UserAgent:       "okhttp/3.14.9",
		Locale:          "en-US",
		AcceptLanguage:  "en-US;q=1, it-US;q=0.9",
		Platform:        "android",
		OSVersion:       "16.1",
		ModelName:       "23127PN0CG",
		PhoneBrand:      "Xiaomi",
		Source:          "app.petkit-android",
		HourMode:        "24",
	}
}

func parseCompatConfig(raw map[string]any) CompatConfig {
	compat := defaultCompatConfig()
	if value := strings.TrimSpace(stringValue(raw["passport_base_url"], "")); value != "" {
		compat.PassportBaseURL = value
	}
	if value := strings.TrimSpace(stringValue(raw["china_base_url"], "")); value != "" {
		compat.ChinaBaseURL = value
	}
	if value := strings.TrimSpace(stringValue(raw["api_version"], "")); value != "" {
		compat.APIVersion = value
	}
	if value := strings.TrimSpace(stringValue(raw["client_header"], "")); value != "" {
		compat.ClientHeader = value
	}
	if value := strings.TrimSpace(stringValue(raw["user_agent"], "")); value != "" {
		compat.UserAgent = value
	}
	if value := strings.TrimSpace(stringValue(raw["locale"], "")); value != "" {
		compat.Locale = value
	}
	if value := strings.TrimSpace(stringValue(raw["accept_language"], "")); value != "" {
		compat.AcceptLanguage = value
	}
	if value := strings.TrimSpace(stringValue(raw["platform"], "")); value != "" {
		compat.Platform = value
	}
	if value := strings.TrimSpace(stringValue(raw["os_version"], "")); value != "" {
		compat.OSVersion = value
	}
	if value := strings.TrimSpace(stringValue(raw["model_name"], "")); value != "" {
		compat.ModelName = value
	}
	if value := strings.TrimSpace(stringValue(raw["phone_brand"], "")); value != "" {
		compat.PhoneBrand = value
	}
	if value := strings.TrimSpace(stringValue(raw["source"], "")); value != "" {
		compat.Source = value
	}
	if value := strings.TrimSpace(stringValue(raw["hour_mode"], "")); value != "" {
		compat.HourMode = value
	}
	compat = upgradeLegacyCompatConfig(raw, compat)
	return compat
}

func upgradeLegacyCompatConfig(raw map[string]any, compat CompatConfig) CompatConfig {
	legacy := legacyCompatConfig()
	current := defaultCompatConfig()
	if strings.TrimSpace(stringValue(raw["api_version"], "")) == legacy.APIVersion {
		compat.APIVersion = current.APIVersion
	}
	if strings.TrimSpace(stringValue(raw["client_header"], "")) == legacy.ClientHeader {
		compat.ClientHeader = current.ClientHeader
	}
	if strings.TrimSpace(stringValue(raw["user_agent"], "")) == legacy.UserAgent {
		compat.UserAgent = current.UserAgent
	}
	if strings.TrimSpace(stringValue(raw["os_version"], "")) == legacy.OSVersion {
		compat.OSVersion = current.OSVersion
	}
	return compat
}

func (p *Plugin) syncAccountSession(accountName string, c *client.Client) error {
	baseURL, session, ok := c.CurrentSession()
	if !ok {
		return nil
	}

	var snapshot Config
	changed := false

	p.mu.Lock()
	for idx := range p.config.Accounts {
		account := p.config.Accounts[idx]
		if account.Name != accountName {
			continue
		}
		createdAt := ""
		if !session.CreatedAt.IsZero() {
			createdAt = session.CreatedAt.UTC().Format(time.RFC3339)
		}
		expiresAt := ""
		if !session.ExpiresAt.IsZero() {
			expiresAt = session.ExpiresAt.UTC().Format(time.RFC3339)
		}
		if account.SessionID != session.ID {
			account.SessionID = session.ID
			changed = true
		}
		if account.SessionUserID != session.UserID {
			account.SessionUserID = session.UserID
			changed = true
		}
		if account.SessionCreatedAt != createdAt {
			account.SessionCreatedAt = createdAt
			changed = true
		}
		if account.SessionExpiresAt != expiresAt {
			account.SessionExpiresAt = expiresAt
			changed = true
		}
		if account.SessionBaseURL != baseURL {
			account.SessionBaseURL = baseURL
			changed = true
		}
		if changed {
			p.config.Accounts[idx] = account
			if runtime := p.runtimes[client.AccountKey(account)]; runtime != nil {
				runtime.cfg = account
			}
			snapshot = p.config
		}
		break
	}
	p.mu.Unlock()

	if !changed {
		return nil
	}
	payload, err := configMap(snapshot)
	if err != nil {
		return err
	}
	persistCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := coreapi.PersistPluginConfig(persistCtx, "petkit", payload); err != nil {
		return fmt.Errorf("persist petkit runtime config: %w", err)
	}
	return nil
}

func configMap(cfg Config) (map[string]any, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
