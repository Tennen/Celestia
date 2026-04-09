package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const uwsWSSGatewayURL = "https://uws.haier.net/gmsWS/wsag/assign"

// getWSSGatewayURL fetches the WebSocket gateway address for this account.
// POST body: {"clientId": "<clientId>", "token": "<accessToken>"}
// WSS URL format: {agAddr}/userag?token={accessToken}&agClientId={accessToken}
func (c *UWSClient) getWSSGatewayURL(ctx context.Context) (string, error) {
	body := map[string]any{
		"clientId": c.cfg.ClientID,
		"token":    c.auth.AccessToken,
	}
	req, err := c.newSignedRequest(ctx, http.MethodPost, uwsWSSGatewayURL, body)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("uws wss gateway: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("uws wss gateway failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var result struct {
		RetCode string `json:"retCode"`
		RetInfo string `json:"retInfo"`
		AgAddr  string `json:"agAddr"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("uws wss gateway decode: %w", err)
	}
	if result.RetCode != "" && result.RetCode != "00000" {
		return "", fmt.Errorf("uws wss gateway failed: retCode=%s retInfo=%s", result.RetCode, result.RetInfo)
	}
	addr := StringFromAny(result.AgAddr)
	if addr == "" {
		return "", fmt.Errorf("uws wss gateway: missing agAddr in response")
	}
	addr = strings.Replace(addr, "http://", "wss://", 1)
	addr = strings.Replace(addr, "https://", "wss://", 1)
	return addr, nil
}

func buildWSSConnectURL(gatewayURL, accessToken string) (string, string, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return "", "", fmt.Errorf("uws wss: missing access token")
	}
	return fmt.Sprintf("%s/userag?token=%s&agClientId=%s", gatewayURL, accessToken, accessToken), accessToken, nil
}

// wssMessage is the envelope for all WebSocket messages.
type wssMessage struct {
	AgClientID string         `json:"agClientId"`
	Topic      string         `json:"topic"`
	Content    map[string]any `json:"content"`
}

// parseWSSDeviceUpdate decodes a GenMsgDown/DigitalModel WebSocket message and
// returns the deviceId and its attribute map.
// Encoding: base64 → JSON → base64 → gzip → JSON
func parseWSSDeviceUpdate(msg wssMessage) (deviceID string, attributes map[string]string, err error) {
	if msg.Topic != "GenMsgDown" {
		return "", nil, fmt.Errorf("not a GenMsgDown message (topic=%s)", msg.Topic)
	}
	if bt := StringFromAny(msg.Content["businType"]); bt != "DigitalModel" {
		return "", nil, fmt.Errorf("not a DigitalModel message (businType=%s)", bt)
	}

	// Layer 1: base64-decode the "data" field.
	dataB64 := StringFromAny(msg.Content["data"])
	if dataB64 == "" {
		return "", nil, fmt.Errorf("missing data field in GenMsgDown content")
	}
	layer1, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return "", nil, fmt.Errorf("base64 decode layer1: %w", err)
	}

	// Layer 1 JSON: {"dev": "<deviceId>", "args": "<base64+gzip>"}
	var layer1JSON struct {
		Dev  string `json:"dev"`
		Args string `json:"args"`
	}
	if err := json.Unmarshal(layer1, &layer1JSON); err != nil {
		return "", nil, fmt.Errorf("decode layer1 json: %w", err)
	}
	deviceID = layer1JSON.Dev

	// Layer 2: base64-decode then gzip-decompress the "args" field.
	argsRaw, err := base64.StdEncoding.DecodeString(layer1JSON.Args)
	if err != nil {
		return "", nil, fmt.Errorf("base64 decode args: %w", err)
	}
	gzReader, err := gzip.NewReader(bytes.NewReader(argsRaw))
	if err != nil {
		return "", nil, fmt.Errorf("gzip open args: %w", err)
	}
	defer gzReader.Close()
	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		return "", nil, fmt.Errorf("gzip decompress args: %w", err)
	}

	// Layer 2 JSON: {"attributes": [{"name": "...", "value": "..."}, ...]}
	var layer2JSON struct {
		Attributes []struct {
			Name  string `json:"name"`
			Value any    `json:"value"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(decompressed, &layer2JSON); err != nil {
		return "", nil, fmt.Errorf("decode layer2 json: %w", err)
	}

	attributes = make(map[string]string, len(layer2JSON.Attributes))
	for _, attr := range layer2JSON.Attributes {
		if attr.Name == "" {
			continue
		}
		attributes[attr.Name] = StringFromAny(attr.Value)
	}
	return deviceID, attributes, nil
}
