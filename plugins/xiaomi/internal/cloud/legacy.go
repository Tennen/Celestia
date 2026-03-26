package cloud

import (
	"context"
	"crypto/md5"
	"crypto/rc4"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/xiaomi/oauth"
)

const (
	accountBaseURL       = "https://account.xiaomi.com"
	legacyCallbackURL    = "https://sts.api.io.mi.com/sts"
	legacySID            = "xiaomiio"
	legacySDKVersion     = "3.8.6"
	legacyChannel        = "MI_APP_STORE"
	legacyProtocolHeader = "PROTOCAL-HTTP2"
	legacyUserAgentFmt   = "Android-7.1.1-1.0.0-ONEPLUS A3010-136-%s APP/xiaomi.smarthome APPV/62830"
)

type orderedParam struct {
	Key   string
	Value string
}

func legacyUserAgent(deviceID string) string {
	id := strings.TrimSpace(deviceID)
	if id == "" {
		id = "CELESTIA00000000"
	}
	return fmt.Sprintf(legacyUserAgentFmt, id)
}

func normalizeLocale(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "en_US"
	}
	return value
}

func normalizeTimezone(value string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	_, offset := time.Now().Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("GMT%s%02d:%02d", sign, offset/3600, (offset%3600)/60)
}

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
	jar, _ := cookiejar.New(nil)
	c.mu.Lock()
	c.loginClient = &http.Client{Timeout: 30 * time.Second, Jar: jar}
	c.serviceToken = ""
	c.ssecurity = ""
	c.userID = ""
	c.cuserID = ""
	verifyURL := strings.TrimSpace(c.cfg.VerifyURL)
	verifyTicket := strings.TrimSpace(c.cfg.VerifyTicket)
	c.mu.Unlock()

	if verifyURL != "" && verifyTicket != "" {
		if err := c.loginWithVerification(ctx, verifyURL, verifyTicket); err == nil {
			return nil
		} else {
			return err
		}
	}

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

