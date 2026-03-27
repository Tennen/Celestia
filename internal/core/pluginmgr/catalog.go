package pluginmgr

import "github.com/chentianyu/celestia/internal/models"

func BuiltinCatalog() []models.CatalogPlugin {
	return []models.CatalogPlugin{
		{
			ID:          "xiaomi",
			Name:        "Xiaomi MIoT Plugin",
			Description: "Phase 1 Xiaomi MIoT cloud integration with real account login or session reuse, multi-account, multi-region, aquarium control, and speaker text push.",
			BinaryName:  "xiaomi-plugin",
			Manifest: models.PluginManifest{
				ID:           "xiaomi",
				Name:         "Xiaomi MIoT Plugin",
				Version:      "1.0.0",
				Vendor:       "xiaomi",
				Capabilities: []string{"discover", "state", "command", "events", "oauth", "account_password_login", "real_cloud", "multi_account", "multi_region", "service_token_session", "aquarium_control", "speaker_voice_push"},
				ConfigSchema: map[string]any{
					"type": "object",
					"default": map[string]any{
						"accounts": []map[string]any{
							{
								"name":          "primary",
								"region":        "cn",
								"username":      "<xiaomi-username>",
								"password":      "<xiaomi-password>",
								"device_id":     "CELESTIAXIAOMI01",
								"verify_url":    "<optional-xiaomi-verify-url>",
								"verify_ticket": "<optional-sms-or-email-code>",
								"service_token": "<optional-service-token>",
								"ssecurity":     "<optional-ssecurity>",
								"user_id":       "<optional-user-id>",
								"cuser_id":      "<optional-cuser-id>",
								"locale":        "zh_CN",
								"timezone":      "GMT+08:00",
								"home_ids":      []string{"<optional-home-id>"},
							},
						},
						"poll_interval_seconds": 30,
					},
					"properties": map[string]any{
						"accounts": map[string]any{
							"type":        "array",
							"description": "Real Xiaomi MIoT accounts with username/password or service_token/ssecurity/user_id. OAuth client_id/redirect_url remains optional for auth-code or refresh-token flows.",
						},
						"poll_interval_seconds": map[string]any{
							"type":    "number",
							"default": 30,
						},
					},
				},
				DeviceKinds: []models.DeviceKind{
					models.DeviceKindLight,
					models.DeviceKindSwitch,
					models.DeviceKindSensor,
					models.DeviceKindClimate,
					models.DeviceKindAquarium,
					models.DeviceKindSpeaker,
				},
			},
		},
		{
			ID:          "petkit",
			Name:        "Petkit Plugin",
			Description: "Phase 2 Petkit cloud integration covering feeder, litter box, and fountain capabilities with real login/session handling.",
			BinaryName:  "petkit-plugin",
			Manifest: models.PluginManifest{
				ID:           "petkit",
				Name:         "Petkit Plugin",
				Version:      "1.0.0",
				Vendor:       "petkit",
				Capabilities: []string{"discover", "state", "command", "events", "cloud_login", "cloud_session", "feeder_control", "litter_control", "fountain_ble_relay"},
				ConfigSchema: map[string]any{
					"type": "object",
					"default": map[string]any{
						"accounts": []map[string]any{
							{
								"name":     "primary",
								"username": "<petkit-username>",
								"password": "<petkit-password>",
								"region":   "US",
								"timezone": "Asia/Shanghai",
							},
						},
						"poll_interval_seconds": 30,
						"compat": map[string]any{
							"passport_base_url": "https://passport.petkt.com/",
							"china_base_url":    "https://api.petkit.cn/6/",
							"api_version":       "13.2.1",
							"client_header":     "android(16.1;23127PN0CG)",
							"user_agent":        "okhttp/3.14.9",
							"locale":            "en-US",
							"accept_language":   "en-US;q=1, it-US;q=0.9",
							"platform":          "android",
							"os_version":        "16.1",
							"model_name":        "23127PN0CG",
							"phone_brand":       "Xiaomi",
							"source":            "app.petkit-android",
							"hour_mode":         "24",
						},
					},
					"properties": map[string]any{
						"accounts": map[string]any{
							"type":        "array",
							"description": "Real Petkit accounts with username, password, region, and timezone.",
						},
						"compat": map[string]any{
							"type":        "object",
							"description": "Optional Petkit cloud compatibility overrides. Defaults are exposed by Core and should be treated as the current upstream app signature.",
						},
						"poll_interval_seconds": map[string]any{
							"type":    "number",
							"default": 30,
						},
					},
				},
				DeviceKinds: []models.DeviceKind{models.DeviceKindPetFeeder, models.DeviceKindPetLitterBox, models.DeviceKindPetFountain},
			},
		},
		{
			ID:          "haier",
			Name:        "Haier Washer Plugin",
			Description: "Phase 3 Haier hOn washer integration with real auth, appliance discovery, and model capability matrices.",
			BinaryName:  "haier-plugin",
			Manifest: models.PluginManifest{
				ID:           "haier",
				Name:         "Haier Washer Plugin",
				Version:      "0.2.0",
				Vendor:       "haier",
				Capabilities: []string{"discover", "state", "command", "events", "real_cloud", "auth", "refresh_token", "washer_capability_matrix"},
				ConfigSchema: map[string]any{
					"type": "object",
					"default": map[string]any{
						"accounts": []map[string]any{
							{
								"name":      "primary",
								"email":     "<hon-email>",
								"password":  "<hon-password>",
								"mobile_id": "celestia-primary",
								"timezone":  "Asia/Shanghai",
							},
						},
						"poll_interval_seconds": 20,
					},
					"properties": map[string]any{
						"accounts": map[string]any{
							"type":        "array",
							"description": "Real hOn accounts with email/password or refresh_token plus optional mobile_id/timezone.",
						},
						"poll_interval_seconds": map[string]any{
							"type":    "number",
							"default": 20,
						},
					},
				},
				DeviceKinds: []models.DeviceKind{models.DeviceKindWasher},
			},
		},
	}
}
