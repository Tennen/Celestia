package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
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

type haierClient struct {
	cfg    AccountConfig
	client *http.Client
	auth   haierAuthState
}

type haierDeviceInfo struct {
	Appliance map[string]any
	Commands  map[string]any
	Model     map[string]any
}

func newHaierClient(cfg AccountConfig) (*haierClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &haierClient{
		cfg: cfg,
		client: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (c *haierClient) authenticated(ctx context.Context) bool {
	return c.auth.CognitoToken != "" && c.auth.IDToken != ""
}

func (c *haierClient) CurrentRefreshToken() string {
	return strings.TrimSpace(c.cfg.RefreshToken)
}

func (c *haierClient) authenticate(ctx context.Context) error {
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

func (c *haierClient) refresh(ctx context.Context, refreshToken string) error {
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
	c.auth.IDToken = stringFromAny(payload["id_token"])
	c.auth.AccessToken = stringFromAny(payload["access_token"])
	c.auth.RefreshToken = stringFromAny(payload["refresh_token"])
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

func (c *haierClient) loginWithPassword(ctx context.Context) error {
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

func (c *haierClient) introduce(ctx context.Context) (string, bool, error) {
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

func (c *haierClient) handleRedirects(ctx context.Context, loginURL string) (string, error) {
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

func (c *haierClient) followOnce(ctx context.Context, target string) (string, error) {
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

func (c *haierClient) loadLoginPage(ctx context.Context, loginURL string) (string, error) {
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

func (c *haierClient) submitLogin(ctx context.Context, loginURL string) (string, error) {
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
	redirect := stringFromAny(values["url"])
	if redirect == "" {
		return "", fmt.Errorf("login response missing redirect url: %s", trimForError(string(body)))
	}
	return redirect, nil
}

func (c *haierClient) loadToken(ctx context.Context, redirectURL string) (string, error) {
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

func (c *haierClient) extractTokens(ctx context.Context, tokenURL string) error {
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

func (c *haierClient) extractTokensFromText(text string) error {
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

func (c *haierClient) apiLogin(ctx context.Context) error {
	body := map[string]any{
		"appVersion":  haierAppVersion,
		"mobileId":    c.cfg.normalizedMobileID(),
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
	token := stringFromAny(cognitoUser["Token"])
	if token == "" {
		return fmt.Errorf("api auth response missing cognito token: %s", trimForError(string(bodyBytes)))
	}
	c.auth.CognitoToken = token
	return nil
}

func (c *haierClient) loadAppliances(ctx context.Context) ([]map[string]any, error) {
	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodGet, haierAPIBase+"/commands/v1/appliance", nil, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	result := []map[string]any{}
	if appliances, ok := payload["appliances"].([]any); ok {
		for _, raw := range appliances {
			if item, ok := raw.(map[string]any); ok {
				result = append(result, item)
			}
		}
		return result, nil
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		if appliances, ok := nested["appliances"].([]any); ok {
			for _, raw := range appliances {
				if item, ok := raw.(map[string]any); ok {
					result = append(result, item)
				}
			}
		}
	}
	return result, nil
}

func (c *haierClient) loadCommands(ctx context.Context, appliance map[string]any) (map[string]any, error) {
	params := url.Values{}
	params.Set("applianceType", stringFromAny(appliance["applianceTypeName"]))
	params.Set("applianceModelId", stringFromAny(appliance["applianceModelId"]))
	params.Set("macAddress", stringFromAny(appliance["macAddress"]))
	params.Set("os", haierOS)
	params.Set("appVersion", haierAppVersion)
	params.Set("code", stringFromAny(appliance["code"]))
	if firmwareID := stringFromAny(appliance["eepromId"]); firmwareID != "" {
		params.Set("firmwareId", firmwareID)
	}
	if firmwareVersion := stringFromAny(appliance["fwVersion"]); firmwareVersion != "" {
		params.Set("fwVersion", firmwareVersion)
	}
	if series := stringFromAny(appliance["series"]); series != "" {
		params.Set("series", series)
	}

	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodGet, haierAPIBase+"/commands/v1/retrieve?"+params.Encode(), nil, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	if resultCode := stringFromAny(payload["resultCode"]); resultCode != "" && resultCode != "0" {
		return nil, fmt.Errorf("command metadata request failed: resultCode=%s", resultCode)
	}
	return payload, nil
}

func (c *haierClient) loadAttributes(ctx context.Context, appliance map[string]any) (map[string]any, error) {
	params := url.Values{}
	params.Set("macAddress", stringFromAny(appliance["macAddress"]))
	params.Set("applianceType", stringFromAny(appliance["applianceTypeName"]))
	params.Set("category", "CYCLE")
	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodGet, haierAPIBase+"/commands/v1/context?"+params.Encode(), nil, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	return payload, nil
}

func (c *haierClient) loadStatistics(ctx context.Context, appliance map[string]any) (map[string]any, error) {
	params := url.Values{}
	params.Set("macAddress", stringFromAny(appliance["macAddress"]))
	params.Set("applianceType", stringFromAny(appliance["applianceTypeName"]))
	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodGet, haierAPIBase+"/commands/v1/statistics?"+params.Encode(), nil, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	return payload, nil
}

func (c *haierClient) loadMaintenance(ctx context.Context, appliance map[string]any) (map[string]any, error) {
	params := url.Values{}
	params.Set("macAddress", stringFromAny(appliance["macAddress"]))
	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodGet, haierAPIBase+"/commands/v1/maintenance-cycle?"+params.Encode(), nil, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	return payload, nil
}

func (c *haierClient) sendCommand(ctx context.Context, appliance map[string]any, command string, parameters map[string]any, ancillaryParameters map[string]any, programName string) (map[string]any, error) {
	now := time.Now().UTC()
	body := map[string]any{
		"macAddress":       stringFromAny(appliance["macAddress"]),
		"timestamp":        now.Format("2006-01-02T15:04:05.000Z"),
		"commandName":      command,
		"transactionId":    fmt.Sprintf("%s_%s", stringFromAny(appliance["macAddress"]), now.Format("2006-01-02T15:04:05.000Z")),
		"applianceOptions": applianceOptions(appliance),
		"device": map[string]any{
			"appVersion":  haierAppVersion,
			"mobileId":    c.cfg.normalizedMobileID(),
			"mobileOs":    haierOS,
			"osVersion":   haierOSVersion,
			"deviceModel": haierDeviceModel,
		},
		"attributes": map[string]any{
			"channel":     "mobileApp",
			"origin":      "standardProgram",
			"energyLabel": "0",
		},
		"ancillaryParameters": ancillaryParameters,
		"parameters":          parameters,
		"applianceType":       stringFromAny(appliance["applianceTypeName"]),
	}
	if programName != "" && command == "startProgram" {
		body["programName"] = strings.ToUpper(programName)
	}
	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodPost, haierAPIBase+"/commands/v1/send", body, nil, &payload); err != nil {
		return nil, err
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		payload = nested
	}
	if resultCode := resultCodeFrom(payload); resultCode != "" && resultCode != "0" {
		return payload, fmt.Errorf("command failed: resultCode=%s", resultCode)
	}
	return payload, nil
}

func (c *haierClient) requestJSON(ctx context.Context, method, target string, body any, headers map[string]string, out *map[string]any) error {
	if !c.authenticated(ctx) && !strings.Contains(target, "/auth/") {
		if err := c.authenticate(ctx); err != nil {
			return err
		}
	}
	return c.requestJSONWithRetry(ctx, method, target, body, headers, out, 0)
}

func (c *haierClient) requestJSONWithRetry(ctx context.Context, method, target string, body any, headers map[string]string, out *map[string]any, attempt int) error {
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
	req.Header.Set("X-TimezoneId", c.cfg.normalizedTimezone())
	req.Header.Set("X-Timezone", timezoneOffset(c.cfg.normalizedTimezone()))
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
			if err := c.authenticate(ctx); err != nil {
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

func applianceOptions(appliance map[string]any) map[string]any {
	if options, ok := appliance["options"].(map[string]any); ok {
		return options
	}
	if model, ok := appliance["applianceModel"].(map[string]any); ok {
		if options, ok := model["options"].(map[string]any); ok {
			return options
		}
	}
	return map[string]any{}
}

func stringFromAny(v any) string {
	switch raw := v.(type) {
	case string:
		return raw
	case fmt.Stringer:
		return raw.String()
	case float64:
		if raw == float64(int64(raw)) {
			return fmt.Sprintf("%d", int64(raw))
		}
		return fmt.Sprintf("%v", raw)
	case int:
		return fmt.Sprintf("%d", raw)
	case json.Number:
		return raw.String()
	default:
		return ""
	}
}

func resultCodeFrom(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if code := stringFromAny(payload["resultCode"]); code != "" {
		return code
	}
	if nested, ok := payload["payload"].(map[string]any); ok {
		return stringFromAny(nested["resultCode"])
	}
	return ""
}

func trimForError(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
}

func mustJSONString(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func mustJSONRawMessage(s string) json.RawMessage {
	return json.RawMessage([]byte(s))
}

func urlSafeUnescape(value string) string {
	if decoded, err := url.QueryUnescape(value); err == nil {
		return decoded
	}
	return value
}

func generateNonce() string {
	raw := fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()/97)
	return strings.ReplaceAll(raw, "--", "-")
}

func timezoneOffset(timezone string) string {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		_, offset := time.Now().Zone()
		return formatOffset(offset)
	}
	_, offset := time.Now().In(loc).Zone()
	return formatOffset(offset)
}

func formatOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes)
}

func (c *haierClient) noRedirectClient() *http.Client {
	clone := *c.client
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clone
}
