package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strings"
	"time"
)

const (
	haierAuthAPI       = "https://account2.hon-smarthome.com"
	haierAPIBase       = "https://api-iot.he.services"
	haierClientID      = "3MVG9QDx8IX8nP5T2Ha8ofvlmjLZl5L_gvfbT9.HJvpHGKoAS_dcMN8LYpTSYeVFCraUnV.2Ag1Ki7m4znVO6"
	haierAppVersion    = "2.6.5"
	haierOSVersion     = 999
	haierOS            = "android"
	haierDeviceModel   = "pyhOn"
	haierUserAgent     = "Chrome/999.999.999.999"
	haierAPIKey        = "GRCqFhC6Gk@ikWXm1RmnSmX1cm,MxY-configuration"
	haierScope         = "api openid refresh_token web"
	haierTimeWindowSec = 8 * 60 * 60
)

var (
	loginContextRe = regexp.MustCompile(`"fwuid":"(.*?)","loaded":(\{.*?\})`)
	urlRefRe       = regexp.MustCompile(`(?:url|href)\s*=\s*'(.+?)'`)
	tokenRe        = regexp.MustCompile(`(access_token|refresh_token|id_token)=(.*?)&`)
)

type haierAuthState struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	CognitoToken string
	ExpiresAt    time.Time
}

// HaierClient is the Haier hOn cloud API client.
type HaierClient struct {
	cfg    AccountConfig
	client *http.Client
	auth   haierAuthState
}

type haierDeviceInfo struct {
	Appliance map[string]any
	Commands  map[string]any
	Model     map[string]any
}

func NewHaierClient(cfg AccountConfig) (*HaierClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &HaierClient{
		cfg: cfg,
		client: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *HaierClient) authenticated(ctx context.Context) bool {
	return c.auth.CognitoToken != "" && c.auth.IDToken != ""
}

func (c *HaierClient) CurrentRefreshToken() string {
	return strings.TrimSpace(c.cfg.RefreshToken)
}

func (c *HaierClient) requestJSON(ctx context.Context, method, target string, body any, headers map[string]string, out *map[string]any) error {
	if !c.authenticated(ctx) && !strings.Contains(target, "/auth/") {
		if err := c.Authenticate(ctx); err != nil {
			return err
		}
	}
	return c.requestJSONWithRetry(ctx, method, target, body, headers, out, 0)
}

func (c *HaierClient) requestJSONWithRetry(ctx context.Context, method, target string, body any, headers map[string]string, out *map[string]any, attempt int) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", haierUserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", haierAPIKey)
	req.Header.Set("cognito-token", c.auth.CognitoToken)
	req.Header.Set("id-token", c.auth.IDToken)
	req.Header.Set("X-TimezoneId", c.cfg.NormalizedTimezone())
	req.Header.Set("X-Timezone", timezoneOffset(c.cfg.NormalizedTimezone()))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if attempt == 0 {
			if err := c.Authenticate(ctx); err != nil {
				return err
			}
			return c.requestJSONWithRetry(ctx, method, target, body, headers, out, attempt+1)
		}
		return fmt.Errorf("unauthorized from %s: %s", target, strings.TrimSpace(string(raw)))
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s failed with %d: %s", target, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode %s response: %w", target, err)
	}
	return nil
}

func (c *HaierClient) noRedirectClient() *http.Client {
	clone := *c.client
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clone
}
