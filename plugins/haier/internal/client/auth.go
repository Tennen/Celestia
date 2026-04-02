package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (c *HaierClient) Authenticate(ctx context.Context) error {
	if strings.TrimSpace(c.cfg.RefreshToken) != "" {
		if err := c.refresh(ctx, c.cfg.RefreshToken); err == nil {
			return nil
		}
	}
	if strings.TrimSpace(c.cfg.Email) == "" || strings.TrimSpace(c.cfg.Password) == "" {
		return errors.New("missing email/password or refresh token")
	}
	if err := c.loginWithPassword(ctx); err != nil {
		return err
	}
	return c.apiLogin(ctx)
}

func (c *HaierClient) refresh(ctx context.Context, refreshToken string) error {
	params := url.Values{}
	params.Set("client_id", haierClientID)
	params.Set("refresh_token", refreshToken)
	params.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, haierAuthAPI+"/services/oauth2/token?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", haierUserAgent)
	resp, err := c.noRedirectClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("refresh token failed: %s", strings.TrimSpace(string(body)))
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("refresh token decode failed: %w", err)
	}
	c.auth.IDToken = StringFromAny(payload["id_token"])
	c.auth.AccessToken = StringFromAny(payload["access_token"])
	c.auth.RefreshToken = StringFromAny(payload["refresh_token"])
	c.auth.ExpiresAt = time.Now().Add(haierTimeWindowSec * time.Second)
	if c.auth.RefreshToken == "" {
		c.auth.RefreshToken = refreshToken
	}
	if c.auth.IDToken == "" || c.auth.AccessToken == "" {
		return errors.New("refresh token response missing tokens")
	}
	c.cfg.RefreshToken = c.auth.RefreshToken
	return c.apiLogin(ctx)
}

func (c *HaierClient) loginWithPassword(ctx context.Context) error {
	loginURL, parsedFromToken, err := c.introduce(ctx)
	if err != nil {
		return err
	}
	if parsedFromToken {
		return nil
	}
	loginURL, err = c.handleRedirects(ctx, loginURL)
	if err != nil {
		return err
	}
	loginURL, err = c.loadLoginPage(ctx, loginURL)
	if err != nil {
		return err
	}
	redirectURL, err := c.submitLogin(ctx, loginURL)
	if err != nil {
		return err
	}
	tokenURL, err := c.loadToken(ctx, redirectURL)
	if err != nil {
		return err
	}
	return c.extractTokens(ctx, tokenURL)
}

func (c *HaierClient) introduce(ctx context.Context) (string, bool, error) {
	nonce := generateNonce()
	params := url.Values{}
	params.Set("response_type", "token+id_token")
	params.Set("client_id", haierClientID)
	params.Set("redirect_uri", "hon://mobilesdk/detect/oauth/done")
	params.Set("display", "touch")
	params.Set("scope", haierScope)
	params.Set("nonce", nonce)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, haierAuthAPI+"/services/oauth2/authorize/expid_Login?"+params.Encode(), nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", haierUserAgent)
	resp, err := c.noRedirectClient().Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if strings.Contains(text, "oauth/done#access_token=") {
		if err := c.extractTokensFromText(text); err != nil {
			return "", false, err
		}
		return "", true, nil
	}
	match := urlRefRe.FindStringSubmatch(text)
	if len(match) < 2 {
		return "", false, fmt.Errorf("unable to locate login url in auth response: %s", trimForError(text))
	}
	loginURL := match[1]
	if strings.HasPrefix(loginURL, "/NewhOnLogin") {
		loginURL = haierAuthAPI + "/s/login" + loginURL
	}
	return loginURL, false, nil
}

func (c *HaierClient) handleRedirects(ctx context.Context, loginURL string) (string, error) {
	first, err := c.followOnce(ctx, loginURL)
	if err != nil {
		return "", err
	}
	second, err := c.followOnce(ctx, first)
	if err != nil {
		return "", err
	}
	return second + "&System=IoT_Mobile_App&RegistrationSubChannel=hOn", nil
}

func (c *HaierClient) followOnce(ctx context.Context, target string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", haierUserAgent)
	resp, err := c.noRedirectClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if loc := resp.Header.Get("Location"); loc != "" {
		return loc, nil
	}
	return target, nil
}

func (c *HaierClient) loadLoginPage(ctx context.Context, loginURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, loginURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", haierUserAgent)
	resp, err := c.noRedirectClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if match := loginContextRe.FindStringSubmatch(text); len(match) == 3 {
		return loginURL + "#fwuid=" + match[1] + "&loaded=" + url.QueryEscape(match[2]), nil
	}
	return "", fmt.Errorf("unable to parse login page context: %s", trimForError(text))
}

