package app

import "testing"

func TestParseConfig_AssignsStableIDsAndDefaults(t *testing.T) {
	cfg, err := parseConfig(map[string]any{
		"poll_interval_seconds": 1,
		"entries": []any{
			map[string]any{
				"name":     "Front Door",
				"host":     "192.168.1.10",
				"username": "admin",
				"password": "secret",
			},
			map[string]any{
				"name":     "Front Door",
				"host":     "192.168.1.11",
				"username": "admin",
				"password": "secret",
			},
		},
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	if cfg.PollIntervalSeconds != minPollIntervalSeconds {
		t.Fatalf("poll interval = %d, want %d", cfg.PollIntervalSeconds, minPollIntervalSeconds)
	}
	if len(cfg.Entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(cfg.Entries))
	}
	if cfg.Entries[0].EntryID == cfg.Entries[1].EntryID {
		t.Fatalf("entry ids should be unique: %q", cfg.Entries[0].EntryID)
	}
	if cfg.Entries[0].DeviceID == cfg.Entries[1].DeviceID {
		t.Fatalf("device ids should be unique: %q", cfg.Entries[0].DeviceID)
	}
	if cfg.Entries[0].SDKLibDir == "" {
		t.Fatal("sdk lib dir should have default")
	}
}

func TestParseConfig_RequiresCredentials(t *testing.T) {
	_, err := parseConfig(map[string]any{
		"entries": []any{
			map[string]any{
				"host":     "192.168.1.10",
				"username": "",
				"password": "",
			},
		},
	})
	if err == nil {
		t.Fatal("expected parseConfig to fail without credentials")
	}
}
