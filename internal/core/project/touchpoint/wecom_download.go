package touchpoint

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

var unsafeFilenamePattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func (s *Service) resolveWeComVoiceInput(ctx context.Context, config models.AgentWeComConfig, mediaID string, msgID string, recognition string) string {
	fallback := strings.TrimSpace(recognition)
	if strings.TrimSpace(mediaID) == "" {
		return firstNonEmpty(fallback, "收到语音但缺少 media_id，无法下载转写。")
	}
	audioPath, err := s.downloadWeComMedia(ctx, config, mediaID, msgID)
	if err != nil {
		return firstNonEmpty(fallback, "语音下载失败: "+err.Error())
	}
	s.mu.Lock()
	provider := s.voice
	s.mu.Unlock()
	if provider == nil {
		return firstNonEmpty(fallback, "语音已保存但没有配置 STT provider; file="+audioPath)
	}
	result, err := provider.Transcribe(ctx, models.AgentSpeechRequest{AudioPath: audioPath})
	if err != nil {
		return firstNonEmpty(fallback, "语音已保存但 STT 转写失败: "+err.Error()+"; file="+audioPath)
	}
	return firstNonEmpty(result.Text, fallback, "语音转写为空; file="+audioPath)
}

func (s *Service) downloadWeComMedia(ctx context.Context, config models.AgentWeComConfig, mediaID string, msgID string) (string, error) {
	token := ""
	var err error
	if strings.TrimSpace(config.BridgeURL) != "" {
		token, err = s.wecomBridgeToken(ctx, config)
		if err == nil {
			path, bridgeErr := fetchWeComMediaViaBridge(ctx, config, token, mediaID, msgID)
			if bridgeErr == nil {
				return path, nil
			}
			err = bridgeErr
		}
	}
	if token == "" {
		token, err = s.wecomAccessToken(ctx, config)
		if err != nil {
			return "", err
		}
	}
	path, directErr := fetchWeComMediaDirect(ctx, config, token, mediaID, msgID)
	if directErr != nil {
		if err != nil {
			return "", errors.New(err.Error() + "; direct fallback failed: " + directErr.Error())
		}
		return "", directErr
	}
	return path, nil
}

func fetchWeComMediaViaBridge(ctx context.Context, config models.AgentWeComConfig, token string, mediaID string, msgID string) (string, error) {
	var out struct {
		Base64      string `json:"base64"`
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	endpoint := strings.TrimRight(config.BridgeURL, "/") + "/proxy/media/get"
	if err := wecomBridgePost(ctx, endpoint, config.BridgeToken, map[string]any{
		"access_token": token,
		"media_id":     strings.TrimSpace(mediaID),
	}, &out); err != nil {
		return "", err
	}
	if out.ErrCode != 0 || strings.TrimSpace(out.Base64) == "" {
		return "", errors.New("WeCom bridge media get failed: " + firstNonEmpty(out.ErrMsg, "missing base64"))
	}
	data, err := base64.StdEncoding.DecodeString(stripBase64Prefix(out.Base64))
	if err != nil {
		return "", err
	}
	return writeWeComMediaFile(config, data, firstNonEmpty(out.Filename, mediaID), out.ContentType, msgID, mediaID)
}

func fetchWeComMediaDirect(ctx context.Context, config models.AgentWeComConfig, token string, mediaID string, msgID string) (string, error) {
	endpoint := strings.TrimRight(config.BaseURL, "/") + "/cgi-bin/media/get"
	params := url.Values{"access_token": {token}, "media_id": {strings.TrimSpace(mediaID)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/json") {
		var out struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return "", errors.New("WeCom media get failed: " + firstNonEmpty(out.ErrMsg, resp.Status))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New("WeCom media get HTTP " + resp.Status)
	}
	var body bytes.Buffer
	if _, err := io.Copy(&body, io.LimitReader(resp.Body, 32<<20)); err != nil {
		return "", err
	}
	return writeWeComMediaFile(config, body.Bytes(), parseContentDispositionFilename(resp.Header.Get("Content-Disposition")), resp.Header.Get("Content-Type"), msgID, mediaID)
}

func writeWeComMediaFile(config models.AgentWeComConfig, data []byte, filename string, contentType string, msgID string, mediaID string) (string, error) {
	if len(data) == 0 {
		return "", errors.New("WeCom media payload is empty")
	}
	day := time.Now().UTC().Format("2006-01-02")
	dir := filepath.Join(firstNonEmpty(config.AudioDir, "data/touchpoints/wecom-audio"), day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ext := inferWeComMediaExt(filename, contentType, "amr")
	name := safeFilenamePart(firstNonEmpty(msgID, time.Now().UTC().Format("150405"))) + "-" + safeFilenamePart(mediaID)
	path := filepath.Join(dir, name+"."+ext)
	return path, os.WriteFile(path, data, 0o644)
}

func parseContentDispositionFilename(value string) string {
	if value == "" {
		return ""
	}
	if match := regexp.MustCompile(`filename\*=UTF-8''([^;]+)`).FindStringSubmatch(value); len(match) == 2 {
		decoded, err := url.QueryUnescape(match[1])
		if err == nil {
			return decoded
		}
		return match[1]
	}
	if match := regexp.MustCompile(`filename="?([^";]+)"?`).FindStringSubmatch(value); len(match) == 2 {
		return match[1]
	}
	return ""
}

func inferWeComMediaExt(filename string, contentType string, fallback string) string {
	if ext := strings.TrimPrefix(filepath.Ext(filename), "."); ext != "" {
		return safeFilenamePart(ext)
	}
	normalized := strings.ToLower(contentType)
	switch {
	case strings.Contains(normalized, "audio/mpeg"):
		return "mp3"
	case strings.Contains(normalized, "audio/mp4"):
		return "m4a"
	case strings.Contains(normalized, "audio/wav"):
		return "wav"
	case strings.Contains(normalized, "audio/ogg"):
		return "ogg"
	case strings.Contains(normalized, "audio/webm"):
		return "webm"
	default:
		return fallback
	}
}

func safeFilenamePart(value string) string {
	clean := unsafeFilenamePattern.ReplaceAllString(strings.TrimSpace(value), "_")
	clean = strings.Trim(clean, "._-")
	if clean == "" {
		return "media"
	}
	if len(clean) > 64 {
		return clean[:64]
	}
	return clean
}
