package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/xiaomi/oauth"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
)

const (
	defaultOAuthAPIHost  = "ha.api.io.mi.com"
	defaultLegacyAPIHost = "api.io.mi.com"
	profileURL           = "https://open.account.xiaomi.com/user/profile"
)

type AccountConfig struct {
	Name         string
	Region       string
	Username     string
	Password     string
	VerifyURL    string
	VerifyTicket string
	ClientID     string
	RedirectURL  string
	AccessToken  string
	RefreshToken string
	AuthCode     string
	DeviceID     string
	ServiceToken string
	SSecurity    string
	UserID       string
	CUserID      string
	HomeIDs      []string
	Locale       string
	Timezone     string
	ExpiresAt    time.Time
}

type DeviceRecord struct {
	DID       string
	UID       string
	Name      string
	URN       string
	Model     string
	HomeID    string
	HomeName  string
	RoomID    string
	RoomName  string
	GroupID   string
	Online    bool
	VoiceCtrl int
	Region    string
}

type Client struct {
	httpClient  *http.Client
	loginClient *http.Client
	authClient  *oauth.Client
	cfg         AccountConfig

	oauthBaseURL  string
	legacyBaseURL string
	clientID      string
	redirectURL   string
	deviceID      string
	userAgent     string
	locale        string
	timezone      string
	sid           string

	mu           sync.Mutex
	accessToken  string
	refreshToken string
	expiresAt    time.Time

	serviceToken string
	ssecurity    string
	userID       string
	cuserID      string
}

func NewClient(cfg AccountConfig, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	jar, _ := cookiejar.New(nil)
	region := oauth.NormalizeRegion(cfg.Region)
	oauthHost := defaultOAuthAPIHost
	legacyHost := defaultLegacyAPIHost
	if region != "cn" {
		oauthHost = region + "." + defaultOAuthAPIHost
		legacyHost = region + "." + defaultLegacyAPIHost
	}
	deviceID := strings.TrimSpace(cfg.DeviceID)
	if deviceID == "" {
		deviceID = "CELESTIA00000000"
	}
	return &Client{
		httpClient:    httpClient,
		loginClient:   &http.Client{Timeout: 30 * time.Second, Jar: jar},
		authClient:    oauth.NewClient(httpClient),
		cfg:           cfg,
		oauthBaseURL:  "https://" + oauthHost,
		legacyBaseURL: "https://" + legacyHost + "/app",
		clientID:      strings.TrimSpace(cfg.ClientID),
		redirectURL:   strings.TrimSpace(cfg.RedirectURL),
		deviceID:      deviceID,
		userAgent:     legacyUserAgent(deviceID),
		locale:        normalizeLocale(cfg.Locale),
		timezone:      normalizeTimezone(cfg.Timezone),
		sid:           legacySID,
		accessToken:   strings.TrimSpace(cfg.AccessToken),
		refreshToken:  strings.TrimSpace(cfg.RefreshToken),
		expiresAt:     cfg.ExpiresAt,
		serviceToken:  strings.TrimSpace(cfg.ServiceToken),
		ssecurity:     strings.TrimSpace(cfg.SSecurity),
		userID:        strings.TrimSpace(cfg.UserID),
		cuserID:       strings.TrimSpace(cfg.CUserID),
	}
}

func (c *Client) UpdateConfig(cfg AccountConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cfg = cfg
	c.clientID = strings.TrimSpace(cfg.ClientID)
	c.redirectURL = strings.TrimSpace(cfg.RedirectURL)
	if deviceID := strings.TrimSpace(cfg.DeviceID); deviceID != "" {
		c.deviceID = deviceID
		c.userAgent = legacyUserAgent(deviceID)
	}
	c.locale = normalizeLocale(cfg.Locale)
	c.timezone = normalizeTimezone(cfg.Timezone)

	if value := strings.TrimSpace(cfg.AccessToken); value != "" {
		c.accessToken = value
	}
	if value := strings.TrimSpace(cfg.RefreshToken); value != "" {
		c.refreshToken = value
	}
	if !cfg.ExpiresAt.IsZero() {
		c.expiresAt = cfg.ExpiresAt
	}
	if value := strings.TrimSpace(cfg.ServiceToken); value != "" {
		c.serviceToken = value
	}
	if value := strings.TrimSpace(cfg.SSecurity); value != "" {
		c.ssecurity = value
	}
	if value := strings.TrimSpace(cfg.UserID); value != "" {
		c.userID = value
	}
	if value := strings.TrimSpace(cfg.CUserID); value != "" {
		c.cuserID = value
	}
}

func (c *Client) usesLegacyAuth() bool {
	return strings.TrimSpace(c.cfg.Username) != "" ||
		strings.TrimSpace(c.cfg.Password) != "" ||
		strings.TrimSpace(c.cfg.VerifyURL) != "" ||
		strings.TrimSpace(c.cfg.VerifyTicket) != "" ||
		strings.TrimSpace(c.cfg.ServiceToken) != "" ||
		strings.TrimSpace(c.cfg.SSecurity) != "" ||
		strings.TrimSpace(c.cfg.UserID) != ""
}

