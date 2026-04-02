package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	uwsBaseURL  = "https://uws.haier.net"
	uwsTimezone = "Asia/Shanghai"
	uwsLanguage = "zh-cn"
)

// uwsAuthState holds the in-memory session. accessToken is never persisted.
type uwsAuthState struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// UWSClient is the Haier UWS cloud API client.
type UWSClient struct {
	cfg    AccountConfig
	client *http.Client
	auth   uwsAuthState
}

func NewUWSClient(cfg AccountConfig) (*UWSClient, error) {
	return &UWSClient{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		auth: uwsAuthState{
			RefreshToken: cfg.RefreshToken,
		},
	}, nil
}

// requestJSON performs an authenticated UWS HTTP request with full signature headers.
// On HTTP 401 it refreshes the token once and retries.
func (c *UWSClient) requestJSON(ctx context.Context, method, urlPath string, body any, out any) error {
	if c.auth.AccessToken == "" {
		if err := c.Authenticate(ctx); err != nil {
			return err
		}
	}
	return c.doRequestJSON(ctx, method, urlPath, body, out, 0)
}

func (c *UWSClient) doRequestJSON(ctx context.Context, method, urlPath string, body any, out any, attempt int) error {
	var bodyBytes []byte
	var bodyStr string
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return err
		}
		bodyStr = string(bodyBytes)
	}

	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sign := Sign(urlPath, bodyStr, timestamp)
	sequenceID := uuid.NewString()

	var reader io.Reader
	if len(bodyBytes) > 0 {
		reader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, uwsBaseURL+urlPath, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("accessToken", c.auth.AccessToken)
	req.Header.Set("appId", uwsAppID)
	req.Header.Set("appKey", uwsAppKey)
	req.Header.Set("clientId", c.cfg.ClientID)
	req.Header.Set("sequenceId", sequenceID)
	req.Header.Set("sign", sign)
	req.Header.Set("timestamp", timestamp)
	req.Header.Set("timezone", uwsTimezone)
	req.Header.Set("language", uwsLanguage)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
		if err := c.refreshAccessToken(ctx); err != nil {
			return err
		}
		return c.doRequestJSON(ctx, method, urlPath, body, out, attempt+1)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("uws %s %s failed (%d): %s", method, urlPath, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("uws decode %s response: %w", urlPath, err)
	}
	return nil
}
