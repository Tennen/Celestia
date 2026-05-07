package touchpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type weComMenuResponse struct {
	ErrCode int                `json:"errcode"`
	ErrMsg  string             `json:"errmsg"`
	Button  []weComMenuButton  `json:"button"`
	Menu    *weComMenuEnvelope `json:"menu,omitempty"`
}

type weComMenuEnvelope struct {
	Button []weComMenuButton `json:"button"`
}

type weComMenuButton struct {
	Type       string            `json:"type,omitempty"`
	Name       string            `json:"name"`
	Key        string            `json:"key,omitempty"`
	URL        string            `json:"url,omitempty"`
	SubButton  []weComMenuButton `json:"sub_button,omitempty"`
	SubButtons []weComMenuButton `json:"sub_buttons,omitempty"`
}

func (s *Service) fetchWeComMenu(ctx context.Context, config models.AgentWeComConfig) (models.AgentWeComMenuSnapshot, error) {
	var out weComMenuResponse
	if strings.TrimSpace(config.BridgeURL) != "" {
		token, err := s.wecomBridgeToken(ctx, config)
		if err != nil {
			return models.AgentWeComMenuSnapshot{}, err
		}
		endpoint := strings.TrimRight(config.BridgeURL, "/") + "/proxy/menu/get"
		if err := wecomBridgePost(ctx, endpoint, config.BridgeToken, map[string]any{
			"access_token": token,
			"agentid":      strings.TrimSpace(config.AgentID),
		}, &out); err != nil {
			return models.AgentWeComMenuSnapshot{}, err
		}
	} else {
		token, err := s.wecomAccessToken(ctx, config)
		if err != nil {
			return models.AgentWeComMenuSnapshot{}, err
		}
		endpoint := strings.TrimRight(firstNonEmpty(config.BaseURL, "https://qyapi.weixin.qq.com"), "/") + "/cgi-bin/menu/get"
		params := url.Values{"access_token": {token}, "agentid": {strings.TrimSpace(config.AgentID)}}
		if err := wecomGetJSON(ctx, endpoint+"?"+params.Encode(), &out); err != nil {
			return models.AgentWeComMenuSnapshot{}, err
		}
	}
	if out.ErrCode != 0 {
		if isWeComMenuMissing(out.ErrCode, out.ErrMsg) {
			return emptyWeComMenuSnapshot(), nil
		}
		return models.AgentWeComMenuSnapshot{}, fmt.Errorf("WeCom menu get failed: %s", firstNonEmpty(out.ErrMsg, fmt.Sprint(out.ErrCode)))
	}
	return menuSnapshotFromWeComResponse(out), nil
}

func (s *Service) createWeComMenu(ctx context.Context, config models.AgentWeComConfig, payload map[string]any) error {
	if strings.TrimSpace(config.BridgeURL) != "" {
		token, err := s.wecomBridgeToken(ctx, config)
		if err != nil {
			return err
		}
		var out struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		endpoint := strings.TrimRight(config.BridgeURL, "/") + "/proxy/menu/create"
		if err := wecomBridgePost(ctx, endpoint, config.BridgeToken, map[string]any{
			"access_token": token,
			"agentid":      strings.TrimSpace(config.AgentID),
			"menu":         payload,
		}, &out); err != nil {
			return err
		}
		if out.ErrCode != 0 {
			return fmt.Errorf("WeCom bridge menu create failed: %s", firstNonEmpty(out.ErrMsg, fmt.Sprint(out.ErrCode)))
		}
		return nil
	}
	token, err := s.wecomAccessToken(ctx, config)
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(firstNonEmpty(config.BaseURL, "https://qyapi.weixin.qq.com"), "/") + "/cgi-bin/menu/create"
	params := url.Values{"access_token": {token}, "agentid": {strings.TrimSpace(config.AgentID)}}
	return wecomPost(ctx, endpoint+"?"+params.Encode(), payload, nil)
}

