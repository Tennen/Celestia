package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func TestGetWSSGatewayURL_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !strings.Contains(string(rawBody), `"token":"access-token"`) {
			t.Fatalf("expected body to contain token field, got %s", string(rawBody))
		}
		if strings.Contains(string(rawBody), `"accessToken"`) {
			t.Fatalf("did not expect accessToken field in body, got %s", string(rawBody))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"retCode": "00000",
			"agAddr":  "http://gw.haier.local/ws",
		})
	}))
	defer srv.Close()

	client := newTestUWSClient(AccountConfig{ClientID: "client1", RefreshToken: "refresh"}, srv)
	client.auth.AccessToken = "access-token"
	addr, err := client.getWSSGatewayURL(context.Background())
	if err != nil {
		t.Fatalf("getWSSGatewayURL failed: %v", err)
	}
	if addr != "wss://gw.haier.local/ws" {
		t.Fatalf("expected gateway URL to be rewritten to wss, got %q", addr)
	}
}

func TestBuildWSSConnectURL_UsesAccessTokenForAgClientID(t *testing.T) {
	url, agClientID, err := buildWSSConnectURL("wss://gw.haier.local/ws", "access-token")
	if err != nil {
		t.Fatalf("buildWSSConnectURL failed: %v", err)
	}
	if agClientID != "access-token" {
		t.Fatalf("agClientID = %q, want access-token", agClientID)
	}
	if url != "wss://gw.haier.local/ws/userag?token=access-token&agClientId=access-token" {
		t.Fatalf("url = %q", url)
	}
}

func TestBuildWSSConnectURL_MissingAccessToken(t *testing.T) {
	if _, _, err := buildWSSConnectURL("wss://gw.haier.local/ws", " "); err == nil {
		t.Fatal("expected error for missing access token")
	}
}

func TestBuildWSSBatchCommandMessage_UsesHACommandShape(t *testing.T) {
	msg, err := buildWSSBatchCommandMessage("access-token", "device-123", map[string]any{
		"machMode": "0",
		"prCode":   "7",
	})
	if err != nil {
		t.Fatalf("buildWSSBatchCommandMessage failed: %v", err)
	}
	if msg.AgClientID != "access-token" {
		t.Fatalf("AgClientID = %q, want access-token", msg.AgClientID)
	}
	if msg.Topic != "BatchCmdReq" {
		t.Fatalf("Topic = %q, want BatchCmdReq", msg.Topic)
	}
	trace, _ := msg.Content["trace"].(string)
	if len(trace) != 32 {
		t.Fatalf("trace len = %d, want 32", len(trace))
	}
	sn, _ := msg.Content["sn"].(string)
	if len(sn) != 32 {
		t.Fatalf("sn len = %d, want 32", len(sn))
	}

	data, ok := msg.Content["data"].([]map[string]any)
	if !ok {
		t.Fatalf("data type = %T, want []map[string]any", msg.Content["data"])
	}
	if len(data) != 1 {
		t.Fatalf("data len = %d, want 1", len(data))
	}
	entry := data[0]
	if entry["sn"] != sn {
		t.Fatalf("entry sn = %#v, want %q", entry["sn"], sn)
	}
	if entry["index"] != 0 {
		t.Fatalf("entry index = %#v, want 0", entry["index"])
	}
	if entry["delaySeconds"] != 0 {
		t.Fatalf("entry delaySeconds = %#v, want 0", entry["delaySeconds"])
	}
	if entry["subSn"] != sn+":0" {
		t.Fatalf("entry subSn = %#v, want %q", entry["subSn"], sn+":0")
	}
	if entry["deviceId"] != "device-123" {
		t.Fatalf("entry deviceId = %#v, want device-123", entry["deviceId"])
	}
	cmdArgs, ok := entry["cmdArgs"].(map[string]any)
	if !ok {
		t.Fatalf("cmdArgs type = %T, want map[string]any", entry["cmdArgs"])
	}
	if cmdArgs["machMode"] != "0" || cmdArgs["prCode"] != "7" {
		t.Fatalf("cmdArgs = %#v", cmdArgs)
	}
	if _, ok := msg.Content["cmdList"]; ok {
		t.Fatal("did not expect legacy cmdList payload")
	}
}

func TestBuildWSSBatchCommandMessage_RequiresInputs(t *testing.T) {
	if _, err := buildWSSBatchCommandMessage("", "device-123", map[string]any{"machMode": "0"}); err == nil {
		t.Fatal("expected error for missing agClientId")
	}
	if _, err := buildWSSBatchCommandMessage("access-token", "", map[string]any{"machMode": "0"}); err == nil {
		t.Fatal("expected error for missing deviceId")
	}
	if _, err := buildWSSBatchCommandMessage("access-token", "device-123", nil); err == nil {
		t.Fatal("expected error for missing params")
	}
}

