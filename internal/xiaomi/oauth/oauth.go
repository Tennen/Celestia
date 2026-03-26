package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultOAuthHost   = "ha.api.io.mi.com"
	defaultAuthURL     = "https://account.xiaomi.com/oauth2/authorize"
	tokenLifetimeRatio = 0.7
)

type TokenSet struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type Client struct {
	httpClient *http.Client
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{httpClient: httpClient}
}

func NormalizeRegion(region string) string {
	switch strings.ToLower(strings.TrimSpace(region)) {
	case "cn", "":
		return "cn"
	case "eu", "de":
		return "de"
	case "in", "i2":
		return "i2"
	case "ru", "sg", "us":
		return strings.ToLower(strings.TrimSpace(region))
	default:
		return strings.ToLower(strings.TrimSpace(region))
	}
}

func OAuthHost(region string) string {
	region = NormalizeRegion(region)
	if region == "cn" {
		return defaultOAuthHost
	}
	return region + "." + defaultOAuthHost
}

func AuthorizeURL(clientID, redirectURL, deviceID, state string, scope []string, skipConfirm bool) (string, error) {
	if strings.TrimSpace(clientID) == "" {
		return "", fmt.Errorf("xiaomi client_id is required when generating auth_url")
	}
	if strings.TrimSpace(redirectURL) == "" {
		return "", fmt.Errorf("xiaomi redirect_url is required when generating auth_url")
	}
	if strings.TrimSpace(deviceID) == "" {
		return "", fmt.Errorf("xiaomi device_id is required when generating auth_url")
	}
	if strings.TrimSpace(state) == "" {
		return "", fmt.Errorf("xiaomi state is required when generating auth_url")
	}
	endpoint, err := url.Parse(defaultAuthURL)
	if err != nil {
		return "", err
	}
	query := endpoint.Query()
	query.Set("redirect_uri", redirectURL)
	query.Set("client_id", clientID)
	query.Set("response_type", "code")
	query.Set("device_id", deviceID)
	query.Set("state", state)
	if len(scope) > 0 {
		query.Set("scope", strings.Join(scope, " "))
	}
	query.Set("skip_confirm", fmt.Sprintf("%t", skipConfirm))
	endpoint.RawQuery = query.Encode()
	return endpoint.String(), nil
}

func (c *Client) ExchangeCode(ctx context.Context, region, clientID, redirectURL, authCode, deviceID string) (TokenSet, error) {
	if strings.TrimSpace(authCode) == "" {
		return TokenSet{}, fmt.Errorf("xiaomi auth code is required")
	}
	if strings.TrimSpace(clientID) == "" {
		return TokenSet{}, fmt.Errorf("xiaomi client_id is required when exchanging auth_code")
	}
	if strings.TrimSpace(redirectURL) == "" {
		return TokenSet{}, fmt.Errorf("xiaomi redirect_url is required when exchanging auth_code")
	}
	if strings.TrimSpace(deviceID) == "" {
		return TokenSet{}, fmt.Errorf("xiaomi device_id is required when exchanging auth_code")
	}
	return c.getToken(ctx, region, map[string]any{
		"client_id":    clientID,
		"redirect_uri": redirectURL,
		"code":         authCode,
		"device_id":    deviceID,
	})
}

func (c *Client) RefreshToken(ctx context.Context, region, clientID, redirectURL, refreshToken string) (TokenSet, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return TokenSet{}, fmt.Errorf("xiaomi refresh token is required")
	}
	if strings.TrimSpace(clientID) == "" {
		return TokenSet{}, fmt.Errorf("xiaomi client_id is required when refreshing token")
	}
	if strings.TrimSpace(redirectURL) == "" {
		return TokenSet{}, fmt.Errorf("xiaomi redirect_url is required when refreshing token")
	}
	return c.getToken(ctx, region, map[string]any{
		"client_id":     clientID,
		"redirect_uri":  redirectURL,
		"refresh_token": refreshToken,
	})
}

func (c *Client) getToken(ctx context.Context, region string, payload map[string]any) (TokenSet, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return TokenSet{}, err
	}
	endpoint := url.URL{
		Scheme: "https",
		Host:   OAuthHost(region),
		Path:   "/app/v2/ha/oauth/get_token",
	}
	query := endpoint.Query()
	query.Set("data", string(data))
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return TokenSet{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return TokenSet{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return TokenSet{}, fmt.Errorf("xiaomi oauth get_token failed: %s", resp.Status)
	}
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Result  struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			ExpiresIn    int    `json:"expires_in"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return TokenSet{}, err
	}
	if body.Code != 0 || body.Result.AccessToken == "" {
		return TokenSet{}, fmt.Errorf("xiaomi oauth get_token failed: code=%d message=%s", body.Code, body.Message)
	}
	expiresIn := time.Duration(float64(body.Result.ExpiresIn)*tokenLifetimeRatio) * time.Second
	return TokenSet{
		AccessToken:  body.Result.AccessToken,
		RefreshToken: body.Result.RefreshToken,
		ExpiresAt:    time.Now().UTC().Add(expiresIn),
	}, nil
}
