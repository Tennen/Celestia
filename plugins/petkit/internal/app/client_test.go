package app

import "testing"

func TestParseRegionServerList(t *testing.T) {
	t.Run("map with list", func(t *testing.T) {
		value := map[string]any{
			"list": []any{
				map[string]any{"id": "US"},
			},
			"pref": "CN",
		}
		list, ok := parseRegionServerList(value)
		if !ok {
			t.Fatal("expected list to parse from map payload")
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 region entry, got %d", len(list))
		}
	})

	t.Run("bare list", func(t *testing.T) {
		value := []any{
			map[string]any{"id": "EU"},
		}
		list, ok := parseRegionServerList(value)
		if !ok {
			t.Fatal("expected list to parse from bare array payload")
		}
		if len(list) != 1 {
			t.Fatalf("expected 1 region entry, got %d", len(list))
		}
	})

	t.Run("invalid shape", func(t *testing.T) {
		if _, ok := parseRegionServerList(map[string]any{"pref": "CN"}); ok {
			t.Fatal("expected invalid payload to fail parsing")
		}
	})
}

func TestCompatClientPayloadMatchesUpstreamFormat(t *testing.T) {
	client := NewClient(
		AccountConfig{
			Username: "user@example.com",
			Password: "secret",
			Region:   "us",
			Timezone: "Asia/Shanghai",
		},
		defaultCompatConfig(),
	)
	got := client.compatClientPayload("Asia/Shanghai")
	want := "{'locale': 'en-US', 'name': '23127PN0CG', 'osVersion': '16.1', 'phoneBrand': 'Xiaomi', 'platform': 'android', 'source': 'app.petkit-android', 'version': '13.2.1', 'timezoneId': 'Asia/Shanghai'}"
	if got != want {
		t.Fatalf("unexpected client payload:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestSanitizeSessionBaseURLDropsLegacyPassportPath(t *testing.T) {
	compat := defaultCompatConfig()
	got := sanitizeSessionBaseURL("https://passport.petkt.com/6/", "us", compat)
	if got != "" {
		t.Fatalf("expected legacy passport path to be dropped, got %q", got)
	}
}
