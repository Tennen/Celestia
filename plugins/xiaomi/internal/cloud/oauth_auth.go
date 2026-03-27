package cloud

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/chentianyu/celestia/internal/xiaomi/oauth"
)

func (c *Client) ensureOAuthToken(ctx context.Context) error {
	c.mu.Lock()
	accessToken := c.accessToken
	refreshToken := c.refreshToken
	expiresAt := c.expiresAt
	authCode := c.cfg.AuthCode
	deviceID := c.deviceID
	region := c.cfg.Region
	clientID := c.clientID
	redirectURL := c.redirectURL
	c.mu.Unlock()

	if accessToken != "" && (expiresAt.IsZero() || time.Now().UTC().Before(expiresAt)) {
		return nil
	}

	var (
		tokenSet oauth.TokenSet
		err      error
	)
	switch {
	case refreshToken != "":
		tokenSet, err = c.authClient.RefreshToken(ctx, region, clientID, redirectURL, refreshToken)
	case authCode != "":
		tokenSet, err = c.authClient.ExchangeCode(ctx, region, clientID, redirectURL, authCode, deviceID)
	default:
		if accessToken != "" {
			return nil
		}
		return fmt.Errorf("xiaomi account %q requires access_token or refresh_token or auth_code", c.cfg.Name)
	}
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.accessToken = tokenSet.AccessToken
	if tokenSet.RefreshToken != "" {
		c.refreshToken = tokenSet.RefreshToken
	}
	c.expiresAt = tokenSet.ExpiresAt
	c.mu.Unlock()
	return nil
}

func (c *Client) UserProfile(ctx context.Context) (map[string]any, error) {
	if c.usesLegacyAuth() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.userID == "" {
			return nil, fmt.Errorf("xiaomi account %q is not authenticated", c.cfg.Name)
		}
		return map[string]any{
			"user_id":  c.userID,
			"cuser_id": c.cuserID,
			"region":   oauth.NormalizeRegion(c.cfg.Region),
			"mode":     "service_token",
		}, nil
	}
	if err := c.ensureOAuthToken(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, profileURL, nil)
	if err != nil {
		return nil, err
	}
	query := req.URL.Query()
	query.Set("clientId", c.clientID)
	query.Set("token", c.AccessToken())
	req.URL.RawQuery = query.Encode()
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var body struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := c.do(req, &body); err != nil {
		return nil, err
	}
	if body.Code != 0 || body.Data == nil {
		return nil, fmt.Errorf("xiaomi user profile request failed")
	}
	return body.Data, nil
}
