package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func handleProxyGetToken(w http.ResponseWriter, r *http.Request, cfg bridgeConfig) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !checkBridgeAuth(w, r, cfg) {
		return
	}

	body, err := readBody(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing body"))
		return
	}
	var payload struct {
		CorpID     string `json:"corpid"`
		CorpSecret string `json:"corpsecret"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json"))
		return
	}
	if payload.CorpID == "" || payload.CorpSecret == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing corpid/corpsecret"))
		return
	}

	qs := url.Values{}
	qs.Set("corpid", payload.CorpID)
	qs.Set("corpsecret", payload.CorpSecret)
	endpoint := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?%s", qs.Encode())

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("gettoken failed"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(fmt.Sprintf("token http %d", resp.StatusCode)))
		return
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("token read failed"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func handleProxySend(w http.ResponseWriter, r *http.Request, cfg bridgeConfig) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !checkBridgeAuth(w, r, cfg) {
		return
	}

	body, err := readBody(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing body"))
		return
	}
	var payload struct {
		AccessToken string          `json:"access_token"`
		Message     json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json"))
		return
	}
	if payload.AccessToken == "" || len(payload.Message) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing access_token/message"))
		return
	}

	endpoint := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", payload.AccessToken)
	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(payload.Message))
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("send failed"))
		return
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("send read failed"))
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(fmt.Sprintf("send http %d", resp.StatusCode)))
		return
	}

	var result struct {
		ErrCode int `json:"errcode"`
	}
	_ = json.Unmarshal(data, &result)
	if result.ErrCode != 0 {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("send failed"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
}

func handleProxyUpload(w http.ResponseWriter, r *http.Request, cfg bridgeConfig) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !checkBridgeAuth(w, r, cfg) {
		return
	}

	body, err := readBody(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing body"))
		return
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		Type        string `json:"type"`
		Media       struct {
			Base64      string `json:"base64"`
			Filename    string `json:"filename"`
			ContentType string `json:"content_type"`
		} `json:"media"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json"))
		return
	}
	if payload.AccessToken == "" || payload.Media.Base64 == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing access_token/media"))
		return
	}

	typeName := payload.Type
	if typeName == "" {
		typeName = "image"
	}
	filename := payload.Media.Filename
	if filename == "" {
		if typeName == "image" {
			filename = "upload.jpg"
		} else {
			filename = "upload.dat"
		}
	}
	data, err := base64.StdEncoding.DecodeString(payload.Media.Base64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid base64"))
		return
	}

	endpoint := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/upload?access_token=%s&type=%s", payload.AccessToken, url.QueryEscape(typeName))

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("media", filename)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upload failed"))
		return
	}
	_, _ = part.Write(data)
	_ = writer.Close()

	client := http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodPost, endpoint, &buf)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upload failed"))
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upload failed"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(fmt.Sprintf("upload http %d", resp.StatusCode)))
		return
	}
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upload read failed"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(respData)
}

func handleProxyMediaGet(w http.ResponseWriter, r *http.Request, cfg bridgeConfig) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !checkBridgeAuth(w, r, cfg) {
		return
	}

	body, err := readBody(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing body"))
		return
	}
	var payload struct {
		AccessToken string `json:"access_token"`
		MediaID     string `json:"media_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json"))
		return
	}
	if payload.AccessToken == "" || payload.MediaID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing access_token/media_id"))
		return
	}

	query := url.Values{}
	query.Set("access_token", payload.AccessToken)
	query.Set("media_id", payload.MediaID)
	endpoint := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/get?%s", query.Encode())

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("media get failed"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(fmt.Sprintf("media get http %d", resp.StatusCode)))
		return
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("media get read failed"))
		return
	}

	if strings.Contains(strings.ToLower(contentType), "application/json") {
		var apiErr struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		_ = json.Unmarshal(respData, &apiErr)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(fmt.Sprintf("media get error %d %s", apiErr.ErrCode, apiErr.ErrMsg)))
		return
	}

	filename := parseFilenameFromDisposition(resp.Header.Get("Content-Disposition"))
	if filename == "" {
		filename = fmt.Sprintf("%s.dat", payload.MediaID)
	}
	result := map[string]any{
		"base64":       base64.StdEncoding.EncodeToString(respData),
		"filename":     filename,
		"content_type": firstNonEmpty(contentType, "application/octet-stream"),
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func checkBridgeAuth(w http.ResponseWriter, r *http.Request, cfg bridgeConfig) bool {
	if cfg.BridgeToken == "" {
		return true
	}
	if r.Header.Get("Authorization") != fmt.Sprintf("Bearer %s", cfg.BridgeToken) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
		return false
	}
	return true
}

func readBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, errors.New("empty body")
	}
	return body, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func parseFilenameFromDisposition(disposition string) string {
	trimmed := strings.TrimSpace(disposition)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if idx := strings.Index(lower, "filename*=utf-8''"); idx >= 0 {
		start := idx + len("filename*=utf-8''")
		rest := strings.TrimSpace(trimmed[start:])
		if semi := strings.Index(rest, ";"); semi >= 0 {
			rest = rest[:semi]
		}
		rest = strings.Trim(rest, "\"'")
		if decoded, err := url.QueryUnescape(rest); err == nil {
			return decoded
		}
		return rest
	}

	if idx := strings.Index(lower, "filename="); idx >= 0 {
		start := idx + len("filename=")
		rest := strings.TrimSpace(trimmed[start:])
		if semi := strings.Index(rest, ";"); semi >= 0 {
			rest = rest[:semi]
		}
		return strings.Trim(strings.TrimSpace(rest), "\"'")
	}
	return ""
}