func (c *Client) EnsureAuth(ctx context.Context) error {
	if c.usesLegacyAuth() {
		return c.ensureLegacySession(ctx)
	}
	return c.ensureOAuthToken(ctx)
}

func (c *Client) ensureOAuthToken(ctx context.Context) error {
	c.mu.Lock()
	accessToken := c.accessToken
	refreshToken := c.refreshToken
	expiresAt := c.expiresAt
	authCode := strings.TrimSpace(c.cfg.AuthCode)
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

func (c *Client) AccessToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accessToken
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

func (c *Client) ListDevices(ctx context.Context, selectedHomeIDs []string) ([]DeviceRecord, error) {
	if c.usesLegacyAuth() {
		return c.legacyListDevices(ctx, selectedHomeIDs)
	}
	if err := c.ensureOAuthToken(ctx); err != nil {
		return nil, err
	}
	homeInfos, err := c.oauthHomeInfos(ctx)
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, id := range selectedHomeIDs {
		selected[id] = true
	}

	deviceMeta := map[string]DeviceRecord{}
	for _, bucket := range []map[string]homeInfo{homeInfos.HomeList, homeInfos.ShareHomeList} {
		for homeID, home := range bucket {
			if len(selected) > 0 && !selected[homeID] {
				continue
			}
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
	}
	if len(deviceMeta) == 0 {
		return nil, nil
	}

	deviceInfos, err := c.oauthDeviceInfos(ctx, sortedKeys(deviceMeta))
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
		meta.URN = stringValue(info["urn"])
		meta.Model = stringValue(info["model"])
		meta.Online = boolValue(info["online"])
		meta.VoiceCtrl = intValue(info["voice_ctrl"])
		if meta.RoomName == "" {
			meta.RoomName = meta.HomeName
		}
		devices = append(devices, meta)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	return devices, nil
}

func (c *Client) GetProps(ctx context.Context, params []map[string]any) ([]map[string]any, error) {
	if c.usesLegacyAuth() {
		var body struct {
			Result []map[string]any `json:"result"`
		}
		if err := c.legacyAPIJSON(ctx, http.MethodPost, "miotspec/prop/get", map[string]any{
			"params": params,
		}, &body); err != nil {
			return nil, err
		}
		return body.Result, nil
	}
	var body struct {
		Result []map[string]any `json:"result"`
	}
	if err := c.oauthAPIJSON(ctx, "/app/v2/miotspec/prop/get", map[string]any{
		"datasource": 1,
		"params":     params,
	}, &body); err != nil {
		return nil, err
	}
	return body.Result, nil
}

func (c *Client) SetProps(ctx context.Context, params []map[string]any) ([]map[string]any, error) {
	if c.usesLegacyAuth() {
		var body struct {
			Result []map[string]any `json:"result"`
		}
		if err := c.legacyAPIJSON(ctx, http.MethodPost, "miotspec/prop/set", map[string]any{
			"params": params,
		}, &body); err != nil {
			return nil, err
		}
		return body.Result, nil
	}
	var body struct {
		Result []map[string]any `json:"result"`
	}
	if err := c.oauthAPIJSON(ctx, "/app/v2/miotspec/prop/set", map[string]any{
		"params": params,
	}, &body); err != nil {
		return nil, err
	}
	return body.Result, nil
}

func (c *Client) Action(ctx context.Context, did string, siid, aiid int, inputs []any) (map[string]any, error) {
	if c.usesLegacyAuth() {
		var body struct {
			Result map[string]any `json:"result"`
		}
		if err := c.legacyAPIJSON(ctx, http.MethodPost, "miotspec/action", map[string]any{
			"params": map[string]any{
				"did":  did,
				"siid": siid,
				"aiid": aiid,
				"in":   inputs,
			},
		}, &body); err != nil {
			return nil, err
		}
		return body.Result, nil
	}
	var body struct {
		Result map[string]any `json:"result"`
	}
	if err := c.oauthAPIJSON(ctx, "/app/v2/miotspec/action", map[string]any{
		"params": map[string]any{
			"did":  did,
			"siid": siid,
			"aiid": aiid,
			"in":   inputs,
		},
	}, &body); err != nil {
		return nil, err
	}
	return body.Result, nil
}

func (c *Client) SpecInstance(ctx context.Context, urn string) (spec.Instance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://miot-spec.org/miot-spec-v2/instance", nil)
	if err != nil {
		return spec.Instance{}, err
	}
	query := req.URL.Query()
	query.Set("type", urn)
	req.URL.RawQuery = query.Encode()
	var instance spec.Instance
	if err := c.do(req, &instance); err != nil {
		return spec.Instance{}, err
	}
	if instance.Type == "" || len(instance.Services) == 0 {
		return spec.Instance{}, fmt.Errorf("xiaomi spec unavailable for %s", urn)
	}
	return instance, nil
}

type homeInfosResponse struct {
	HomeList      map[string]homeInfo
	ShareHomeList map[string]homeInfo
}

type homeInfo struct {
	UID      string
	HomeName string
	GroupID  string
	DIDs     []string
	RoomInfo map[string]roomInfo
}

type roomInfo struct {
	RoomName string
	DIDs     []string
}

func (c *Client) oauthHomeInfos(ctx context.Context) (homeInfosResponse, error) {
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
			ShareHomeList []struct {
				ID       any      `json:"id"`
				UID      any      `json:"uid"`
				Name     string   `json:"name"`
				DIDs     []string `json:"dids"`
				RoomList []struct {
					ID   any      `json:"id"`
					Name string   `json:"name"`
					DIDs []string `json:"dids"`
				} `json:"roomlist"`
			} `json:"share_home_list"`
		} `json:"result"`
	}
	if err := c.oauthAPIJSON(ctx, "/app/v2/homeroom/gethome", map[string]any{
		"limit":           150,
		"fetch_share":     true,
		"fetch_share_dev": true,
		"plat_form":       0,
		"app_ver":         9,
	}, &body); err != nil {
		return homeInfosResponse{}, err
	}
	out := homeInfosResponse{
		HomeList:      map[string]homeInfo{},
		ShareHomeList: map[string]homeInfo{},
	}
	for _, item := range body.Result.HomeList {
		out.HomeList[stringify(item.ID)] = parseHomeInfo(item.Name, item.UID, item.DIDs, item.RoomList, stringify(item.ID))
	}
	for _, item := range body.Result.ShareHomeList {
		out.ShareHomeList[stringify(item.ID)] = parseHomeInfo(item.Name, item.UID, item.DIDs, item.RoomList, stringify(item.ID))
	}
	return out, nil
}

