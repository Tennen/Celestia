package client

import "github.com/chentianyu/celestia/internal/models"

// ControlSpecsForDevice returns the control specs for a given device.
func ControlSpecsForDevice(info PetkitDeviceInfo, kind models.DeviceKind) []models.DeviceControlSpec {
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

func feederControlSpecs(info PetkitDeviceInfo) []models.DeviceControlSpec {
	minPortions := 1.0
	stepPortions := 1.0
	specs := []models.DeviceControlSpec{
		actionControlSpec("feed-once", "Feed Once", "feed_once", map[string]any{"portions": 1}, []models.DeviceCommandParamSpec{{
			Name:    "portions",
			Type:    models.DeviceCommandParamTypeNumber,
			Default: 1,
			Min:     &minPortions,
			Step:    &stepPortions,
		}}),
		actionControlSpec("cancel-manual-feed", "Cancel Manual Feed", "cancel_manual_feed", nil),
		actionControlSpec("reset-desiccant", "Reset Desiccant", "reset_desiccant", nil),
	}
	if SupportsFeederFoodReplenished(info.DeviceType) {
		specs = append(specs, actionControlSpec("food-replenished", "Food Replenished", "food_replenished", nil))
	}
	if SupportsFeederCallPet(info.DeviceType) {
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

func actionControlSpec(id, label, action string, params map[string]any, paramsSpec ...[]models.DeviceCommandParamSpec) models.DeviceControlSpec {
	var declaredParams []models.DeviceCommandParamSpec
	if len(paramsSpec) > 0 {
		declaredParams = cloneCommandParams(paramsSpec[0])
	}
	return models.DeviceControlSpec{
		ID:    id,
		Kind:  models.DeviceControlKindAction,
		Label: label,
		Command: &models.DeviceControlCommand{
			Action:     action,
			Params:     cloneControlParams(params),
			ParamsSpec: declaredParams,
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

func cloneCommandParams(input []models.DeviceCommandParamSpec) []models.DeviceCommandParamSpec {
	if len(input) == 0 {
		return nil
	}
	out := make([]models.DeviceCommandParamSpec, len(input))
	copy(out, input)
	return out
}
