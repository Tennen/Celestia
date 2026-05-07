package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func handleProxyMenuGet(w http.ResponseWriter, r *http.Request, cfg bridgeConfig) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !checkBridgeAuth(w, r, cfg) {
		return
	}
	payload, ok := decodeMenuProxyRequest(w, r, false)
	if !ok {
		return
	}
	endpoint := weComMenuEndpoint("get", payload.AccessToken, payload.AgentID)
	forwardWeComMenuRead(w, endpoint, "menu get")
}

func handleProxyMenuCreate(w http.ResponseWriter, r *http.Request, cfg bridgeConfig) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !checkBridgeAuth(w, r, cfg) {
		return
	}
	payload, ok := decodeMenuProxyRequest(w, r, true)
	if !ok {
		return
	}
	endpoint := weComMenuEndpoint("create", payload.AccessToken, payload.AgentID)
	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Post(endpoint, "application/json", bytes.NewReader(payload.Menu))
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("menu create failed: " + err.Error()))
		return
	}
	defer resp.Body.Close()
	forwardWeComMenuResponse(w, resp, "menu create")
}

func handleProxyMenuDelete(w http.ResponseWriter, r *http.Request, cfg bridgeConfig) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !checkBridgeAuth(w, r, cfg) {
		return
	}
	payload, ok := decodeMenuProxyRequest(w, r, false)
	if !ok {
		return
	}
	endpoint := weComMenuEndpoint("delete", payload.AccessToken, payload.AgentID)
	forwardWeComMenuRead(w, endpoint, "menu delete")
}

type menuProxyRequest struct {
	AccessToken string          `json:"access_token"`
	AgentID     string          `json:"agentid"`
	Menu        json.RawMessage `json:"menu"`
}

func decodeMenuProxyRequest(w http.ResponseWriter, r *http.Request, requireMenu bool) (menuProxyRequest, bool) {
	body, err := readBody(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing body"))
		return menuProxyRequest{}, false
	}
	var payload menuProxyRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("invalid json"))
		return menuProxyRequest{}, false
	}
	if payload.AccessToken == "" || payload.AgentID == "" || (requireMenu && len(payload.Menu) == 0) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("missing access_token/agentid/menu"))
		return menuProxyRequest{}, false
	}
	return payload, true
}

func weComMenuEndpoint(action string, accessToken string, agentID string) string {
	return fmt.Sprintf(
		"https://qyapi.weixin.qq.com/cgi-bin/menu/%s?access_token=%s&agentid=%s",
		action,
		url.QueryEscape(accessToken),
		url.QueryEscape(agentID),
	)
}

func forwardWeComMenuRead(w http.ResponseWriter, endpoint string, label string) {
	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(label + " failed: " + err.Error()))
		return
	}
	defer resp.Body.Close()
	forwardWeComMenuResponse(w, resp, label)
}

func forwardWeComMenuResponse(w http.ResponseWriter, resp *http.Response, label string) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(label + " read failed"))
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		w.WriteHeader(http.StatusBadGateway)
		if len(bytes.TrimSpace(data)) > 0 {
			_, _ = w.Write([]byte(fmt.Sprintf("%s http %d: ", label, resp.StatusCode)))
			_, _ = w.Write(data)
			return
		}
		_, _ = w.Write([]byte(fmt.Sprintf("%s http %d", label, resp.StatusCode)))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}
