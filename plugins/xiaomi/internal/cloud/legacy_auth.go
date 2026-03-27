package cloud

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

func (c *Client) ensureLegacySession(ctx context.Context) error {
	c.mu.Lock()
	hasSession := c.serviceToken != "" && c.ssecurity != "" && c.userID != ""
	username := strings.TrimSpace(c.cfg.Username)
	password := strings.TrimSpace(c.cfg.Password)
	c.mu.Unlock()

	if hasSession {
		return nil
	}
	if username == "" || password == "" {
		return fmt.Errorf("xiaomi account %q requires username/password or service_token/ssecurity/user_id", c.cfg.Name)
	}
	return c.loginWithPassword(ctx)
}

func (c *Client) loginWithPassword(ctx context.Context) error {
	c.mu.Lock()
	verifyURL := strings.TrimSpace(c.cfg.VerifyURL)
	verifyTicket := strings.TrimSpace(c.cfg.VerifyTicket)
	if verifyURL != "" && verifyTicket != "" {
		if c.loginClient == nil || c.loginClient.Jar == nil {
			jar, _ := cookiejar.New(nil)
			c.loginClient = &http.Client{Timeout: 30 * time.Second, Jar: jar}
		}
		c.serviceToken = ""
		c.ssecurity = ""
		c.userID = ""
		c.cuserID = ""
		c.mu.Unlock()

		if err := c.loginWithVerification(ctx, verifyURL, verifyTicket); err == nil {
			return nil
		} else {
			return err
		}
	}

	jar, _ := cookiejar.New(nil)
	c.loginClient = &http.Client{Timeout: 30 * time.Second, Jar: jar}
	c.serviceToken = ""
	c.ssecurity = ""
	c.userID = ""
	c.cuserID = ""
	c.mu.Unlock()

	auth, err := c.loginStep1(ctx)
	if err != nil {
		return err
	}
	location, err := c.loginStep2(ctx, auth)
	if err != nil {
		return err
	}
	return c.loginStep3(ctx, location)
}

func (c *Client) loginWithVerification(ctx context.Context, verifyURL, verifyTicket string) error {
	location, err := c.verifyTicket(ctx, verifyURL, verifyTicket)
	if err != nil {
		return err
	}
	if location != "" {
		resp, err := c.accountRequest(ctx, http.MethodGet, location, nil, nil, nil)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
	}
	auth, err := c.loginStep1(ctx)
	if err != nil {
		return err
	}
	if location = stringValue(auth["location"]); location == "" {
		location, err = c.loginStep2(ctx, auth)
		if err != nil {
			return err
		}
	}
	return c.loginStep3(ctx, location)
}

