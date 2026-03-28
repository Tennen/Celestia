package app

import "github.com/chentianyu/celestia/internal/models"

func controlSpecsForDevice(info petkitDeviceInfo, kind models.DeviceKind) []models.DeviceControlSpec {
	switch kind {
	case models.DeviceKindPetFeeder:
		return feederControlSpecs(info)
	case models.DeviceKindPetLitterBox:
		return litterControlSpecs()
	case models.DeviceKindPetFountain:
		return fountainControlSpecs()
	default:
		return nil
	}
}

func feederControlSpecs(info petkitDeviceInfo) []models.DeviceControlSpec {
	specs := []models.DeviceControlSpec{
		actionControlSpec("feed-once", "Feed Once", "feed_once", map[string]any{"portions": 1}),
		actionControlSpec("cancel-manual-feed", "Cancel Manual Feed", "cancel_manual_feed", nil),
		actionControlSpec("reset-desiccant", "Reset Desiccant", "reset_desiccant", nil),
	}
	if supportsFeederFoodReplenished(info.DeviceType) {
		specs = append(specs, actionControlSpec("food-replenished", "Food Replenished", "food_replenished", nil))
	}
	if supportsFeederCallPet(info.DeviceType) {
		specs = append(specs, actionControlSpec("call-pet", "Call Pet", "call_pet", nil))
	}
	return specs
}

func litterControlSpecs() []models.DeviceControlSpec {
	return []models.DeviceControlSpec{
		actionControlSpec("clean-now", "Clean Now", "clean_now", nil),
		actionControlSpec("pause", "Pause", "pause", nil),
		actionControlSpec("resume", "Resume", "resume", nil),
	}
}

func fountainControlSpecs() []models.DeviceControlSpec {
	return []models.DeviceControlSpec{
		toggleControlSpec("power", "Power", "power_status", "set_power"),
		actionControlSpec("pause", "Pause", "pause", nil),
		actionControlSpec("resume", "Resume", "resume", nil),
		actionControlSpec("reset-filter", "Reset Filter", "reset_filter", nil),
	}
}

func actionControlSpec(id, label, action string, params map[string]any) models.DeviceControlSpec {
	return models.DeviceControlSpec{
		ID:    id,
		Kind:  models.DeviceControlKindAction,
		Label: label,
		Command: &models.DeviceControlCommand{
			Action: action,
			Params: cloneControlParams(params),
		},
	}
}

func toggleControlSpec(id, label, stateKey, action string) models.DeviceControlSpec {
	return models.DeviceControlSpec{
		ID:       id,
		Kind:     models.DeviceControlKindToggle,
		Label:    label,
		StateKey: stateKey,
		OnCommand: &models.DeviceControlCommand{
			Action: action,
			Params: map[string]any{"on": true},
		},
		OffCommand: &models.DeviceControlCommand{
			Action: action,
			Params: map[string]any{"on": false},
		},
	}
}

func cloneControlParams(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
