package app

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (c *Client) snapshotTransport() (string, *sessionInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.baseURL == "" {
		c.baseURL = c.defaultBaseURLForRegion()
	}
	return c.baseURL, c.session, nil
}

func (c *Client) CurrentSession() (string, sessionInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil || c.session.ID == "" || strings.TrimSpace(c.baseURL) == "" {
		return "", sessionInfo{}, false
	}
	return c.baseURL, *c.session, true
}

func (c *Client) ensureSession(ctx context.Context) error {
	c.mu.Lock()
	session := c.session
	baseURL := c.baseURL
	c.mu.Unlock()
	if session != nil && time.Until(session.ExpiresAt) > time.Minute && baseURL != "" {
		return nil
	}
	return c.login(ctx)
}

func (c *Client) login(ctx context.Context) error {
	baseURL, err := c.resolveBaseURL(ctx)
	if err != nil {
		return err
	}
	form := url.Values{}
	form.Set("oldVersion", c.compat.APIVersion)
	form.Set("client", c.compatClientPayload(c.cfg.Timezone))
	form.Set("encrypt", "1")
	form.Set("region", c.cfg.Region)
	form.Set("username", c.cfg.Username)
	form.Set("password", md5Hex(c.cfg.Password))
	result, err := c.doRequest(ctx, http.MethodPost, baseURL+"user/login", nil, form, requestAuthPublic)
	if err != nil {
		return err
	}
	sessionMap, ok := result.(map[string]any)
	if !ok {
		return errors.New("unexpected Petkit login response")
	}
	sessionVal, ok := sessionMap["session"].(map[string]any)
	if !ok {
		return errors.New("missing Petkit session in login response")
	}
	session, err := parseSession(sessionVal, c.cfg.Region)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.baseURL = baseURL
	c.session = session
	c.mu.Unlock()
	return nil
}

func (c *Client) resolveBaseURL(ctx context.Context) (string, error) {
	if strings.EqualFold(c.cfg.Region, "cn") || strings.EqualFold(c.cfg.Region, "china") {
		return c.compat.ChinaBaseURL, nil
	}
	resp, err := c.getPublicJSON(ctx, strings.TrimRight(c.compat.PassportBaseURL, "/")+"/v1/regionservers", nil)
	if err != nil {
		return "", err
	}
	list, ok := parseRegionServerList(resp)
	if !ok {
		return "", errors.New("unexpected Petkit region server response")
	}
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := strings.ToLower(stringFromAny(entry["id"], ""))
		name := strings.ToLower(stringFromAny(entry["name"], ""))
		if id == c.cfg.Region || name == c.cfg.Region {
			gateway := stringFromAny(entry["gateway"], "")
			if gateway == "" {
				break
			}
			return strings.TrimRight(gateway, "/") + "/", nil
		}
	}
	return "", fmt.Errorf("Petkit region %q not found", c.cfg.Region)
}

func parseRegionServerList(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case map[string]any:
		list, ok := typed["list"].([]any)
		if ok {
			return list, true
		}
	}
	return nil, false
}

func (c *Client) compatClientHeader() string {
	if value := strings.TrimSpace(c.compat.ClientHeader); value != "" {
		return value
	}
	return fmt.Sprintf("%s(%s;%s)", c.compat.Platform, c.compat.OSVersion, c.compat.ModelName)
}

func (c *Client) compatClientPayload(timezone string) string {
	return fmt.Sprintf(
		"{'locale': '%s', 'name': '%s', 'osVersion': '%s', 'phoneBrand': '%s', 'platform': '%s', 'source': '%s', 'version': '%s', 'timezoneId': '%s'}",
		c.compat.Locale,
		c.compat.ModelName,
		c.compat.OSVersion,
		c.compat.PhoneBrand,
		c.compat.Platform,
		c.compat.Source,
		c.compat.APIVersion,
		timezone,
	)
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (c *Client) defaultBaseURLForRegion() string {
	if strings.EqualFold(c.cfg.Region, "cn") || strings.EqualFold(c.cfg.Region, "china") {
		return strings.TrimRight(c.compat.ChinaBaseURL, "/") + "/"
	}
	return strings.TrimRight(c.compat.PassportBaseURL, "/") + "/"
}

func (c *Client) clearSessionTransport() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = ""
	c.session = nil
}

func sanitizeSessionBaseURL(baseURL string, region string, compat CompatConfig) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return ""
	}
	trimmed := strings.TrimRight(baseURL, "/")
	if strings.EqualFold(trimmed, strings.TrimRight(compat.PassportBaseURL, "/")) {
		return ""
	}
	if !strings.EqualFold(region, "cn") && strings.Contains(trimmed, "passport.petkt.com/6") {
		return ""
	}
	return trimmed + "/"
}

func parseSession(session map[string]any, region string) (*sessionInfo, error) {
	sessionID := stringFromAny(session["id"], "")
	if sessionID == "" {
		return nil, errors.New("missing Petkit session id")
	}
	userID := stringFromAny(session["userId"], "")
	expiresIn := intFromAny(session["expiresIn"], 0)
	createdAt := time.Now().UTC()
	if raw := stringFromAny(session["createdAt"], ""); raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			createdAt = parsed
		}
	}
	return &sessionInfo{
		ID:        sessionID,
		UserID:    userID,
		ExpiresIn: expiresIn,
		Region:    region,
		CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

func storedSessionFromConfig(cfg AccountConfig) (*sessionInfo, bool) {
	sessionID := strings.TrimSpace(cfg.SessionID)
	if sessionID == "" {
		return nil, false
	}
	expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(cfg.SessionExpiresAt))
	if err != nil || expiresAt.IsZero() || time.Now().UTC().After(expiresAt) {
		return nil, false
	}
	createdAt := time.Now().UTC()
	if raw := strings.TrimSpace(cfg.SessionCreatedAt); raw != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, raw); parseErr == nil {
			createdAt = parsed
		}
	}
	return &sessionInfo{
		ID:        sessionID,
		UserID:    strings.TrimSpace(cfg.SessionUserID),
		Region:    cfg.Region,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
		ExpiresIn: int(expiresAt.Sub(createdAt).Seconds()),
	}, true
}