func parseHomeInfo(name string, uid any, dids []string, rooms []struct {
	ID   any      `json:"id"`
	Name string   `json:"name"`
	DIDs []string `json:"dids"`
}, homeID string) homeInfo {
	info := homeInfo{
		UID:      stringify(uid),
		HomeName: name,
		GroupID:  stringify(uid) + ":" + homeID,
		DIDs:     dids,
		RoomInfo: map[string]roomInfo{},
	}
	for _, room := range rooms {
		info.RoomInfo[stringify(room.ID)] = roomInfo{
			RoomName: room.Name,
			DIDs:     room.DIDs,
		}
	}
	return info
}

func (c *Client) oauthDeviceInfos(ctx context.Context, dids []string) (map[string]map[string]any, error) {
	result := map[string]map[string]any{}
	start := ""
	for {
		payload := map[string]any{
			"limit":            200,
			"get_split_device": true,
			"get_third_device": true,
			"dids":             dids,
		}
		if start != "" {
			payload["start_did"] = start
		}
		var body struct {
			Result struct {
				List         []map[string]any `json:"list"`
				HasMore      bool             `json:"has_more"`
				NextStartDID string           `json:"next_start_did"`
			} `json:"result"`
		}
		if err := c.oauthAPIJSON(ctx, "/app/v2/home/device_list_page", payload, &body); err != nil {
			return nil, err
		}
		for _, item := range body.Result.List {
			did := stringValue(item["did"])
			if did == "" {
				continue
			}
			result[did] = map[string]any{
				"did":        did,
				"uid":        item["uid"],
				"name":       item["name"],
				"urn":        item["spec_type"],
				"model":      item["model"],
				"online":     item["isOnline"],
				"voice_ctrl": item["voice_ctrl"],
			}
		}
		if !body.Result.HasMore || body.Result.NextStartDID == "" {
			break
		}
		start = body.Result.NextStartDID
	}
	return result, nil
}

func (c *Client) oauthAPIJSON(ctx context.Context, path string, payload map[string]any, out any) error {
	if err := c.ensureOAuthToken(ctx); err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.oauthBaseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Host", strings.TrimPrefix(strings.TrimPrefix(c.oauthBaseURL, "https://"), "http://"))
	req.Header.Set("X-Client-BizId", "haapi")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.AccessToken())
	req.Header.Set("X-Client-AppId", c.clientID)

	var envelope struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Result  json.RawMessage `json:"result"`
	}
	if err := c.do(req, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		return fmt.Errorf("xiaomi api error %d: %s", envelope.Code, envelope.Message)
	}
	if out == nil {
		return nil
	}
	body := map[string]json.RawMessage{
		"result": envelope.Result,
	}
	rawOut, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return json.Unmarshal(rawOut, out)
}

func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("xiaomi request failed: %s", resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func sortedKeys(items map[string]DeviceRecord) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stringify(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return fmt.Sprintf("%.0f", typed)
	case int:
		return fmt.Sprintf("%d", typed)
	default:
		return ""
	}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return stringify(value)
	}
}

func boolValue(value any) bool {
	raw, ok := value.(bool)
	return ok && raw
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}