func (c *Client) loginStep1(ctx context.Context) (map[string]any, error) {
	query := url.Values{}
	query.Set("sid", c.sid)
	query.Set("_json", "true")
	resp, err := c.accountRequest(ctx, http.MethodGet, "/pass/serviceLogin", query, nil, []*http.Cookie{
		{Name: "sdkVersion", Value: legacySDKVersion},
		{Name: "deviceId", Value: c.deviceID},
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	auth, err := decodeAccountJSON(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error getting xiaomi login sign: %w", err)
	}
	if code := intValue(auth["code"]); code == 0 {
		c.mu.Lock()
		c.userID = stringValue(auth["userId"])
		c.cuserID = stringValue(auth["cUserId"])
		if value := stringValue(auth["ssecurity"]); value != "" {
			c.ssecurity = value
		}
		c.mu.Unlock()
	}
	return auth, nil
}

func (c *Client) loginStep2(ctx context.Context, auth map[string]any) (string, error) {
	form := url.Values{}
	form.Set("user", strings.TrimSpace(c.cfg.Username))
	form.Set("hash", strings.ToUpper(fmt.Sprintf("%x", md5.Sum([]byte(strings.TrimSpace(c.cfg.Password))))))
	form.Set("callback", stringValue(auth["callback"]))
	form.Set("sid", firstNonEmpty(stringValue(auth["sid"]), c.sid))
	form.Set("qs", stringValue(auth["qs"]))
	form.Set("_sign", stringValue(auth["_sign"]))

	query := url.Values{}
	query.Set("_json", "true")
	resp, err := c.accountRequest(ctx, http.MethodPost, "/pass/serviceLoginAuth2", query, form, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := decodeAccountJSON(resp.Body)
	if err != nil {
		return "", err
	}
	location := stringValue(body["location"])
	if location == "" {
		if verifyURL := stringValue(body["notificationUrl"]); verifyURL != "" {
			if !strings.HasPrefix(verifyURL, "http") {
				verifyURL = accountBaseURL + verifyURL
			}
			return "", fmt.Errorf("xiaomi account %q requires secondary verification at %s", c.cfg.Name, verifyURL)
		}
		if captchaURL := stringValue(body["captchaUrl"]); captchaURL != "" {
			if !strings.HasPrefix(captchaURL, "http") {
				captchaURL = accountBaseURL + captchaURL
			}
			return "", fmt.Errorf("xiaomi account %q requires captcha at %s", c.cfg.Name, captchaURL)
		}
		message := strings.TrimSpace(firstNonEmpty(stringValue(body["description"]), stringValue(body["desc"]), stringValue(body["message"])))
		if message == "" {
			message = string(respStatusText(resp))
		}
		return "", fmt.Errorf("xiaomi account %q login failed: %s", c.cfg.Name, message)
	}

	c.mu.Lock()
	c.userID = stringValue(body["userId"])
	c.cuserID = stringValue(body["cUserId"])
	c.ssecurity = stringValue(body["ssecurity"])
	c.mu.Unlock()

	return location, nil
}

func (c *Client) loginStep3(ctx context.Context, location string) error {
	resp, err := c.accountRequest(ctx, http.MethodGet, location, nil, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	serviceToken := cookieValue(resp, c.loginClient.Jar, "serviceToken")
	if serviceToken == "" {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("xiaomi login failed: serviceToken missing (%s)", strings.TrimSpace(string(data)))
	}

	c.mu.Lock()
	c.serviceToken = serviceToken
	if value := cookieValue(resp, c.loginClient.Jar, "userId"); value != "" {
		c.userID = value
	}
	if value := cookieValue(resp, c.loginClient.Jar, "cUserId"); value != "" {
		c.cuserID = value
	}
	c.cfg.ServiceToken = c.serviceToken
	c.cfg.SSecurity = c.ssecurity
	c.cfg.UserID = c.userID
	c.cfg.CUserID = c.cuserID
	c.cfg.VerifyURL = ""
	c.cfg.VerifyTicket = ""
	c.mu.Unlock()
	return nil
}

func (c *Client) accountRequest(ctx context.Context, method, target string, query url.Values, form url.Values, cookies []*http.Cookie) (*http.Response, error) {
	loginClient := c.loginClient
	if loginClient == nil {
		jar, _ := cookiejar.New(nil)
		loginClient = &http.Client{Timeout: 30 * time.Second, Jar: jar}
		c.loginClient = loginClient
	}

	if !strings.HasPrefix(target, "http") {
		target = accountBaseURL + target
	}
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	return loginClient.Do(req)
}

func decodeAccountJSON(r io.Reader) (map[string]any, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(strings.ReplaceAll(string(data), "&&&START&&&", ""))
	if text == "" {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func cookieValue(resp *http.Response, jar http.CookieJar, name string) string {
	for _, cookie := range resp.Cookies() {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	if jar != nil {
		for _, cookie := range jar.Cookies(resp.Request.URL) {
			if cookie.Name == name {
				return cookie.Value
			}
		}
	}
	return ""
}

func (c *Client) verifyTicket(ctx context.Context, verifyURL, ticket string) (string, error) {
	options, identitySession, err := c.checkIdentityList(ctx, verifyURL)
	if err != nil {
		return "", err
	}
	for _, flag := range options {
		api := ""
		switch flag {
		case 4:
			api = "/identity/auth/verifyPhone"
		case 8:
			api = "/identity/auth/verifyEmail"
		default:
			continue
		}
		query := url.Values{}
		query.Set("_dc", fmt.Sprintf("%d", time.Now().UnixMilli()))
		form := url.Values{}
		form.Set("_flag", fmt.Sprintf("%d", flag))
		form.Set("ticket", ticket)
		form.Set("trust", "true")
		form.Set("_json", "true")
		resp, err := c.accountRequest(ctx, http.MethodPost, api, query, form, []*http.Cookie{
			{Name: "identity_session", Value: identitySession},
		})
		if err != nil {
			return "", err
		}
		body, readErr := decodeAccountJSON(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return "", readErr
		}
		if intValue(body["code"]) == 0 {
			return stringValue(body["location"]), nil
		}
	}
	return "", errors.New("xiaomi verification failed: verification code rejected or expired")
}

func (c *Client) checkIdentityList(ctx context.Context, verifyURL string) ([]int, string, error) {
	listURL := strings.TrimSpace(verifyURL)
	if listURL == "" {
		return nil, "", errors.New("xiaomi verification requires verify_url")
	}
	listURL = strings.Replace(listURL, "fe/service/identity/authStart", "identity/list", 1)
	resp, err := c.accountRequest(ctx, http.MethodGet, listURL, nil, nil, nil)
	if err != nil {
		return nil, "", err
	}
	body, readErr := decodeAccountJSON(resp.Body)
	identitySession := cookieValue(resp, c.loginClient.Jar, "identity_session")
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, "", readErr
	}
	if identitySession == "" {
		return nil, "", errors.New("xiaomi verification session unavailable")
	}
	var options []int
	if raw, ok := body["options"].([]any); ok {
		for _, item := range raw {
			options = append(options, intValue(item))
		}
	}
	if len(options) == 0 {
		if flag := intValue(body["flag"]); flag != 0 {
			options = append(options, flag)
		}
	}
	if len(options) == 0 {
		options = append(options, 4)
	}
	return options, identitySession, nil
}

func respStatusText(resp *http.Response) []byte {
	if resp == nil {
		return []byte("unknown response")
	}
	return []byte(resp.Status)
}
