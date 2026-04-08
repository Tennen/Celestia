package cloud

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	loginPath          = "/v3/users/login/v5"
	refreshSessionPath = "/v3/apigateway/login"
	pagelistPath       = "/v3/userdevices/v1/resources/pagelist"
	ptzControlSuffix   = "/ptzControl"
)

const defaultPagelistFilter = "CLOUD, TIME_PLAN, CONNECTION, SWITCH, STATUS, WIFI, NODISTURB, KMS, P2P, TIME_PLAN, CHANNEL, VTM, DETECTOR, FEATURE, CUSTOM_TAG, UPGRADE, VIDEO_QUALITY, QOS, PRODUCTS_INFO, SIM_CARD, MULTI_UPGRADE_EXT, FEATURE_INFO"

type Session struct {
	mu          sync.Mutex
	cfg         Config
	httpClient  *http.Client
	featureCode string
}

func NewSession(cfg Config, httpClient *http.Client) *Session {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 25 * time.Second}
	}
	return &Session{
		cfg:         cfg.Sanitized(),
		httpClient:  httpClient,
		featureCode: generateFeatureCode(),
	}
}

func (s *Session) CurrentConfig() (Config, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.cfg.Sanitized()
	return cfg, cfg.HasSession()
}

func (s *Session) RefreshDevices(ctx context.Context) (map[string]DeviceInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.HasAuth() {
		return nil, errors.New("ezviz cloud auth is not configured")
	}
	if err := s.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	payload, err := s.getPagelist(ctx, 0)
	if err != nil {
		return nil, err
	}
	return buildDeviceIndex(payload), nil
}

func (s *Session) PTZMove(ctx context.Context, serial, command string, speed int, duration time.Duration) error {
	if err := s.ptzControl(ctx, serial, command, "START", speed); err != nil {
		return err
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	if err := s.ptzControl(context.Background(), serial, command, "STOP", speed); err != nil {
		return err
	}
	return ctx.Err()
}

func (s *Session) PTZStop(ctx context.Context, serial, command string, speed int) error {
	return s.ptzControl(ctx, serial, command, "STOP", speed)
}

func (s *Session) ptzControl(ctx context.Context, serial, command, action string, speed int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.cfg.HasAuth() {
		return errors.New("ezviz cloud auth is not configured")
	}
	if err := s.ensureAuthenticated(ctx); err != nil {
		return err
	}

	serial = strings.TrimSpace(serial)
	command = strings.ToUpper(strings.TrimSpace(command))
	action = strings.ToUpper(strings.TrimSpace(action))
	if serial == "" {
		return errors.New("device serial is required")
	}
	if command == "" {
		return errors.New("ptz command is required")
	}
	if action == "" {
		return errors.New("ptz action is required")
	}
	if speed < 1 {
		speed = 1
	}
	if speed > 10 {
		speed = 10
	}

	body := url.Values{
		"command":   []string{command},
		"action":    []string{action},
		"channelNo": []string{"1"},
		"speed":     []string{fmt.Sprintf("%d", speed)},
		"uuid":      []string{uuid.NewString()},
		"serial":    []string{serial},
	}
	path := "/v3/devices/" + serial + ptzControlSuffix
	resp, err := s.doFormRequest(ctx, http.MethodPut, path, nil, body, false)
	if err != nil {
		return err
	}
	if !responseOK(resp) {
		return responseError("ezviz ptz control failed", resp)
	}
	return nil
}

func generateFeatureCode() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = uuid.NewString()
	}
	sum := md5.Sum([]byte(host))
	return hex.EncodeToString(sum[:])
}
