package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const uwsRefreshURL = "https://zj.haier.net/api-gw/oauthserver/account/v1/refreshToken"

// Authenticate exchanges the refreshToken for an accessToken and stores it in memory.
func (c *UWSClient) Authenticate(ctx context.Context) error {
	return c.refreshAccessToken(ctx)
}

// refreshAccessToken calls the UWS refresh endpoint and updates the in-memory auth state.
// On success it also updates cfg.RefreshToken so the caller can persist the new value.
func (c *UWSClient) refreshAccessToken(ctx context.Context) error {
	body := map[string]any{
		"refreshToken": c.auth.RefreshToken,
		"clientId":     c.cfg.ClientID,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uwsRefreshURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("uws refresh token: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("uws refresh token failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("uws refresh token decode: %w", err)
	}
	if result.AccessToken == "" {
		return fmt.Errorf("uws refresh token response missing accessToken: %s", trimForError(string(raw)))
	}

	c.auth.AccessToken = result.AccessToken
	if result.RefreshToken != "" {
		c.auth.RefreshToken = result.RefreshToken
		c.cfg.RefreshToken = result.RefreshToken
	}
	c.auth.ExpiresAt = time.Now().Add(2 * time.Hour)
	return nil
}

// CurrentRefreshToken returns the current in-memory refreshToken for persistence.
func (c *UWSClient) CurrentRefreshToken() string {
	return strings.TrimSpace(c.auth.RefreshToken)
}
