package pluginmgr

import "github.com/chentianyu/celestia/internal/models"

func BuiltinCatalog() []models.CatalogPlugin {
	return []models.CatalogPlugin{
		{
			ID:          "xiaomi",
			Name:        "Xiaomi MIoT Plugin",
			Description: "Phase 1 Xiaomi MIoT cloud integration with real OAuth/token sessions, multi-account, multi-region, aquarium control, and speaker text push.",
			BinaryName:  "xiaomi-plugin",
			Manifest: models.PluginManifest{
				ID:           "xiaomi",
				Name:         "Xiaomi MIoT Plugin",
				Version:      "1.0.0",
				Vendor:       "xiaomi",
				Capabilities: []string{"discover", "state", "command", "events", "oauth", "real_cloud", "multi_account", "multi_region", "aquarium_control", "speaker_voice_push"},
				ConfigSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"accounts": map[string]any{
							"type":        "array",
							"description": "Real Xiaomi MIoT accounts with explicit client_id/redirect_url for refresh_token or auth_code flows.",
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
					"properties": map[string]any{
						"accounts": map[string]any{
							"type":        "array",
							"description": "Real Petkit accounts with username, password, region, and timezone.",
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
					"properties": map[string]any{
						"accounts": map[string]any{
							"type":        "array",
							"description": "Real hOn accounts with email/password or refresh_token plus optional mobile_id/timezone.",
						},
					},
				},
				DeviceKinds: []models.DeviceKind{models.DeviceKindWasher},
			},
		},
	}
}
