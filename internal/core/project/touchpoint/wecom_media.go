package touchpoint

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) SendWeComImage(ctx context.Context, req WeComImageRequest) error {
	if err := requireText(req.ToUser, "to_user"); err != nil {
		return err
	}
	if err := requireText(req.Base64, "base64"); err != nil {
		return err
	}
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return err
	}
	config := snapshot.Settings.WeCom
	recipient, err := resolveWeComRecipient(snapshot.Push.Users, req.ToUser)
	if err != nil {
		return err
	}
	raw, err := decodeWeComImageBase64(req.Base64)
	if err != nil {
		return err
	}
	images, err := prepareWeComImages(raw, firstNonEmpty(req.Filename, "celestia.png"), req.ContentType)
	if err != nil {
		return err
	}
	for _, image := range images {
		mediaID, err := s.uploadWeComImage(ctx, config, WeComImageRequest{
			ToUser:      req.ToUser,
			Base64:      base64.StdEncoding.EncodeToString(image.Bytes),
			Filename:    image.Filename,
			ContentType: image.ContentType,
		})
		if err != nil {
			return err
		}
		payload, groupChat := weComRecipientPayload(recipient, map[string]any{
			"msgtype": "image",
			"image":   map[string]any{"media_id": mediaID},
		}, config.AgentID)
		if err := s.sendWeComPayload(ctx, config, payload, groupChat); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SendProjectImages(ctx context.Context, toUser string, images []models.ProjectOutputImage) error {
	if len(images) == 0 {
		return errors.New("project output image is required")
	}
	for _, image := range images {
		path := strings.TrimSpace(image.Path)
		if path == "" {
			return errors.New("project output image path is required")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := s.SendWeComImage(ctx, WeComImageRequest{
			ToUser:      toUser,
			Base64:      base64.StdEncoding.EncodeToString(raw),
			Filename:    firstNonEmpty(image.Filename, filepath.Base(path)),
			ContentType: firstNonEmpty(image.ContentType, "image/png"),
		}); err != nil {
			return err
		}
	}
	return nil
}

func isImageOnlyProjectResult(result models.ProjectInputResult) bool {
	if len(result.Images) > 0 {
		return true
	}
	if result.Slash == nil || result.Slash.Metadata == nil {
		return false
	}
	return strings.TrimSpace(fmt.Sprint(result.Slash.Metadata["reply_kind"])) == "image"
}

func (s *Service) sendWeComPayload(ctx context.Context, config models.AgentWeComConfig, message map[string]any, groupChat bool) error {
	if strings.TrimSpace(config.BridgeURL) != "" {
		token, err := s.wecomBridgeToken(ctx, config)
		if err != nil {
			return err
		}
		proxyPath := "/proxy/send"
		if groupChat {
			proxyPath = "/proxy/appchat/send"
		}
		endpoint := strings.TrimRight(config.BridgeURL, "/") + proxyPath
		return wecomBridgePost(ctx, endpoint, config.BridgeToken, map[string]any{
			"access_token": token,
			"message":      message,
		}, nil)
	}
	token, err := s.wecomAccessToken(ctx, config)
	if err != nil {
		return err
	}
	apiPath := "/cgi-bin/message/send"
	if groupChat {
		apiPath = "/cgi-bin/appchat/send"
	}
	endpoint := strings.TrimRight(firstNonEmpty(config.BaseURL, "https://qyapi.weixin.qq.com"), "/") + apiPath
	return wecomPost(ctx, endpoint+"?access_token="+url.QueryEscape(token), message, nil)
}

func (s *Service) uploadWeComImage(ctx context.Context, config models.AgentWeComConfig, req WeComImageRequest) (string, error) {
	if err := validateWeComPreparedImage(req); err != nil {
		return "", err
	}
	if strings.TrimSpace(config.BridgeURL) != "" {
		token, err := s.wecomBridgeToken(ctx, config)
		if err != nil {
			return "", err
		}
		var out struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
			MediaID string `json:"media_id"`
		}
		endpoint := strings.TrimRight(config.BridgeURL, "/") + "/proxy/media/upload"
		err = wecomBridgePost(ctx, endpoint, config.BridgeToken, map[string]any{
			"access_token": token,
			"type":         "image",
			"media": map[string]any{
				"base64":       req.Base64,
				"filename":     firstNonEmpty(req.Filename, "celestia.png"),
				"content_type": firstNonEmpty(req.ContentType, "image/png"),
			},
		}, &out)
		if err != nil {
			return "", err
		}
		if out.ErrCode != 0 || strings.TrimSpace(out.MediaID) == "" {
			return "", fmt.Errorf("WeCom bridge upload failed: %s", firstNonEmpty(out.ErrMsg, "missing media_id"))
		}
		return out.MediaID, nil
	}
	return s.uploadWeComImageDirect(ctx, config, req)
}

func (s *Service) uploadWeComImageDirect(ctx context.Context, config models.AgentWeComConfig, req WeComImageRequest) (string, error) {
	token, err := s.wecomAccessToken(ctx, config)
	if err != nil {
		return "", err
	}
	imageBytes, err := base64.StdEncoding.DecodeString(stripBase64Prefix(req.Base64))
	if err != nil {
		return "", err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", firstNonEmpty(req.Filename, "celestia.png"))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, bytes.NewReader(imageBytes)); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	endpoint := strings.TrimRight(config.BaseURL, "/") + "/cgi-bin/media/upload"
	params := url.Values{"access_token": {token}, "type": {"image"}}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+params.Encode(), &body)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		MediaID string `json:"media_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.ErrCode != 0 || strings.TrimSpace(out.MediaID) == "" {
		return "", fmt.Errorf("WeCom image upload failed: %s", firstNonEmpty(out.ErrMsg, resp.Status))
	}
	return out.MediaID, nil
}

func (s *Service) wecomBridgeToken(ctx context.Context, config models.AgentWeComConfig) (string, error) {
	if !config.Enabled {
		return "", errors.New("WeCom integration is disabled")
	}
	if strings.TrimSpace(config.CorpID) == "" || strings.TrimSpace(config.CorpSecret) == "" || strings.TrimSpace(config.AgentID) == "" {
		return "", errors.New("WeCom corp_id, corp_secret, and agent_id are required")
	}
	if strings.TrimSpace(config.BridgeURL) == "" {
		return "", errors.New("WeCom bridge_url is required")
	}
	var out struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
	}
	endpoint := strings.TrimRight(config.BridgeURL, "/") + "/proxy/gettoken"
	err := wecomBridgePost(ctx, endpoint, config.BridgeToken, map[string]any{
		"corpid":     config.CorpID,
		"corpsecret": config.CorpSecret,
	}, &out)
	if err != nil {
		return "", err
	}
	if out.ErrCode != 0 || strings.TrimSpace(out.AccessToken) == "" {
		return "", fmt.Errorf("WeCom bridge token failed: %s", firstNonEmpty(out.ErrMsg, "missing access_token"))
	}
	return out.AccessToken, nil
}

func wecomBridgePost(ctx context.Context, endpoint string, bridgeToken string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(bridgeToken) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(bridgeToken))
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
		return fmt.Errorf("WeCom bridge http %s: %s", resp.Status, detail)
	}
	var envelope struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if out == nil {
		out = &envelope
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return err
	}
	if out == &envelope && envelope.ErrCode != 0 {
		return fmt.Errorf("WeCom bridge request failed: %s", envelope.ErrMsg)
	}
	return nil
}

func splitTextByUTF8Bytes(content string, maxBytes int) []string {
	text := strings.TrimSpace(content)
	if text == "" {
		return []string{}
	}
	if maxBytes <= 0 || len([]byte(text)) <= maxBytes {
		return []string{text}
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	chunks := []string{}
	current := ""
	for _, line := range lines {
		candidate := line
		if current != "" {
			candidate = current + "\n" + line
		}
		if len([]byte(candidate)) <= maxBytes {
			current = candidate
			continue
		}
		if strings.TrimSpace(current) != "" {
			chunks = append(chunks, strings.TrimSpace(current))
		}
		current = ""
		if len([]byte(line)) <= maxBytes {
			current = line
			continue
		}
		hard := splitHardByUTF8Bytes(line, maxBytes)
		chunks = append(chunks, hard[:len(hard)-1]...)
		current = hard[len(hard)-1]
	}
	if strings.TrimSpace(current) != "" {
		chunks = append(chunks, strings.TrimSpace(current))
	}
	return chunks
}

func splitHardByUTF8Bytes(text string, maxBytes int) []string {
	out := []string{}
	current := ""
	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		next := current + string(r)
		if len([]byte(next)) <= maxBytes {
			current = next
			text = text[size:]
			continue
		}
		if current != "" {
			out = append(out, current)
			current = ""
			continue
		}
		out = append(out, string(r))
		text = text[size:]
	}
	if current != "" {
		out = append(out, current)
	}
	return out
}

func stripBase64Prefix(value string) string {
	trimmed := strings.TrimSpace(value)
	if idx := strings.Index(trimmed, ","); strings.HasPrefix(trimmed, "data:") && idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}

func parseAgentID(value string) any {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err == nil {
		return parsed
	}
	return strings.TrimSpace(value)
}