func (s *Service) deleteWeComMenu(ctx context.Context, config models.AgentWeComConfig) error {
	if strings.TrimSpace(config.BridgeURL) != "" {
		token, err := s.wecomBridgeToken(ctx, config)
		if err != nil {
			return err
		}
		var out struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		endpoint := strings.TrimRight(config.BridgeURL, "/") + "/proxy/menu/delete"
		if err := wecomBridgePost(ctx, endpoint, config.BridgeToken, map[string]any{
			"access_token": token,
			"agentid":      strings.TrimSpace(config.AgentID),
		}, &out); err != nil {
			return err
		}
		if out.ErrCode != 0 && !isWeComMenuMissing(out.ErrCode, out.ErrMsg) {
			return fmt.Errorf("WeCom bridge menu delete failed: %s", firstNonEmpty(out.ErrMsg, fmt.Sprint(out.ErrCode)))
		}
		return nil
	}
	token, err := s.wecomAccessToken(ctx, config)
	if err != nil {
		return err
	}
	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	endpoint := strings.TrimRight(firstNonEmpty(config.BaseURL, "https://qyapi.weixin.qq.com"), "/") + "/cgi-bin/menu/delete"
	params := url.Values{"access_token": {token}, "agentid": {strings.TrimSpace(config.AgentID)}}
	if err := wecomGetJSON(ctx, endpoint+"?"+params.Encode(), &out); err != nil {
		return err
	}
	if out.ErrCode != 0 && !isWeComMenuMissing(out.ErrCode, out.ErrMsg) {
		return fmt.Errorf("WeCom menu delete failed: %s", firstNonEmpty(out.ErrMsg, fmt.Sprint(out.ErrCode)))
	}
	return nil
}

func wecomGetJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		detail := strings.TrimSpace(string(data))
		if detail == "" {
			detail = resp.Status
		}
		return fmt.Errorf("WeCom http %s: %s", resp.Status, detail)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func menuSnapshotFromWeComResponse(out weComMenuResponse) models.AgentWeComMenuSnapshot {
	now := time.Now().UTC()
	buttons := out.Button
	if len(buttons) == 0 && out.Menu != nil {
		buttons = out.Menu.Button
	}
	config := models.AgentWeComMenuConfig{
		Version:   1,
		Buttons:   make([]models.AgentWeComButton, 0, len(buttons)),
		UpdatedAt: now,
	}
	validationErrors := []string{}
	for idx, button := range buttons {
		next, errors := agentButtonFromWeCom(button, fmt.Sprintf("menu-%d", idx+1))
		config.Buttons = append(config.Buttons, next)
		validationErrors = append(validationErrors, errors...)
	}
	return models.AgentWeComMenuSnapshot{
		Config:           config,
		PublishPayload:   buildWeComMenuPayload(config),
		ValidationErrors: validationErrors,
	}
}

func agentButtonFromWeCom(button weComMenuButton, fallbackID string) (models.AgentWeComButton, []string) {
	children := button.SubButton
	if len(children) == 0 {
		children = button.SubButtons
	}
	next := models.AgentWeComButton{
		ID:      menuButtonID(fallbackID, button.Name, button.Key),
		Name:    strings.TrimSpace(button.Name),
		Key:     strings.TrimSpace(button.Key),
		Enabled: true,
	}
	errors := []string{}
	if len(children) > 0 {
		next.Key = ""
		next.SubButtons = make([]models.AgentWeComButton, 0, len(children))
		for idx, child := range children {
			childButton, childErrors := agentButtonFromWeCom(child, fmt.Sprintf("%s-%d", fallbackID, idx+1))
			next.SubButtons = append(next.SubButtons, childButton)
			errors = append(errors, childErrors...)
		}
		return next, errors
	}
	if menuType := strings.TrimSpace(button.Type); menuType != "" && menuType != "click" {
		errors = append(errors, fmt.Sprintf("unsupported WeCom menu type %q for %q", menuType, firstNonEmpty(button.Name, fallbackID)))
	}
	if next.Key == "" {
		next.Key = strings.TrimSpace(button.URL)
	}
	return next, errors
}

func normalizeWeComMenuConfig(config models.AgentWeComMenuConfig) models.AgentWeComMenuConfig {
	now := time.Now().UTC()
	config.Version = 1
	config.UpdatedAt = now
	if config.Buttons == nil {
		config.Buttons = []models.AgentWeComButton{}
	}
	return config
}

func emptyWeComMenuSnapshot() models.AgentWeComMenuSnapshot {
	now := time.Now().UTC()
	return models.AgentWeComMenuSnapshot{
		Config: models.AgentWeComMenuConfig{
			Version:   1,
			Buttons:   []models.AgentWeComButton{},
			UpdatedAt: now,
		},
		PublishPayload: map[string]any{"button": []any{}},
	}
}

func isWeComMenuMissing(code int, message string) bool {
	if code == 46003 || code == 46004 {
		return true
	}
	normalized := strings.ToLower(message)
	return strings.Contains(normalized, "not exist") || strings.Contains(message, "不存在")
}

func menuButtonID(fallbackID string, values ...string) string {
	for _, value := range values {
		slug := strings.Trim(strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z':
				return r
			case r >= 'A' && r <= 'Z':
				return r + ('a' - 'A')
			case r >= '0' && r <= '9':
				return r
			case r == '.' || r == '_' || r == '-':
				return r
			default:
				return '-'
			}
		}, strings.TrimSpace(value)), "-")
		if slug != "" {
			return slug
		}
	}
	return fallbackID
}
