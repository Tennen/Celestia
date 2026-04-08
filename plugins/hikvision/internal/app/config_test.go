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

func TestParseConfig_RenameKeepsStableIdentity(t *testing.T) {
	first, err := parseConfig(map[string]any{
		"entries": []any{
			map[string]any{
				"name":     "Front Door",
				"host":     "192.168.1.10",
				"port":     8000,
				"channel":  1,
				"username": "admin",
				"password": "secret",
			},
		},
	})
	if err != nil {
		t.Fatalf("first parseConfig() error = %v", err)
	}

	second, err := parseConfig(map[string]any{
		"entries": []any{
			map[string]any{
				"name":     "Driveway",
				"host":     "192.168.1.10",
				"port":     8000,
				"channel":  1,
				"username": "admin",
				"password": "secret",
			},
		},
	})
	if err != nil {
		t.Fatalf("second parseConfig() error = %v", err)
	}

	if first.Entries[0].EntryID != second.Entries[0].EntryID {
		t.Fatalf("entry id changed after rename: %q -> %q", first.Entries[0].EntryID, second.Entries[0].EntryID)
	}
	if first.Entries[0].DeviceID != second.Entries[0].DeviceID {
		t.Fatalf("device id changed after rename: %q -> %q", first.Entries[0].DeviceID, second.Entries[0].DeviceID)
	}
}

func TestParseConfig_StreamSessionDefaults(t *testing.T) {
	cfg, err := parseConfig(map[string]any{
		"entries": []any{
			map[string]any{
				"host":     "192.168.1.10",
				"username": "admin",
				"password": "secret",
			},
		},
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	entry := cfg.Entries[0]
	if entry.MaxStreamSessions != defaultMaxStreamSessions {
		t.Errorf("MaxStreamSessions = %d, want %d", entry.MaxStreamSessions, defaultMaxStreamSessions)
	}
	if entry.StreamIdleTimeoutSeconds != defaultStreamIdleTimeoutSeconds {
		t.Errorf("StreamIdleTimeoutSeconds = %d, want %d", entry.StreamIdleTimeoutSeconds, defaultStreamIdleTimeoutSeconds)
	}
}

func TestParseConfig_StreamSessionMinEnforcement(t *testing.T) {
	cfg, err := parseConfig(map[string]any{
		"entries": []any{
			map[string]any{
				"host":                        "192.168.1.10",
				"username":                    "admin",
				"password":                    "secret",
				"max_stream_sessions":         0,
				"stream_idle_timeout_seconds": 5,
			},
		},
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	entry := cfg.Entries[0]
	// zero max_stream_sessions should fall back to default (4)
	if entry.MaxStreamSessions != defaultMaxStreamSessions {
		t.Errorf("MaxStreamSessions = %d, want %d (default)", entry.MaxStreamSessions, defaultMaxStreamSessions)
	}
	// stream_idle_timeout_seconds below min (10) should be clamped to min
	if entry.StreamIdleTimeoutSeconds != minStreamIdleTimeoutSeconds {
		t.Errorf("StreamIdleTimeoutSeconds = %d, want %d (min)", entry.StreamIdleTimeoutSeconds, minStreamIdleTimeoutSeconds)
	}
}

func TestParseConfig_StreamSessionBelowMinSessions(t *testing.T) {
	cfg, err := parseConfig(map[string]any{
		"entries": []any{
			map[string]any{
				"host":                "192.168.1.10",
				"username":            "admin",
				"password":            "secret",
				"max_stream_sessions": -3,
			},
		},
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	entry := cfg.Entries[0]
	if entry.MaxStreamSessions != minMaxStreamSessions {
		t.Errorf("MaxStreamSessions = %d, want %d (min)", entry.MaxStreamSessions, minMaxStreamSessions)
	}
}

func TestParseConfig_StreamSessionExplicitValues(t *testing.T) {
	cfg, err := parseConfig(map[string]any{
		"entries": []any{
			map[string]any{
				"host":                        "192.168.1.10",
				"username":                    "admin",
				"password":                    "secret",
				"max_stream_sessions":         8,
				"stream_idle_timeout_seconds": 120,
			},
		},
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	entry := cfg.Entries[0]
	if entry.MaxStreamSessions != 8 {
		t.Errorf("MaxStreamSessions = %d, want 8", entry.MaxStreamSessions)
	}
	if entry.StreamIdleTimeoutSeconds != 120 {
		t.Errorf("StreamIdleTimeoutSeconds = %d, want 120", entry.StreamIdleTimeoutSeconds)
	}
}

func TestParseConfig_CloudModeAcceptsDeviceSerialWithoutCloudAuth(t *testing.T) {
	cfg, err := parseConfig(map[string]any{
		"mode": "cloud",
		"entries": []any{
			map[string]any{
				"device_serial": "J12345678",
				"rtsp_url":      "rtsp://viewer:secret@example.invalid/live",
			},
		},
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}
	if cfg.Mode != RuntimeModeCloud {
		t.Fatalf("Mode = %q, want cloud", cfg.Mode)
	}
	if cfg.Entries[0].DeviceSerial != "J12345678" {
		t.Fatalf("DeviceSerial = %q, want J12345678", cfg.Entries[0].DeviceSerial)
	}
	if cfg.Cloud.HasAuth() {
		t.Fatal("cloud auth should be empty when not configured")
	}
}

func TestParseConfig_CloudModeRequiresIdentityOrRTSP(t *testing.T) {
	_, err := parseConfig(map[string]any{
		"mode": "cloud",
		"entries": []any{
			map[string]any{
				"name": "camera-without-identity",
			},
		},
	})
	if err == nil {
		t.Fatal("expected parseConfig to fail without device_serial or RTSP settings in cloud mode")
	}
}
