package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

type WeComSendRequest struct {
	ToUser string `json:"to_user"`
	Text   string `json:"text"`
}

type WeComImageRequest struct {
	ToUser      string `json:"to_user"`
	Base64      string `json:"base64"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}

func (s *Service) SaveWeComMenu(ctx context.Context, config models.AgentWeComMenuConfig) (models.AgentSnapshot, error) {
	return s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		config.Version = 1
		config.UpdatedAt = now
		snapshot.WeComMenu.Config = config
		snapshot.WeComMenu.PublishPayload = buildWeComMenuPayload(config)
		snapshot.WeComMenu.ValidationErrors = validateWeComMenu(config)
		snapshot.UpdatedAt = now
		return nil
	})
}

func (s *Service) PublishWeComMenu(ctx context.Context) (models.AgentWeComMenuSnapshot, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentWeComMenuSnapshot{}, err
	}
	if len(snapshot.WeComMenu.ValidationErrors) > 0 {
		return models.AgentWeComMenuSnapshot{}, errors.New(strings.Join(snapshot.WeComMenu.ValidationErrors, "; "))
	}
	token, err := s.wecomAccessToken(ctx, snapshot.Settings.WeCom)
	if err != nil {
		return models.AgentWeComMenuSnapshot{}, err
	}
	payload := buildWeComMenuPayload(snapshot.WeComMenu.Config)
	endpoint := strings.TrimRight(snapshot.Settings.WeCom.BaseURL, "/") + "/cgi-bin/menu/create"
	params := url.Values{"access_token": {token}, "agentid": {snapshot.Settings.WeCom.AgentID}}
	if err := wecomPost(ctx, endpoint+"?"+params.Encode(), payload, nil); err != nil {
		return models.AgentWeComMenuSnapshot{}, err
	}
	next, err := s.update(ctx, func(item *models.AgentSnapshot) error {
		now := time.Now().UTC()
		item.WeComMenu.PublishPayload = payload
		item.WeComMenu.Config.LastPublishedAt = &now
		item.WeComMenu.Config.UpdatedAt = now
		item.UpdatedAt = now
		return nil
	})
	if err != nil {
		return models.AgentWeComMenuSnapshot{}, err
	}
	return next.WeComMenu, nil
}

func (s *Service) SendWeComMessage(ctx context.Context, req WeComSendRequest) error {
	if err := requireText(req.ToUser, "to_user"); err != nil {
		return err
	}
	if err := requireText(req.Text, "text"); err != nil {
		return err
	}
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return err
	}
	chunks := splitTextByUTF8Bytes(req.Text, maxInt(snapshot.Settings.WeCom.TextMaxBytes, 1800))
	for _, chunk := range chunks {
		payload := map[string]any{
			"touser":  strings.TrimSpace(req.ToUser),
			"msgtype": "text",
			"agentid": parseAgentID(snapshot.Settings.WeCom.AgentID),
			"text":    map[string]any{"content": chunk},
		}
		if err := s.sendWeComPayload(ctx, snapshot.Settings.WeCom, payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) RecordWeComXML(ctx context.Context, raw []byte) (models.AgentWeComEventRecord, error) {
	result, err := s.HandleWeComXML(ctx, raw)
	return result.Record, err
}

func (s *Service) HandleWeComXML(ctx context.Context, raw []byte) (models.AgentWeComInboundResult, error) {
	var payload struct {
		XMLName     xml.Name `xml:"xml"`
		ToUserName  string   `xml:"ToUserName"`
		FromUser    string   `xml:"FromUserName"`
		MsgType     string   `xml:"MsgType"`
		Event       string   `xml:"Event"`
		EventKey    string   `xml:"EventKey"`
		Content     string   `xml:"Content"`
		Recognition string   `xml:"Recognition"`
		MediaID     string   `xml:"MediaId"`
		MsgID       string   `xml:"MsgId"`
		AgentID     string   `xml:"AgentID"`
	}
	if err := xml.Unmarshal(raw, &payload); err != nil {
		return models.AgentWeComInboundResult{}, err
	}
	msgType := strings.ToLower(strings.TrimSpace(payload.MsgType))
	if msgType == "event" {
		record, err := s.recordWeComEvent(ctx, strings.ToLower(payload.Event), payload.EventKey, payload.FromUser, payload.ToUserName, payload.AgentID)
		if err != nil {
			return models.AgentWeComInboundResult{}, err
		}
		response := ""
		if strings.TrimSpace(record.DispatchText) != "" {
			conversation, convErr := s.Converse(ctx, models.AgentConversationRequest{
				SessionID: record.FromUser,
				Input:     record.DispatchText,
				Actor:     "wecom",
			})
			if convErr != nil {
				response = convErr.Error()
			} else {
				response = conversation.Response
			}
		}
		return models.AgentWeComInboundResult{Record: record, ResponseText: response, FromUser: payload.FromUser, ToUser: payload.ToUserName}, nil
	}
	if msgType == "text" || msgType == "voice" {
		input := strings.TrimSpace(payload.Content)
		if msgType == "voice" {
			input = strings.TrimSpace(payload.Recognition)
			if input == "" {
				input = "收到语音但没有 Recognition 文本；请启用企业微信语音识别或通过 STT API 转写 media_id: " + strings.TrimSpace(payload.MediaID)
			}
		}
		record, err := s.recordWeComMessage(ctx, msgType, payload.FromUser, payload.ToUserName, payload.AgentID, payload.MsgID, input)
		if err != nil {
			return models.AgentWeComInboundResult{}, err
		}
		conversation, convErr := s.Converse(ctx, models.AgentConversationRequest{
			SessionID: payload.FromUser,
			Input:     input,
			Actor:     "wecom",
		})
		response := conversation.Response
		if convErr != nil {
			response = convErr.Error()
		}
		return models.AgentWeComInboundResult{Record: record, ResponseText: response, FromUser: payload.FromUser, ToUser: payload.ToUserName}, nil
	}
	record, err := s.recordWeComMessage(ctx, firstNonEmpty(msgType, "unknown"), payload.FromUser, payload.ToUserName, payload.AgentID, payload.MsgID, "")
	if err != nil {
		return models.AgentWeComInboundResult{}, err
	}
	return models.AgentWeComInboundResult{Record: record, FromUser: payload.FromUser, ToUser: payload.ToUserName}, nil
}

func (s *Service) recordWeComEvent(ctx context.Context, eventType, eventKey, fromUser, toUser, agentID string) (models.AgentWeComEventRecord, error) {
	if strings.TrimSpace(eventKey) == "" {
		return models.AgentWeComEventRecord{}, errors.New("event key is required")
	}
	now := time.Now().UTC()
	record := models.AgentWeComEventRecord{
		ID:         uuid.NewString(),
		EventType:  firstNonEmpty(eventType, "click"),
		EventKey:   strings.TrimSpace(eventKey),
		FromUser:   strings.TrimSpace(fromUser),
		ToUser:     strings.TrimSpace(toUser),
		AgentID:    strings.TrimSpace(agentID),
		Status:     "recorded",
		ReceivedAt: now,
	}
	next, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		if button, ok := findWeComButton(snapshot.WeComMenu.Config.Buttons, record.EventKey); ok {
			record.MatchedButtonID = button.ID
			record.MatchedButtonName = button.Name
			record.DispatchText = button.DispatchText
			record.Status = "dispatched"
		}
		snapshot.WeComMenu.RecentEvents = append([]models.AgentWeComEventRecord{record}, snapshot.WeComMenu.RecentEvents...)
		snapshot.WeComMenu.RecentEvents = truncateList(snapshot.WeComMenu.RecentEvents, 50)
		snapshot.UpdatedAt = now
		return nil
	})
	if err != nil {
		return models.AgentWeComEventRecord{}, err
	}
	return next.WeComMenu.RecentEvents[0], nil
}

func (s *Service) recordWeComMessage(ctx context.Context, msgType, fromUser, toUser, agentID, msgID, content string) (models.AgentWeComEventRecord, error) {
	now := time.Now().UTC()
	record := models.AgentWeComEventRecord{
		ID:         uuid.NewString(),
		EventType:  firstNonEmpty(msgType, "message"),
		EventKey:   strings.TrimSpace(msgID),
		FromUser:   strings.TrimSpace(fromUser),
		ToUser:     strings.TrimSpace(toUser),
		AgentID:    strings.TrimSpace(agentID),
		Status:     "dispatched",
		ReceivedAt: now,
	}
	if strings.TrimSpace(content) != "" {
		record.DispatchText = strings.TrimSpace(content)
	}
	next, err := s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		snapshot.WeComMenu.RecentEvents = append([]models.AgentWeComEventRecord{record}, snapshot.WeComMenu.RecentEvents...)
		snapshot.WeComMenu.RecentEvents = truncateList(snapshot.WeComMenu.RecentEvents, 50)
		snapshot.UpdatedAt = now
		return nil
	})
	if err != nil {
		return models.AgentWeComEventRecord{}, err
	}
	return next.WeComMenu.RecentEvents[0], nil
}

func (s *Service) wecomAccessToken(ctx context.Context, config models.AgentWeComConfig) (string, error) {
	if !config.Enabled {
		return "", errors.New("WeCom integration is disabled")
	}
	if strings.TrimSpace(config.CorpID) == "" || strings.TrimSpace(config.CorpSecret) == "" || strings.TrimSpace(config.AgentID) == "" {
		return "", errors.New("WeCom corp_id, corp_secret, and agent_id are required")
	}
	baseURL := strings.TrimRight(firstNonEmpty(config.BaseURL, "https://qyapi.weixin.qq.com"), "/")
	endpoint := baseURL + "/cgi-bin/gettoken?corpid=" + url.QueryEscape(config.CorpID) + "&corpsecret=" + url.QueryEscape(config.CorpSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.ErrCode != 0 || strings.TrimSpace(out.AccessToken) == "" {
		return "", fmt.Errorf("WeCom token request failed: %s", firstNonEmpty(out.ErrMsg, resp.Status))
	}
	return out.AccessToken, nil
}

func wecomPost(ctx context.Context, endpoint string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var response struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	target := any(&response)
	if out != nil {
		target = out
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}
	if response.ErrCode != 0 && out == nil {
		return fmt.Errorf("WeCom request failed: %s", response.ErrMsg)
	}
	return nil
}

func buildWeComMenuPayload(config models.AgentWeComMenuConfig) map[string]any {
	buttons := make([]map[string]any, 0, len(config.Buttons))
	for _, button := range config.Buttons {
		if !button.Enabled {
			continue
		}
		if len(button.SubButtons) > 0 {
			sub := make([]map[string]any, 0, len(button.SubButtons))
			for _, child := range button.SubButtons {
				if child.Enabled && strings.TrimSpace(child.Name) != "" && strings.TrimSpace(child.Key) != "" {
					sub = append(sub, map[string]any{"type": "click", "name": child.Name, "key": child.Key})
				}
			}
			buttons = append(buttons, map[string]any{"name": button.Name, "sub_button": sub})
			continue
		}
		if strings.TrimSpace(button.Name) != "" && strings.TrimSpace(button.Key) != "" {
			buttons = append(buttons, map[string]any{"type": "click", "name": button.Name, "key": button.Key})
		}
	}
	return map[string]any{"button": buttons}
}

func validateWeComMenu(config models.AgentWeComMenuConfig) []string {
	var out []string
	if len(config.Buttons) > 3 {
		out = append(out, "WeCom supports at most 3 top-level buttons")
	}
	for _, button := range config.Buttons {
		if strings.TrimSpace(button.Name) == "" {
			out = append(out, "button name is required")
		}
		if len(button.SubButtons) == 0 && strings.TrimSpace(button.Key) == "" {
			out = append(out, "leaf button key is required")
		}
		if len(button.SubButtons) > 5 {
			out = append(out, "WeCom supports at most 5 sub-buttons per group")
		}
	}
	return out
}

func findWeComButton(buttons []models.AgentWeComButton, key string) (models.AgentWeComButton, bool) {
	for _, button := range buttons {
		if button.Enabled && strings.TrimSpace(button.Key) == strings.TrimSpace(key) {
			return button, true
		}
		if child, ok := findWeComButton(button.SubButtons, key); ok {
			return child, true
		}
	}
	return models.AgentWeComButton{}, false
}