func (c *Client) legacyListDevices(ctx context.Context, selectedHomeIDs []string) ([]DeviceRecord, error) {
	if err := c.ensureLegacySession(ctx); err != nil {
		return nil, err
	}
	homeInfos, err := c.legacyHomeInfos(ctx)
	if err != nil {
		return nil, err
	}

	selected := map[string]bool{}
	for _, id := range selectedHomeIDs {
		selected[id] = true
	}

	deviceMeta := map[string]DeviceRecord{}
	homeEntries := make([]legacyHomeEntry, 0, len(homeInfos.HomeList))
	for homeID, home := range homeInfos.HomeList {
		if len(selected) > 0 && !selected[homeID] {
			continue
		}
		homeEntries = append(homeEntries, legacyHomeEntry{
			HomeID:   homeID,
			UID:      home.UID,
			HomeName: home.HomeName,
			GroupID:  home.GroupID,
		})
		for _, did := range home.DIDs {
			deviceMeta[did] = DeviceRecord{
				DID:      did,
				HomeID:   homeID,
				HomeName: home.HomeName,
				RoomID:   homeID,
				RoomName: home.HomeName,
				GroupID:  home.GroupID,
				Region:   oauth.NormalizeRegion(c.cfg.Region),
			}
		}
		for roomID, room := range home.RoomInfo {
			for _, did := range room.DIDs {
				deviceMeta[did] = DeviceRecord{
					DID:      did,
					HomeID:   homeID,
					HomeName: home.HomeName,
					RoomID:   roomID,
					RoomName: room.RoomName,
					GroupID:  home.GroupID,
					Region:   oauth.NormalizeRegion(c.cfg.Region),
				}
			}
		}
	}
	if len(deviceMeta) == 0 {
		return nil, nil
	}

	deviceInfos, err := c.legacyDeviceInfos(ctx, homeEntries)
	if err != nil {
		return nil, err
	}

	devices := make([]DeviceRecord, 0, len(deviceInfos))
	for did, info := range deviceInfos {
		meta, ok := deviceMeta[did]
		if !ok {
			continue
		}
		meta.Name = stringValue(info["name"])
		meta.UID = stringValue(info["uid"])
		meta.URN = stringValue(info["spec_type"])
		meta.Model = stringValue(info["model"])
		meta.Online = boolValue(info["isOnline"])
		meta.VoiceCtrl = intValue(info["voice_ctrl"])
		if meta.RoomName == "" {
			meta.RoomName = meta.HomeName
		}
		devices = append(devices, meta)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	return devices, nil
}

type legacyHomeEntry struct {
	HomeID   string
	UID      string
	HomeName string
	GroupID  string
}

func (c *Client) legacyHomeInfos(ctx context.Context) (homeInfosResponse, error) {
	var body struct {
		Result struct {
			HomeList []struct {
				ID       any      `json:"id"`
				UID      any      `json:"uid"`
				Name     string   `json:"name"`
				DIDs     []string `json:"dids"`
				RoomList []struct {
					ID   any      `json:"id"`
					Name string   `json:"name"`
					DIDs []string `json:"dids"`
				} `json:"roomlist"`
			} `json:"homelist"`
		} `json:"result"`
	}
	if err := c.legacyAPIJSON(ctx, http.MethodPost, "v2/homeroom/gethome_merged", map[string]any{
		"fg":              true,
		"fetch_share":     true,
		"fetch_share_dev": true,
		"fetch_cariot":    true,
		"limit":           300,
		"app_ver":         7,
		"plat_form":       0,
	}, &body); err != nil {
		return homeInfosResponse{}, err
	}
	out := homeInfosResponse{
		HomeList:      map[string]homeInfo{},
		ShareHomeList: map[string]homeInfo{},
	}
	for _, item := range body.Result.HomeList {
		homeID := stringify(item.ID)
		out.HomeList[homeID] = parseHomeInfo(item.Name, item.UID, item.DIDs, item.RoomList, homeID)
	}
	return out, nil
}

func (c *Client) legacyDeviceInfos(ctx context.Context, homes []legacyHomeEntry) (map[string]map[string]any, error) {
	result := map[string]map[string]any{}
	for _, home := range homes {
		start := ""
		for {
			var body struct {
				Result struct {
					DeviceInfo []map[string]any `json:"device_info"`
					HasMore    bool             `json:"has_more"`
					MaxDID     string           `json:"max_did"`
				} `json:"result"`
			}
			if err := c.legacyAPIJSON(ctx, http.MethodPost, "v2/home/home_device_list", map[string]any{
				"home_owner":         home.UID,
				"home_id":            home.HomeID,
				"limit":              300,
				"start_did":          start,
				"get_split_device":   false,
				"support_smart_home": true,
				"get_cariot_device":  true,
				"get_third_device":   true,
			}, &body); err != nil {
				return nil, err
			}
			for _, item := range body.Result.DeviceInfo {
				did := stringValue(item["did"])
				if did == "" {
					continue
				}
				result[did] = item
			}
			if !body.Result.HasMore || body.Result.MaxDID == "" {
				break
			}
			start = body.Result.MaxDID
		}
	}
	return result, nil
}

func (c *Client) legacyAPIJSON(ctx context.Context, method, api string, payload map[string]any, out any) error {
	raw, err := c.doLegacyRequest(ctx, method, api, payload, true)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func (c *Client) doLegacyRequest(ctx context.Context, method, api string, payload map[string]any, allowRetry bool) ([]byte, error) {
	if err := c.ensureLegacySession(ctx); err != nil {
		return nil, err
	}

	endpoint := c.legacyAPIURL(api)
	params := make([]orderedParam, 0, 1)
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		params = append(params, orderedParam{Key: "data", Value: string(data)})
	}

	formParams, nonce, err := c.buildRC4Params(method, endpoint, params)
	if err != nil {
		return nil, err
	}

	body := url.Values{}
	for _, item := range formParams {
		body.Set(item.Key, item.Value)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-XIAOMI-PROTOCAL-FLAG-CLI", legacyProtocolHeader)
	req.Header.Set("MIOT-ENCRYPT-ALGORITHM", "ENCRYPT-RC4")
	req.Header.Set("Accept-Encoding", "identity")
	c.addLegacyCookies(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		c.clearLegacySession()
		if allowRetry && strings.TrimSpace(c.cfg.Username) != "" && strings.TrimSpace(c.cfg.Password) != "" {
			if err := c.loginWithPassword(ctx); err != nil {
				return nil, err
			}
			return c.doLegacyRequest(ctx, method, api, payload, false)
		}
		return nil, fmt.Errorf("xiaomi legacy request failed: %s", resp.Status)
	}

	decoded, err := c.decodeLegacyBody(nonce, rawBody)
	if err != nil {
		return nil, err
	}
	if legacyAuthExpired(decoded) {
		c.clearLegacySession()
		if allowRetry && strings.TrimSpace(c.cfg.Username) != "" && strings.TrimSpace(c.cfg.Password) != "" {
			if err := c.loginWithPassword(ctx); err != nil {
				return nil, err
			}
			return c.doLegacyRequest(ctx, method, api, payload, false)
		}
	}
	return decoded, nil
}

func (c *Client) clearLegacySession() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serviceToken = ""
	c.ssecurity = ""
	c.userID = ""
	c.cuserID = ""
}

func (c *Client) addLegacyCookies(req *http.Request) {
	c.mu.Lock()
	userID := c.userID
	serviceToken := c.serviceToken
	c.mu.Unlock()
	req.AddCookie(&http.Cookie{Name: "userId", Value: userID})
	req.AddCookie(&http.Cookie{Name: "yetAnotherServiceToken", Value: serviceToken})
	req.AddCookie(&http.Cookie{Name: "serviceToken", Value: serviceToken})
	req.AddCookie(&http.Cookie{Name: "locale", Value: c.locale})
	req.AddCookie(&http.Cookie{Name: "timezone", Value: c.timezone})
	req.AddCookie(&http.Cookie{Name: "is_daylight", Value: fmt.Sprintf("%d", boolToInt(time.Now().IsDST()))})
	req.AddCookie(&http.Cookie{Name: "dst_offset", Value: fmt.Sprintf("%d", boolToInt(time.Now().IsDST())*60*60*1000)})
	req.AddCookie(&http.Cookie{Name: "channel", Value: legacyChannel})
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func (c *Client) legacyAPIURL(api string) string {
	api = strings.TrimPrefix(api, "/")
	return c.legacyBaseURL + "/" + api
}

func (c *Client) buildRC4Params(method, endpoint string, params []orderedParam) ([]orderedParam, string, error) {
	nonce, err := generateNonce()
	if err != nil {
		return nil, "", err
	}
	signedNonce, err := c.signedNonce(nonce)
	if err != nil {
		return nil, "", err
	}

	params = append(params, orderedParam{
		Key:   "rc4_hash__",
		Value: sha1Sign(method, endpoint, params, signedNonce),
	})

	encrypted := make([]orderedParam, 0, len(params)+3)
	for _, item := range params {
		value, err := encryptRC4(signedNonce, item.Value)
		if err != nil {
			return nil, "", err
		}
		encrypted = append(encrypted, orderedParam{Key: item.Key, Value: value})
	}
	encrypted = append(encrypted,
		orderedParam{Key: "signature", Value: sha1Sign(method, endpoint, encrypted, signedNonce)},
		orderedParam{Key: "ssecurity", Value: c.currentSSecurity()},
		orderedParam{Key: "_nonce", Value: nonce},
	)
	return encrypted, nonce, nil
}

func (c *Client) currentSSecurity() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ssecurity
}

func (c *Client) signedNonce(nonce string) (string, error) {
	c.mu.Lock()
	ssecurity := c.ssecurity
	c.mu.Unlock()
	if ssecurity == "" {
		return "", errors.New("xiaomi legacy session missing ssecurity")
	}
	key, err := base64.StdEncoding.DecodeString(ssecurity)
	if err != nil {
		return "", err
	}
	nonceBytes, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(append(key, nonceBytes...))
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

func sha1Sign(method, endpoint string, params []orderedParam, nonce string) string {
	path := mustURLPath(endpoint)
	if strings.HasPrefix(path, "/app/") {
		path = path[4:]
	}
	parts := []string{strings.ToUpper(method), path}
	for _, item := range params {
		parts = append(parts, item.Key+"="+item.Value)
	}
	parts = append(parts, nonce)
	sum := sha1.Sum([]byte(strings.Join(parts, "&")))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func mustURLPath(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	return parsed.Path
}

func encryptRC4(passwordB64, plain string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(passwordB64)
	if err != nil {
		return "", err
	}
	cipher, err := rc4.NewCipher(key)
	if err != nil {
		return "", err
	}
	drop := make([]byte, 1024)
	cipher.XORKeyStream(drop, drop)
	out := make([]byte, len(plain))
	cipher.XORKeyStream(out, []byte(plain))
	return base64.StdEncoding.EncodeToString(out), nil
}

func decryptRC4(passwordB64 string, ciphertext []byte) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(passwordB64)
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(ciphertext)))
	if err != nil {
		return nil, err
	}
	cipher, err := rc4.NewCipher(key)
	if err != nil {
		return nil, err
	}
	drop := make([]byte, 1024)
	cipher.XORKeyStream(drop, drop)
	out := make([]byte, len(data))
	cipher.XORKeyStream(out, data)
	return out, nil
}

func generateNonce() (string, error) {
	random := make([]byte, 8)
	rand.New(rand.NewSource(time.Now().UnixNano())).Read(random)
	minutes := uint32(time.Now().UnixMilli() / 60000)
	suffix := make([]byte, 4)
	binary.BigEndian.PutUint32(suffix, minutes)
	return base64.StdEncoding.EncodeToString(append(random, suffix...)), nil
}

func (c *Client) decodeLegacyBody(nonce string, raw []byte) ([]byte, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return nil, errors.New("xiaomi legacy request returned empty body")
	}
	if strings.HasPrefix(text, "{") {
		return []byte(text), nil
	}
	signedNonce, err := c.signedNonce(nonce)
	if err != nil {
		return nil, err
	}
	decoded, err := decryptRC4(signedNonce, raw)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func legacyAuthExpired(raw []byte) bool {
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(body.Message))
	return body.Code == 2 || body.Code == 3 || strings.Contains(message, "auth err") || strings.Contains(message, "servicetoken_expired") || strings.Contains(message, "invalid signature")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
