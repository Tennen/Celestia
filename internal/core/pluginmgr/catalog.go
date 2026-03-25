package pluginmgr

import "github.com/chentianyu/celestia/internal/models"

func BuiltinCatalog() []models.CatalogPlugin {
	return []models.CatalogPlugin{
		{
			ID:          "xiaomi",
			Name:        "Xiaomi MIoT Plugin",
			Description: "Phase 1 Xiaomi cloud adapter scaffold with multi-account and region-aware device mapping.",
			BinaryName:  "xiaomi-plugin",
			Manifest: models.PluginManifest{
				ID:           "xiaomi",
				Name:         "Xiaomi MIoT Plugin",
				Version:      "0.1.0",
				Vendor:       "xiaomi",
				Capabilities: []string{"discover", "state", "command", "events", "oauth", "multi_account", "multi_region"},
				ConfigSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"accounts": map[string]any{
							"type":        "array",
							"description": "Demo accounts representing Xiaomi OAuth sessions.",
						},
						"poll_interval_seconds": map[string]any{
							"type":    "number",
							"default": 20,
						},
					},
				},
				DeviceKinds: []models.DeviceKind{models.DeviceKindLight, models.DeviceKindSwitch, models.DeviceKindSensor, models.DeviceKindClimate},
			},
		},
		{
			ID:          "petkit",
			Name:        "Petkit Plugin",
			Description: "Phase 2 Petkit scaffold covering feeder, litter box, and fountain capabilities.",
			BinaryName:  "petkit-plugin",
			Manifest: models.PluginManifest{
				ID:           "petkit",
				Name:         "Petkit Plugin",
				Version:      "0.1.0",
				Vendor:       "petkit",
				Capabilities: []string{"discover", "state", "command", "events", "relay", "media_reserved"},
				ConfigSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"accounts": map[string]any{
							"type":        "array",
							"description": "Demo Petkit cloud sessions.",
						},
					},
				},
				DeviceKinds: []models.DeviceKind{models.DeviceKindPetFeeder, models.DeviceKindPetLitterBox, models.DeviceKindPetFountain},
			},
		},
		{
			ID:          "haier",
			Name:        "Haier Washer Plugin",
			Description: "Phase 3 Haier hOn washer-focused plugin scaffold with model capability matrices.",
			BinaryName:  "haier-plugin",
			Manifest: models.PluginManifest{
				ID:           "haier",
				Name:         "Haier Washer Plugin",
				Version:      "0.1.0",
				Vendor:       "haier",
				Capabilities: []string{"discover", "state", "command", "events", "capability_matrix"},
				ConfigSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"accounts": map[string]any{
							"type":        "array",
							"description": "Demo hOn washer accounts.",
						},
					},
				},
				DeviceKinds: []models.DeviceKind{models.DeviceKindWasher},
			},
		},
	}
}

