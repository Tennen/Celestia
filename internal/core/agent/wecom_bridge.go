package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type weComBridgePayload struct {
	MessageID  string `json:"messageId"`
	SessionID  string `json:"sessionId"`
	FromUser   string `json:"fromUser"`
	ToUser     string `json:"toUser"`
	Text       string `json:"text"`
	MsgType    string `json:"msgType"`
	Event      string `json:"event"`
	EventKey   string `json:"eventKey"`
	AgentID    string `json:"agentId"`
	MediaID    string `json:"mediaId"`
	PicURL     string `json:"picUrl"`
	ReceivedAt string `json:"receivedAt"`
}

func (s *Service) runWeComBridge() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-s.stop
		cancel()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			sleepOrStop(ctx, 10*time.Second)
			continue
		}
		config := snapshot.Settings.WeCom
		if !config.Enabled || !config.BridgeStreamEnabled || strings.TrimSpace(config.BridgeURL) == "" {
			sleepOrStop(ctx, 15*time.Second)
			continue
		}
		if err := s.consumeWeComBridgeStream(ctx, config); err != nil && ctx.Err() == nil {
			log.Printf("[agent-wecom-bridge] stream disconnected: %v", err)
			sleepOrStop(ctx, 3*time.Second)
		}
	}
}

func (s *Service) consumeWeComBridgeStream(ctx context.Context, config models.AgentWeComConfig) error {
	endpoint := strings.TrimRight(config.BridgeURL, "/") + "/stream"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(config.BridgeToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(config.BridgeToken))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return io.ErrUnexpectedEOF
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	var event bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if event.Len() > 0 {
				s.dispatchWeComBridgeEvent(ctx, event.String())
				event.Reset()
			}
			continue
		}
		event.WriteString(line)
		event.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return io.ErrUnexpectedEOF
}

func (s *Service) dispatchWeComBridgeEvent(ctx context.Context, raw string) {
	data := parseSSEData(raw)
	if data == "" {
		return
	}
	var payload weComBridgePayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		log.Printf("[agent-wecom-bridge] invalid event: %v", err)
		return
	}
	if strings.TrimSpace(payload.MessageID) == "" || strings.TrimSpace(payload.FromUser) == "" {
		return
	}
	if err := s.handleWeComBridgePayload(ctx, payload); err != nil {
		log.Printf("[agent-wecom-bridge] handle message=%s failed: %v", payload.MessageID, err)
	}
}

func (s *Service) handleWeComBridgePayload(ctx context.Context, payload weComBridgePayload) error {
	msgType := strings.ToLower(firstNonEmpty(payload.MsgType, "text"))
	if msgType == "event" {
		record, err := s.recordWeComEvent(ctx, strings.ToLower(payload.Event), payload.EventKey, payload.FromUser, payload.ToUser, payload.AgentID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(record.DispatchText) == "" {
			return nil
		}
		conversation, err := s.Converse(ctx, models.AgentConversationRequest{SessionID: payload.FromUser, Input: record.DispatchText, Actor: "wecom-bridge"})
		return s.replyWeComBridge(ctx, payload.FromUser, conversation.Response, err)
	}
	input := strings.TrimSpace(payload.Text)
	if msgType == "voice" {
		snapshot, err := s.Snapshot(ctx)
		if err != nil {
			return err
		}
		input = s.resolveWeComVoiceInput(ctx, snapshot.Settings.WeCom, payload.MediaID, payload.MessageID, payload.Text)
	}
	if msgType == "image" {
		input = firstNonEmpty(payload.Text, "收到图片: "+payload.PicURL)
	}
	if strings.TrimSpace(input) == "" {
		input = "收到空消息"
	}
	if _, err := s.recordWeComMessage(ctx, msgType, payload.FromUser, payload.ToUser, payload.AgentID, payload.MessageID, input); err != nil {
		return err
	}
	conversation, err := s.Converse(ctx, models.AgentConversationRequest{SessionID: payload.FromUser, Input: input, Actor: "wecom-bridge"})
	return s.replyWeComBridge(ctx, payload.FromUser, conversation.Response, err)
}

func (s *Service) replyWeComBridge(ctx context.Context, toUser string, text string, priorErr error) error {
	response := strings.TrimSpace(text)
	if priorErr != nil {
		response = firstNonEmpty(response, priorErr.Error())
	}
	if response == "" {
		return priorErr
	}
	if err := s.SendWeComMessage(ctx, WeComSendRequest{ToUser: toUser, Text: response}); err != nil {
		return err
	}
	return priorErr
}

func parseSSEData(raw string) string {
	lines := strings.Split(raw, "\n")
	data := []string{}
	for _, line := range lines {
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	return strings.Join(data, "\n")
}

func sleepOrStop(ctx context.Context, duration time.Duration) {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
