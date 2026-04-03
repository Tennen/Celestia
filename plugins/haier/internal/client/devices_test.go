package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

func newDevicesTestClient(srv *httptest.Server) *UWSClient {
	return &UWSClient{
		cfg:  AccountConfig{ClientID: "client1", RefreshToken: "token"},
		auth: uwsAuthState{AccessToken: "access-token", RefreshToken: "token"},
		client: &http.Client{
			Transport: &rewriteHostTransport{target: srv.URL},
		},
	}
}

// TestLoadAppliances_Success verifies basic happy-path parsing.
func TestLoadAppliances_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"retCode": "00000",
			"retInfo": "成功",
			"deviceinfos": []any{
				map[string]any{"deviceId": "dev1", "deviceName": "Washer A", "online": true},
				map[string]any{"deviceId": "dev2", "deviceName": "Washer B", "online": false},
			},
		})
	}))
	defer srv.Close()

	c := newDevicesTestClient(srv)
	appliances, err := c.LoadAppliances(context.Background())
	if err != nil {
		t.Fatalf("LoadAppliances failed: %v", err)
	}
	if len(appliances) != 2 {
		t.Fatalf("expected 2 appliances, got %d", len(appliances))
	}
}

// TestLoadAppliances_NonZeroRetCode verifies error on non-zero retCode.
func TestLoadAppliances_NonZeroRetCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"retCode": "12345",
			"retInfo": "error",
		})
	}))
	defer srv.Close()

	c := newDevicesTestClient(srv)
	_, err := c.LoadAppliances(context.Background())
	if err == nil {
		t.Fatal("expected error for non-zero retCode, got nil")
	}
	if !strings.Contains(err.Error(), "12345") {
		t.Errorf("error should contain retCode, got: %v", err)
	}
}

// TestLoadDigitalModels_Success verifies attribute parsing.
func TestLoadDigitalModels_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"retCode": "00000",
			"detailInfo": map[string]any{
				"dev1": `{"attributes":[{"name":"machMode","value":"0"},{"name":"prCode","value":"3"}]}`,
			},
		})
	}))
	defer srv.Close()

	c := newDevicesTestClient(srv)
	result, err := c.LoadDigitalModels(context.Background(), []string{"dev1"})
	if err != nil {
		t.Fatalf("LoadDigitalModels failed: %v", err)
	}
	attrs, ok := result["dev1"]
	if !ok {
		t.Fatal("expected dev1 in result")
	}
	if attrs["machMode"] != "0" {
		t.Errorf("expected machMode=0, got %q", attrs["machMode"])
	}
	if attrs["prCode"] != "3" {
		t.Errorf("expected prCode=3, got %q", attrs["prCode"])
	}
}

// TestLoadAppliances_Property3_ParseCompleteness is Property 3:
// For any N-device response with retCode=="00000", LoadAppliances returns exactly N entries
// each with a non-empty deviceId.
// Feature: haier-uws-platform-migration, Property 3: 设备列表解析完整性
func TestLoadAppliances_Property3_ParseCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 20).Draw(t, "n")
		devices := make([]any, n)
		for i := range devices {
			devices[i] = map[string]any{
				"deviceId":   rapid.StringMatching(`[a-zA-Z0-9]{4,16}`).Draw(t, "deviceId"),
				"deviceName": rapid.String().Draw(t, "deviceName"),
			}
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"retCode":     "00000",
				"deviceinfos": devices,
			})
		}))
		defer srv.Close()

		c := newDevicesTestClient(srv)
		result, err := c.LoadAppliances(context.Background())
		if err != nil {
			t.Fatalf("LoadAppliances failed: %v", err)
		}
		if len(result) != n {
			t.Fatalf("expected %d appliances, got %d", n, len(result))
		}
		for _, item := range result {
			if StringFromAny(item["deviceId"]) == "" {
				t.Fatal("appliance missing deviceId")
			}
		}
	})
}

// TestLoadAppliances_Property4_NonZeroRetCodeError is Property 4:
// For any retCode != "00000", LoadAppliances returns a non-nil error containing the retCode.
// Feature: haier-uws-platform-migration, Property 4: 非零 retCode 返回错误
func TestLoadAppliances_Property4_NonZeroRetCodeError(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		retCode := rapid.StringMatching(`[0-9]{5}`).Draw(t, "retCode")
		if retCode == "00000" {
			retCode = "99999"
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"retCode": retCode,
				"retInfo": "error",
			})
		}))
		defer srv.Close()

		c := newDevicesTestClient(srv)
		_, err := c.LoadAppliances(context.Background())
		if err == nil {
			t.Fatalf("expected error for retCode=%s, got nil", retCode)
		}
		if !strings.Contains(err.Error(), retCode) {
			t.Fatalf("error %q should contain retCode %q", err.Error(), retCode)
		}
	})
}

// TestLoadDigitalModels_Property5_AttributeParseRoundTrip is Property 5:
// For any valid digital model response, the number of parsed attributes equals
// the number of non-empty-name attributes in the response.
// Feature: haier-uws-platform-migration, Property 5: 数字模型属性解析往返
func TestLoadDigitalModels_Property5_AttributeParseRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 15).Draw(t, "n")
		attrs := make([]any, n)
		expectedCount := 0
		for i := range attrs {
			name := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{2,10}`).Draw(t, "attrName")
			value := rapid.String().Draw(t, "attrValue")
			attrs[i] = map[string]any{"name": name, "value": value}
			expectedCount++
		}
		rawDetail, err := json.Marshal(map[string]any{"attributes": attrs})
		if err != nil {
			t.Fatalf("marshal detail: %v", err)
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"retCode": "00000",
				"detailInfo": map[string]any{
					"dev1": string(rawDetail),
				},
			})
		}))
		defer srv.Close()

		c := newDevicesTestClient(srv)
		result, err := c.LoadDigitalModels(context.Background(), []string{"dev1"})
		if err != nil {
			t.Fatalf("LoadDigitalModels failed: %v", err)
		}
		parsed, ok := result["dev1"]
		if !ok {
			t.Fatal("expected dev1 in result")
		}
		if len(parsed) != expectedCount {
			t.Fatalf("expected %d attributes, got %d", expectedCount, len(parsed))
		}
	})
}