// encodeWSSDeviceUpdate encodes a deviceID + attributes into the GenMsgDown/DigitalModel
// wire format: base64(JSON{dev, args: base64(gzip(JSON{attributes}))})
func encodeWSSDeviceUpdate(deviceID string, attrs map[string]string) (string, error) {
	// Build attributes array.
	type attrEntry struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	attrList := make([]attrEntry, 0, len(attrs))
	for k, v := range attrs {
		attrList = append(attrList, attrEntry{Name: k, Value: v})
	}
	layer2, err := json.Marshal(map[string]any{"attributes": attrList})
	if err != nil {
		return "", err
	}

	// gzip compress layer2.
	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	if _, err := gz.Write(layer2); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}

	// base64 encode the gzip bytes.
	argsB64 := base64.StdEncoding.EncodeToString(gzBuf.Bytes())

	// Build layer1 JSON.
	layer1, err := json.Marshal(map[string]any{
		"dev":  deviceID,
		"args": argsB64,
	})
	if err != nil {
		return "", err
	}

	// base64 encode layer1.
	return base64.StdEncoding.EncodeToString(layer1), nil
}

func TestParseWSSDeviceUpdate_RoundTrip(t *testing.T) {
	attrs := map[string]string{"machMode": "0", "prCode": "3"}
	data, err := encodeWSSDeviceUpdate("device-123", attrs)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	msg := wssMessage{
		Topic: "GenMsgDown",
		Content: map[string]any{
			"businType": "DigitalModel",
			"data":      data,
		},
	}
	deviceID, parsed, err := parseWSSDeviceUpdate(msg)
	if err != nil {
		t.Fatalf("parseWSSDeviceUpdate failed: %v", err)
	}
	if deviceID != "device-123" {
		t.Errorf("expected deviceID=device-123, got %q", deviceID)
	}
	if parsed["machMode"] != "0" {
		t.Errorf("expected machMode=0, got %q", parsed["machMode"])
	}
	if parsed["prCode"] != "3" {
		t.Errorf("expected prCode=3, got %q", parsed["prCode"])
	}
}

func TestParseWSSDeviceUpdate_WrongTopic(t *testing.T) {
	msg := wssMessage{Topic: "HeartBeat", Content: map[string]any{}}
	_, _, err := parseWSSDeviceUpdate(msg)
	if err == nil {
		t.Fatal("expected error for wrong topic")
	}
}

// TestParseWSSDeviceUpdate_Property7_RoundTrip is Property 7:
// For any valid GenMsgDown/DigitalModel message, parseWSSDeviceUpdate returns
// a non-empty deviceId and attribute map consistent with the original.
// Feature: haier-uws-platform-migration, Property 7: WSS 消息解码往返
func TestParseWSSDeviceUpdate_Property7_RoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		deviceID := rapid.StringMatching(`[a-zA-Z0-9]{4,20}`).Draw(t, "deviceID")
		n := rapid.IntRange(1, 10).Draw(t, "n")
		attrs := make(map[string]string, n)
		for i := 0; i < n; i++ {
			key := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{2,10}`).Draw(t, "key")
			val := rapid.StringMatching(`[a-zA-Z0-9]{0,10}`).Draw(t, "val")
			attrs[key] = val
		}

		data, err := encodeWSSDeviceUpdate(deviceID, attrs)
		if err != nil {
			t.Fatalf("encode failed: %v", err)
		}

		msg := wssMessage{
			Topic: "GenMsgDown",
			Content: map[string]any{
				"businType": "DigitalModel",
				"data":      data,
			},
		}
		gotDeviceID, gotAttrs, err := parseWSSDeviceUpdate(msg)
		if err != nil {
			t.Fatalf("parseWSSDeviceUpdate failed: %v", err)
		}
		if gotDeviceID != deviceID {
			t.Fatalf("deviceID mismatch: want %q got %q", deviceID, gotDeviceID)
		}
		if len(gotAttrs) != len(attrs) {
			t.Fatalf("attribute count mismatch: want %d got %d", len(attrs), len(gotAttrs))
		}
		for k, v := range attrs {
			if gotAttrs[k] != v {
				t.Fatalf("attr %q: want %q got %q", k, v, gotAttrs[k])
			}
		}
	})
}
