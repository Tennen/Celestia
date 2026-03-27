package app

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	petkitEndpointFamilyList   = "group/family/list"
	petkitEndpointDeviceDetail = "device_detail"
)

type requestAuthMode int

const (
	requestAuthPublic requestAuthMode = iota
	requestAuthSession
)

type sessionInfo struct {
	ID        string
	UserID    string
	ExpiresIn int
	Region    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type petkitRequestError struct {
	Status  int
	Method  string
	URL     string
	Code    int
	Message string
	Body    string
	Form    string
}

func (e *petkitRequestError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{
		fmt.Sprintf("petkit request failed"),
		fmt.Sprintf("method=%s", e.Method),
		fmt.Sprintf("status=%d", e.Status),
		fmt.Sprintf("url=%s", e.URL),
	}
	if e.Code != 0 {
		parts = append(parts, fmt.Sprintf("code=%d", e.Code))
	}
	if e.Message != "" {
		parts = append(parts, fmt.Sprintf("message=%q", e.Message))
	}
	if e.Form != "" {
		parts = append(parts, fmt.Sprintf("form=%s", e.Form))
	}
	if e.Body != "" {
		parts = append(parts, fmt.Sprintf("response=%s", e.Body))
	}
	return strings.Join(parts, " ")
}

type petkitDeviceInfo struct {
	DeviceID   int
	DeviceType string
	GroupID    int
	TypeCode   int
	UniqueID   string
	DeviceName string
	CreatedAt  int
	MAC        string
}

type Client struct {
	mu          sync.Mutex
	cfg         AccountConfig
	compat      CompatConfig
	httpClient  *http.Client
	baseURL     string
	session     *sessionInfo
	bleCounters map[int]int
	lastSyncErr error
}

func NewClient(cfg AccountConfig, compat CompatConfig) *Client {
	client := &Client{
		cfg:    cfg,
		compat: compat,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		bleCounters: map[int]int{},
	}
	if baseURL := sanitizeSessionBaseURL(strings.TrimSpace(cfg.SessionBaseURL), cfg.Region, compat); baseURL != "" {
		client.baseURL = strings.TrimRight(baseURL, "/") + "/"
	}
	if session, ok := storedSessionFromConfig(cfg); ok {
		client.session = session
	}
	return client
}
