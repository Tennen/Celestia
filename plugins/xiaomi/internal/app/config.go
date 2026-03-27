package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/coreapi"
	"github.com/chentianyu/celestia/internal/xiaomi/oauth"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/cloud"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
)

func (p *Plugin) syncAccountSessionConfig(accountName string, client *cloud.Client) error {
	serviceToken, ssecurity, userID, cuserID, hasLegacy := client.CurrentLegacySession()
	accessToken, refreshToken, expiresAt, hasOAuth := client.CurrentOAuthTokenSet()
	if !hasLegacy && !hasOAuth {
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
		if hasLegacy {
			if account.ServiceToken != serviceToken {
				account.ServiceToken = serviceToken
				changed = true
			}
			if account.SSecurity != ssecurity {
				account.SSecurity = ssecurity
				changed = true
			}
			if account.UserID != userID {
				account.UserID = userID
				changed = true
			}
			if account.CUserID != cuserID {
				account.CUserID = cuserID
				changed = true
			}
			if account.VerifyURL != "" {
				account.VerifyURL = ""
				changed = true
			}
			if account.VerifyTicket != "" {
				account.VerifyTicket = ""
				changed = true
			}
		}
		if hasOAuth {
			if account.AccessToken != accessToken {
				account.AccessToken = accessToken
				changed = true
			}
			if refreshToken != "" && account.RefreshToken != refreshToken {
				account.RefreshToken = refreshToken
				changed = true
			}
			expires := ""
			if !expiresAt.IsZero() {
				expires = expiresAt.UTC().Format(time.RFC3339)
			}
			if account.ExpiresAt != expires {
				account.ExpiresAt = expires
				changed = true
			}
			if account.AuthCode != "" {
				account.AuthCode = ""
				changed = true
			}
		}
		if changed {
			p.config.Accounts[idx] = account
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
	if err := coreapi.PersistPluginConfig(persistCtx, "xiaomi", payload); err != nil {
		return fmt.Errorf("persist xiaomi runtime config: %w", err)
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

func parseConfig(cfg map[string]any, existing map[string]*accountRuntime) (Config, map[string]*accountRuntime, error) {
	config := Config{PollIntervalSeconds: 30}
	if raw, ok := cfg["poll_interval_seconds"].(float64); ok && int(raw) > 0 {
		config.PollIntervalSeconds = int(raw)
	}
	accountsRaw, ok := cfg["accounts"].([]any)
	if !ok || len(accountsRaw) == 0 {
		return Config{}, nil, errors.New("accounts is required")
	}
	runtimes := make(map[string]*accountRuntime, len(accountsRaw))
	for idx, raw := range accountsRaw {
		entry, _ := raw.(map[string]any)
		account, cloudCfg, err := parseAccount(entry, idx)
		if err != nil {
			return Config{}, nil, err
		}
		config.Accounts = append(config.Accounts, account)
		if prev := existing[account.Name]; prev != nil && canReuseAccountRuntime(prev.cfg, cloudCfg) {
			prev.cfg = cloudCfg
			prev.client.UpdateConfig(cloudCfg)
			runtimes[account.Name] = prev
			continue
		}
		runtimes[account.Name] = &accountRuntime{
			cfg:    cloudCfg,
			client: cloud.NewClient(cloudCfg, nil),
			specs:  map[string]spec.Instance{},
		}
	}
	return config, runtimes, nil
}

func canReuseAccountRuntime(current, next cloud.AccountConfig) bool {
	return current.Name == next.Name &&
		current.Region == next.Region &&
		current.Username == next.Username &&
		current.DeviceID == next.DeviceID
}

func parseAccount(entry map[string]any, idx int) (AccountConfig, cloud.AccountConfig, error) {
	account := AccountConfig{
		Name:         stringParam(entry["name"]),
		Region:       oauth.NormalizeRegion(stringParam(entry["region"])),
		Username:     stringParam(entry["username"]),
		Password:     stringParam(entry["password"]),
		VerifyURL:    stringParam(entry["verify_url"]),
		VerifyTicket: stringParam(entry["verify_ticket"]),
		ClientID:     stringParam(entry["client_id"]),
		RedirectURL:  stringParam(entry["redirect_url"]),
		AccessToken:  stringParam(entry["access_token"]),
		RefreshToken: stringParam(entry["refresh_token"]),
		AuthCode:     stringParam(entry["auth_code"]),
		DeviceID:     stringParam(entry["device_id"]),
		ServiceToken: stringParam(entry["service_token"]),
		SSecurity:    stringParam(entry["ssecurity"]),
		UserID:       stringParam(entry["user_id"]),
		CUserID:      stringParam(entry["cuser_id"]),
		Locale:       stringParam(entry["locale"]),
		Timezone:     stringParam(entry["timezone"]),
		ExpiresAt:    stringParam(entry["expires_at"]),
	}
	if account.Name == "" {
		account.Name = fmt.Sprintf("xiaomi-%d", idx+1)
	}
	if !slices.Contains([]string{"cn", "de", "i2", "ru", "sg", "us"}, account.Region) {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("unsupported Xiaomi region %q", account.Region)
	}
	hasPasswordLogin := account.Username != "" || account.Password != ""
	if hasPasswordLogin && (account.Username == "" || account.Password == "") {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires both username and password", account.Name)
	}
	hasVerification := account.VerifyURL != "" || account.VerifyTicket != ""
	if hasVerification && (account.VerifyURL == "" || account.VerifyTicket == "") {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires verify_url and verify_ticket together", account.Name)
	}
	if hasVerification && !hasPasswordLogin {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires username/password when verify_ticket is provided", account.Name)
	}
	hasLegacySession := account.ServiceToken != "" || account.SSecurity != "" || account.UserID != ""
	if hasLegacySession && (account.ServiceToken == "" || account.SSecurity == "" || account.UserID == "") {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires service_token, ssecurity, and user_id together", account.Name)
	}
	hasOAuthSession := account.AccessToken != "" || account.RefreshToken != "" || account.AuthCode != ""
	if (account.RefreshToken != "" || account.AuthCode != "") && (account.ClientID == "" || account.RedirectURL == "") {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires client_id and redirect_url for refresh_token/auth_code flows", account.Name)
	}
	if account.AuthCode != "" && account.DeviceID == "" {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires device_id when auth_code is provided", account.Name)
	}
	if !hasPasswordLogin && !hasLegacySession && !hasOAuthSession {
		return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q requires username/password, service_token/ssecurity/user_id, or OAuth token fields", account.Name)
	}
	if account.DeviceID == "" && hasPasswordLogin {
		account.DeviceID = stableDeviceID(account.Name, account.Username, account.Region)
	}
	if rawHomeIDs, ok := entry["home_ids"].([]any); ok {
		for _, item := range rawHomeIDs {
			value := stringParam(item)
			if value != "" {
				account.HomeIDs = append(account.HomeIDs, value)
			}
		}
	}
	var expiresAt time.Time
	if strings.TrimSpace(account.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, account.ExpiresAt)
		if err != nil {
			return AccountConfig{}, cloud.AccountConfig{}, fmt.Errorf("xiaomi account %q expires_at must be RFC3339", account.Name)
		}
		expiresAt = parsed.UTC()
	}
	cloudCfg := cloud.AccountConfig{
		Name:         account.Name,
		Region:       account.Region,
		Username:     account.Username,
		Password:     account.Password,
		VerifyURL:    account.VerifyURL,
		VerifyTicket: account.VerifyTicket,
		ClientID:     account.ClientID,
		RedirectURL:  account.RedirectURL,
		AccessToken:  account.AccessToken,
		RefreshToken: account.RefreshToken,
		AuthCode:     account.AuthCode,
		DeviceID:     account.DeviceID,
		ServiceToken: account.ServiceToken,
		SSecurity:    account.SSecurity,
		UserID:       account.UserID,
		CUserID:      account.CUserID,
		HomeIDs:      account.HomeIDs,
		Locale:       account.Locale,
		Timezone:     account.Timezone,
		ExpiresAt:    expiresAt,
	}
	return account, cloudCfg, nil
}

func stableDeviceID(parts ...string) string {
	joined := strings.ToUpper(strings.Join(parts, "|"))
	if joined == "" {
		return "CELESTIA00000000"
	}
	replacer := strings.NewReplacer("|", "", "@", "", ".", "", "-", "")
	joined = replacer.Replace(joined)
	if len(joined) >= 16 {
		return joined[:16]
	}
	for len(joined) < 16 {
		joined += "0"
	}
	return joined
}