func (c *HaierClient) submitLogin(ctx context.Context, loginURL string) (string, error) {
	parts := strings.SplitN(loginURL, "#fwuid=", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("login context missing fwuid")
	}
	baseURL := parts[0]
	contextParts := strings.SplitN(parts[1], "&loaded=", 2)
	if len(contextParts) != 2 {
		return "", fmt.Errorf("login context missing loaded payload")
	}
	fwUID := contextParts[0]
	loadedJSON, err := url.QueryUnescape(contextParts[1])
	if err != nil {
		return "", err
	}
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	startURL := ""
	if raw := parsedBaseURL.Query().Get("startURL"); raw != "" {
		if decoded, err := url.QueryUnescape(raw); err == nil {
			startURL = strings.Split(decoded, "%3D")[0]
		} else {
			startURL = raw
		}
	}
	action := map[string]any{
		"id":                "79;a",
		"descriptor":        "apex://LightningLoginCustomController/ACTION$login",
		"callingDescriptor": "markup://c:loginForm",
		"params": map[string]any{
			"username": c.cfg.Email,
			"password": c.cfg.Password,
			"startUrl": startURL,
		},
	}
	auraContext := map[string]any{
		"mode":    "PROD",
		"fwuid":   fwUID,
		"app":     "siteforce:loginApp2",
		"loaded":  mustJSONRawMessage(loadedJSON),
		"dn":      []any{},
		"globals": map[string]any{},
		"uad":     false,
	}
	form := url.Values{}
	form.Set("message", mustJSONString(map[string]any{"actions": []any{action}}))
	form.Set("aura.context", mustJSONString(auraContext))
	form.Set("aura.pageURI", loginURL)
	form.Set("aura.token", "")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, haierAuthAPI+"/s/sfsites/aura?r=3&other.LightningLoginCustom.login=1", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", haierUserAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.noRedirectClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("login response decode failed: %w", err)
	}
	events, _ := payload["events"].([]any)
	if len(events) == 0 {
		return "", fmt.Errorf("login response missing events: %s", trimForError(string(body)))
	}
	first, _ := events[0].(map[string]any)
	attr, _ := first["attributes"].(map[string]any)
	values, _ := attr["values"].(map[string]any)
	redirect := StringFromAny(values["url"])
	if redirect == "" {
		return "", fmt.Errorf("login response missing redirect url: %s", trimForError(string(body)))
	}
	return redirect, nil
}

func (c *HaierClient) loadToken(ctx context.Context, redirectURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, redirectURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", haierUserAgent)
	resp, err := c.noRedirectClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if strings.Contains(text, "ProgressiveLogin") {
		match := urlRefRe.FindStringSubmatch(text)
		if len(match) > 1 && match[1] != "" {
			req2, err := http.NewRequestWithContext(ctx, http.MethodGet, match[1], nil)
			if err != nil {
				return "", err
			}
			req2.Header.Set("User-Agent", haierUserAgent)
			resp2, err := c.noRedirectClient().Do(req2)
			if err != nil {
				return "", err
			}
			defer resp2.Body.Close()
			body, _ = io.ReadAll(resp2.Body)
			text = string(body)
		}
	}
	match := urlRefRe.FindStringSubmatch(text)
	if len(match) < 2 {
		return "", fmt.Errorf("unable to find token redirect in auth response: %s", trimForError(text))
	}
	if strings.HasPrefix(match[1], "http") {
		return match[1], nil
	}
	return haierAuthAPI + match[1], nil
}

func (c *HaierClient) extractTokens(ctx context.Context, tokenURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", haierUserAgent)
	resp, err := c.noRedirectClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("token response failed: %s", strings.TrimSpace(string(body)))
	}
	if err := c.extractTokensFromText(string(body)); err != nil {
		return err
	}
	if c.auth.AccessToken == "" || c.auth.RefreshToken == "" || c.auth.IDToken == "" {
		return errors.New("token response did not contain access, refresh and id tokens")
	}
	c.auth.ExpiresAt = time.Now().Add(haierTimeWindowSec * time.Second)
	return nil
}

func (c *HaierClient) extractTokensFromText(text string) error {
	matches := tokenRe.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) != 3 {
			continue
		}
		switch match[1] {
		case "access_token":
			c.auth.AccessToken = match[2]
		case "refresh_token":
			c.auth.RefreshToken = urlSafeUnescape(match[2])
		case "id_token":
			c.auth.IDToken = match[2]
		}
	}
	if c.auth.AccessToken == "" || c.auth.RefreshToken == "" || c.auth.IDToken == "" {
		return errors.New("unable to parse auth tokens")
	}
	c.cfg.RefreshToken = c.auth.RefreshToken
	return nil
}

func (c *HaierClient) apiLogin(ctx context.Context) error {
	body := map[string]any{
		"appVersion":  haierAppVersion,
		"mobileId":    c.cfg.NormalizedMobileID(),
		"os":          haierOS,
		"osVersion":   haierOSVersion,
		"deviceModel": haierDeviceModel,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, haierAPIBase+"/auth/v1/login", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", haierUserAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("id-token", c.auth.IDToken)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("api auth failed: %s", strings.TrimSpace(string(bodyBytes)))
	}
	var payloadMap map[string]any
	if err := json.Unmarshal(bodyBytes, &payloadMap); err != nil {
		return fmt.Errorf("api auth decode failed: %w", err)
	}
	cognitoUser, _ := payloadMap["cognitoUser"].(map[string]any)
	token := StringFromAny(cognitoUser["Token"])
	if token == "" {
		return fmt.Errorf("api auth response missing cognito token: %s", trimForError(string(bodyBytes)))
	}
	c.auth.CognitoToken = token
	return nil
}
