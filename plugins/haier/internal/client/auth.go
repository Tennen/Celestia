package client

import (
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
	}
	req, err := c.newSignedRequest(ctx, http.MethodPost, uwsRefreshURL, body)
	if err != nil {
		return err
	}

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
		RetCode      string `json:"retCode"`
		RetInfo      string `json:"retInfo"`
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
		Data         struct {
			TokenInfo struct {
				AccountToken string `json:"accountToken"`
				RefreshToken string `json:"refreshToken"`
				ExpiresIn    int    `json:"expiresIn"`
			} `json:"tokenInfo"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("uws refresh token decode: %w", err)
	}
	if result.RetCode != "" && result.RetCode != "00000" {
		return fmt.Errorf("uws refresh token failed: retCode=%s retInfo=%s", result.RetCode, result.RetInfo)
	}

	accessToken := firstNonEmpty(result.AccessToken, result.Data.TokenInfo.AccountToken)
	if accessToken == "" {
		return fmt.Errorf("uws refresh token response missing accessToken: %s", trimForError(string(raw)))
	}
	refreshToken := firstNonEmpty(result.RefreshToken, result.Data.TokenInfo.RefreshToken)
	expiresIn := result.ExpiresIn
	if result.Data.TokenInfo.ExpiresIn > 0 {
		expiresIn = result.Data.TokenInfo.ExpiresIn
	}

	c.auth.AccessToken = accessToken
	if refreshToken != "" {
		c.auth.RefreshToken = refreshToken
		c.cfg.RefreshToken = refreshToken
	}
	if expiresIn > 0 {
		c.auth.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	} else {
		c.auth.ExpiresAt = time.Now().Add(2 * time.Hour)
	}
	return nil
}

// CurrentRefreshToken returns the current in-memory refreshToken for persistence.
func (c *UWSClient) CurrentRefreshToken() string {
	return strings.TrimSpace(c.auth.RefreshToken)
}
