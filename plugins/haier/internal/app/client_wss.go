package app

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const haierWSSGatewayAPI = "https://uws.haier.net/gmsWS/wsag/assign"

// getWSSGatewayURL fetches the WebSocket gateway address for this account.
func (c *haierClient) getWSSGatewayURL(ctx context.Context) (string, error) {
	body := map[string]any{
		"clientId": c.cfg.normalizedMobileID(),
		"token":    c.auth.CognitoToken,
	}
	var payload map[string]any
	if err := c.requestJSON(ctx, http.MethodPost, haierWSSGatewayAPI, body, nil, &payload); err != nil {
		return "", fmt.Errorf("haier wss gateway: %w", err)
	}
	addr := stringFromAny(payload["agAddr"])
	if addr == "" {
		return "", fmt.Errorf("haier wss gateway: missing agAddr in response")
	}
	// The API returns http:// but the connection must use wss://.
	addr = strings.Replace(addr, "http://", "wss://", 1)
	addr = strings.Replace(addr, "https://", "wss://", 1)
	return addr, nil
}

// wssMessage is the envelope for all WebSocket messages.
type wssMessage struct {
	AgClientID string         `json:"agClientId"`
	Topic      string         `json:"topic"`
	Content    map[string]any `json:"content"`
}

// parseWSSDeviceUpdate decodes a GenMsgDown/DigitalModel WebSocket message and
// returns the deviceId and its attribute map.
func parseWSSDeviceUpdate(msg wssMessage) (deviceID string, attributes map[string]string, err error) {
	if msg.Topic != "GenMsgDown" {
		return "", nil, fmt.Errorf("not a GenMsgDown message (topic=%s)", msg.Topic)
	}
	if bt := stringFromAny(msg.Content["businType"]); bt != "DigitalModel" {
		return "", nil, fmt.Errorf("not a DigitalModel message (businType=%s)", bt)
	}

	// Layer 1: base64-decode the "data" field.
	dataB64 := stringFromAny(msg.Content["data"])
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
	// The Python code uses zlib.decompress with wbits=16+MAX_WBITS which is gzip.
	gzReader, err := zlib.NewReader(bytes.NewReader(argsRaw))
	if err != nil {
		return "", nil, fmt.Errorf("gzip open args: %w", err)
	}
	defer gzReader.Close()
	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		return "", nil, fmt.Errorf("gzip decompress args: %w", err)
	}

	// Layer 2 JSON: {"attributes": [{"name": "...", "value": "..."}, ...], ...}
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
		attributes[attr.Name] = stringFromAny(attr.Value)
	}
	return deviceID, attributes, nil
}
