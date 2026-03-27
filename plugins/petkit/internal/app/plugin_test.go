package app

import "testing"

func TestParseConfigAppliesCompatDefaults(t *testing.T) {
	cfg, err := parseConfig(map[string]any{
		"accounts": []any{
			map[string]any{
				"name":     "primary",
				"username": "user@example.com",
				"password": "secret",
				"region":   "US",
				"timezone": "Asia/Shanghai",
			},
		},
	})
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg.Compat.PassportBaseURL != "https://passport.petkt.com/" {
		t.Fatalf("unexpected passport base url: %s", cfg.Compat.PassportBaseURL)
	}
	if cfg.Compat.APIVersion != "12.4.9" {
		t.Fatalf("unexpected api version: %s", cfg.Compat.APIVersion)
	}
	if cfg.Compat.ClientHeader != "android(15.1;23127PN0CG)" {
		t.Fatalf("unexpected client header: %s", cfg.Compat.ClientHeader)
	}
}

func TestParseConfigOverridesCompatFields(t *testing.T) {
	cfg, err := parseConfig(map[string]any{
		"accounts": []any{
			map[string]any{
				"name":     "primary",
				"username": "user@example.com",
				"password": "secret",
				"region":   "US",
			},
		},
		"compat": map[string]any{
			"passport_base_url": "https://override.example/latest/",
			"api_version":       "99.1.0",
		},
	})
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg.Compat.PassportBaseURL != "https://override.example/latest/" {
		t.Fatalf("unexpected passport base url override: %s", cfg.Compat.PassportBaseURL)
	}
	if cfg.Compat.APIVersion != "99.1.0" {
		t.Fatalf("unexpected api version override: %s", cfg.Compat.APIVersion)
	}
	if cfg.Compat.ClientHeader != "android(15.1;23127PN0CG)" {
		t.Fatalf("expected untouched default client header, got %s", cfg.Compat.ClientHeader)
	}
}

func TestParseConfigUpgradesLegacyCompatDefaults(t *testing.T) {
	cfg, err := parseConfig(map[string]any{
		"accounts": []any{
			map[string]any{
				"name":     "primary",
				"username": "user@example.com",
				"password": "secret",
				"region":   "US",
			},
		},
		"compat": map[string]any{
			"api_version":   "13.2.1",
			"client_header": "android(16.1;23127PN0CG)",
			"user_agent":    "okhttp/3.14.9",
			"os_version":    "16.1",
		},
	})
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if cfg.Compat.APIVersion != "12.4.9" {
		t.Fatalf("expected legacy api version to upgrade, got %s", cfg.Compat.APIVersion)
	}
	if cfg.Compat.ClientHeader != "android(15.1;23127PN0CG)" {
		t.Fatalf("expected legacy client header to upgrade, got %s", cfg.Compat.ClientHeader)
	}
	if cfg.Compat.UserAgent != "okhttp/3.14.19" {
		t.Fatalf("expected legacy user agent to upgrade, got %s", cfg.Compat.UserAgent)
	}
	if cfg.Compat.OSVersion != "15.1" {
		t.Fatalf("expected legacy os version to upgrade, got %s", cfg.Compat.OSVersion)
	}
}
