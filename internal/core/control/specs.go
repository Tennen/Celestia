package control

import (
	"encoding/json"

	"github.com/chentianyu/celestia/internal/models"
)

type controlSpec struct {
	view       models.DeviceControl
	command    *models.DeviceControlCommand
	onCommand  *models.DeviceControlCommand
	offCommand *models.DeviceControlCommand
}

func controlSpecs(device models.Device, state models.DeviceStateSnapshot) []controlSpec {
	declared := parseDeclaredControls(device)
	specs := make([]controlSpec, 0, len(declared))
	added := map[string]bool{}
	for _, declaredSpec := range declared {
		item, ok := buildControlSpec(declaredSpec, state)
		if !ok || item.view.ID == "" || added[item.view.ID] {
			continue
		}
		added[item.view.ID] = true
		specs = append(specs, item)
	}
	return specs
}

func buildControlSpec(spec models.DeviceControlSpec, state models.DeviceStateSnapshot) (controlSpec, bool) {
	view := models.DeviceControl{
		ID:             spec.ID,
		Kind:           spec.Kind,
		Label:          firstNonEmpty(spec.Label, spec.ID),
		Disabled:       spec.Disabled,
		DisabledReason: spec.DisabledReason,
		Visible:        true,
	}
	switch spec.Kind {
	case models.DeviceControlKindToggle:
		if spec.OnCommand == nil || spec.OffCommand == nil {
			return controlSpec{}, false
		}
		view.State = toggleState(state, spec.StateKey)
		return controlSpec{
			view:       view,
			onCommand:  cloneCommand(spec.OnCommand),
			offCommand: cloneCommand(spec.OffCommand),
		}, true
	case models.DeviceControlKindAction:
		if spec.Command == nil || spec.Command.Action == "" {
			return controlSpec{}, false
		}
		view.Command = cloneCommand(spec.Command)
		return controlSpec{
			view:    view,
			command: cloneCommand(spec.Command),
		}, true
	case models.DeviceControlKindSelect:
		if spec.Command == nil || spec.Command.Action == "" || spec.Command.ValueParam == "" || len(spec.Options) == 0 {
			return controlSpec{}, false
		}
		view.Value = stateValue(state, spec.StateKey)
		view.Options = cloneOptions(spec.Options)
		view.Command = cloneCommand(spec.Command)
		return controlSpec{view: view}, true
	case models.DeviceControlKindNumber:
		if spec.Command == nil || spec.Command.Action == "" || spec.Command.ValueParam == "" {
			return controlSpec{}, false
		}
		view.Value = stateValue(state, spec.StateKey)
		view.Min = cloneNumberPtr(spec.Min)
		view.Max = cloneNumberPtr(spec.Max)
		view.Step = cloneNumberPtr(spec.Step)
		view.Unit = spec.Unit
		view.Command = cloneCommand(spec.Command)
		return controlSpec{view: view}, true
	default:
		return controlSpec{}, false
	}
}

func parseDeclaredControls(device models.Device) []models.DeviceControlSpec {
	if device.Metadata == nil {
		return nil
	}
	specs := decodeControlSpecs(device.Metadata["controls"])
	if len(specs) > 0 {
		return specs
	}
	return append(parseLegacyToggleRefs(device), parseLegacyValueControls(device)...)
}

func decodeControlSpecs(value any) []models.DeviceControlSpec {
	switch typed := value.(type) {
	case nil:
		return nil
	case []models.DeviceControlSpec:
		out := make([]models.DeviceControlSpec, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var specs []models.DeviceControlSpec
	if err := json.Unmarshal(raw, &specs); err != nil {
		return nil
	}
	return specs
}

func parseLegacyToggleRefs(device models.Device) []models.DeviceControlSpec {
	items := mapSlice(device.Metadata["toggle_refs"])
	if len(items) == 0 {
		return nil
	}
	specs := make([]models.DeviceControlSpec, 0, len(items))
	for _, toggle := range items {
		id := stringValue(toggle["id"])
		if id == "" {
			continue
		}
		specs = append(specs, models.DeviceControlSpec{
			ID:       id,
			Kind:     models.DeviceControlKindToggle,
			Label:    firstNonEmpty(stringValue(toggle["label"]), id),
			StateKey: stringValue(toggle["state_key"]),
			OnCommand: &models.DeviceControlCommand{
				Action: "set_toggle",
				Params: map[string]any{"toggle_id": id, "on": true},
			},
			OffCommand: &models.DeviceControlCommand{
				Action: "set_toggle",
				Params: map[string]any{"toggle_id": id, "on": false},
			},
		})
	}
	return specs
}

func parseLegacyValueControls(device models.Device) []models.DeviceControlSpec {
	items := mapSlice(device.Metadata["value_controls"])
	if len(items) == 0 {
		return nil
	}
	specs := make([]models.DeviceControlSpec, 0, len(items))
	for _, valueControl := range items {
		id := stringValue(valueControl["id"])
		if id == "" {
			continue
		}
		kind := models.DeviceControlKind(stringValue(valueControl["kind"]))
		if kind != models.DeviceControlKindSelect && kind != models.DeviceControlKindNumber {
			continue
		}
		action := stringValue(valueControl["action"])
		valueParam := stringValue(valueControl["value_param"])
		if action == "" || valueParam == "" {
			continue
		}
		spec := models.DeviceControlSpec{
			ID:       id,
			Kind:     kind,
			Label:    firstNonEmpty(stringValue(valueControl["label"]), id),
			StateKey: stringValue(valueControl["state_key"]),
			Unit:     stringValue(valueControl["unit"]),
			Command: &models.DeviceControlCommand{
				Action:     action,
				Params:     mapValue(valueControl["params"]),
				ValueParam: valueParam,
			},
		}
		if kind == models.DeviceControlKindNumber {
			spec.Min = numberPtr(valueControl["min"])
			spec.Max = numberPtr(valueControl["max"])
			spec.Step = numberPtr(valueControl["step"])
		}
		if kind == models.DeviceControlKindSelect {
			spec.Options = parseControlOptions(valueControl["options"])
			if len(spec.Options) == 0 {
				continue
			}
		}
		specs = append(specs, spec)
	}
	return specs
}
