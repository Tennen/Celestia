package cloud

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/xiaomi/oauth"
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

func (c *Client) AccessToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.accessToken
}

func (c *Client) CurrentLegacySession() (serviceToken, ssecurity, userID, cuserID string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.serviceToken == "" || c.ssecurity == "" || c.userID == "" {
		return "", "", "", "", false
	}
	return c.serviceToken, c.ssecurity, c.userID, c.cuserID, true
}

func (c *Client) CurrentOAuthTokenSet() (accessToken, refreshToken string, expiresAt time.Time, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken == "" {
		return "", "", time.Time{}, false
	}
	return c.accessToken, c.refreshToken, c.expiresAt, true
}
