package control

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

type Service struct{}

type controlSpec struct {
	view      models.DeviceControl
	onAction  string
	onParams  map[string]any
	offAction string
	offParams map[string]any
	action    string
	params    map[string]any
}

func New() *Service {
	return &Service{}
}

func (s *Service) BuildView(device models.Device, state models.DeviceStateSnapshot) models.DeviceView {
	return models.DeviceView{
		Device:   device,
		State:    state,
		Controls: s.List(device, state),
	}
}

func (s *Service) ApplyPreferences(view models.DeviceView, prefs []models.DeviceControlPreference) models.DeviceView {
	indexed := make(map[string]models.DeviceControlPreference, len(prefs))
	for _, pref := range prefs {
		indexed[pref.ControlID] = pref
	}
	for idx := range view.Controls {
		control := view.Controls[idx]
		control.DefaultLabel = control.Label
		control.Visible = true
		if pref, ok := indexed[control.ID]; ok {
			control.Alias = pref.Alias
			control.Visible = pref.Visible
			if strings.TrimSpace(pref.Alias) != "" {
				control.Label = pref.Alias
			}
		}
		view.Controls[idx] = control
	}
	return view
}

func (s *Service) List(device models.Device, state models.DeviceStateSnapshot) []models.DeviceControl {
	specs := controlSpecs(device, state)
	out := make([]models.DeviceControl, 0, len(specs))
	for _, item := range specs {
		out = append(out, item.view)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) ResolveToggle(device models.Device, state models.DeviceStateSnapshot, controlID string, on bool) (models.CommandRequest, error) {
	for _, item := range controlSpecs(device, state) {
		if item.view.Kind != models.DeviceControlKindToggle || item.view.ID != controlID {
			continue
		}
		action := item.onAction
		params := cloneParams(item.onParams)
		if !on {
			action = item.offAction
			params = cloneParams(item.offParams)
		}
		if action == "" {
			return models.CommandRequest{}, fmt.Errorf("toggle %q does not support this state", controlID)
		}
		return models.CommandRequest{
			DeviceID: device.ID,
			Action:   action,
			Params:   params,
		}, nil
	}
	return models.CommandRequest{}, fmt.Errorf("toggle %q not found", controlID)
}

func (s *Service) ResolveAction(device models.Device, state models.DeviceStateSnapshot, controlID string) (models.CommandRequest, error) {
	for _, item := range controlSpecs(device, state) {
		if item.view.Kind != models.DeviceControlKindAction || item.view.ID != controlID {
			continue
		}
		if item.action == "" {
			return models.CommandRequest{}, fmt.Errorf("action %q is not executable", controlID)
		}
		return models.CommandRequest{
			DeviceID: device.ID,
			Action:   item.action,
			Params:   cloneParams(item.params),
		}, nil
	}
	return models.CommandRequest{}, fmt.Errorf("action %q not found", controlID)
}

func controlSpecs(device models.Device, state models.DeviceStateSnapshot) []controlSpec {
	specs := make([]controlSpec, 0, 8)
	added := map[string]bool{}
	appendSpec := func(item controlSpec) {
		if item.view.ID == "" || added[item.view.ID] {
			return
		}
		added[item.view.ID] = true
		specs = append(specs, item)
	}

	toggleRefs := parseToggleRefs(device, state)
	for _, item := range toggleRefs {
		appendSpec(item)
	}

	if len(toggleRefs) == 0 && hasAnyCapability(device, "power", "set_power", "turn_on", "turn_off") {
		appendSpec(toggleSpec("power", "Power", "Turn the device on or off.", toggleState(state, "power", "power_status"), "set_power", map[string]any{"on": true}, "set_power", map[string]any{"on": false}))
	}
	if hasCapability(device, "pump_power") {
		appendSpec(toggleSpec("pump", "Pump", "Control the aquarium pump.", toggleState(state, "pump_power"), "set_pump_power", map[string]any{"on": true}, "set_pump_power", map[string]any{"on": false}))
	}
	if hasCapability(device, "light_power") {
		appendSpec(toggleSpec("light", "Light", "Control the aquarium light.", toggleState(state, "light_power"), "set_light_power", map[string]any{"on": true}, "set_light_power", map[string]any{"on": false}))
	}
	if hasCapability(device, "mute") {
		appendSpec(toggleSpec("mute", "Mute", "Mute or unmute the speaker.", toggleState(state, "mute"), "set_mute", map[string]any{"on": true}, "set_mute", map[string]any{"on": false}))
	}
	if hasCapability(device, "feed_once") {
		appendSpec(actionSpec("feed-once", "Feed Once", "Dispense a single feeding portion.", "feed_once", map[string]any{"portions": 1}))
	}
	if hasCapability(device, "clean_now") {
		appendSpec(actionSpec("clean-now", "Clean Now", "Start an immediate cleaning cycle.", "clean_now", nil))
	}
	if hasCapability(device, "start") {
		appendSpec(actionSpec("start", "Start", "Start the current appliance cycle.", "start", nil))
	}
	if hasCapability(device, "pause") {
		appendSpec(actionSpec("pause", "Pause", "Pause the current appliance cycle.", "pause", nil))
	}
	if hasCapability(device, "resume") {
		appendSpec(actionSpec("resume", "Resume", "Resume the paused appliance cycle.", "resume", nil))
	}
	if hasCapability(device, "reset_filter") {
		appendSpec(actionSpec("reset-filter", "Reset Filter", "Reset the filter maintenance counter.", "reset_filter", nil))
	}

	return specs
}

func actionSpec(id, label, description, action string, params map[string]any) controlSpec {
	return controlSpec{
		view: models.DeviceControl{
			ID:          id,
			Kind:        models.DeviceControlKindAction,
			Label:       label,
			Description: description,
			Visible:     true,
		},
		action: action,
		params: params,
	}
}

func toggleSpec(id, label, description string, state *bool, onAction string, onParams map[string]any, offAction string, offParams map[string]any) controlSpec {
	return controlSpec{
		view: models.DeviceControl{
			ID:          id,
			Kind:        models.DeviceControlKindToggle,
			Label:       label,
			Description: description,
			State:       state,
			Visible:     true,
		},
		onAction:  onAction,
		onParams:  onParams,
		offAction: offAction,
		offParams: offParams,
	}
}

func parseToggleRefs(device models.Device, state models.DeviceStateSnapshot) []controlSpec {
	raw, ok := device.Metadata["toggle_refs"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	specs := make([]controlSpec, 0, len(items))
	for _, entry := range items {
		toggle, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		id := stringValue(toggle["id"])
		if id == "" {
			continue
		}
		label := stringValue(toggle["label"])
		if label == "" {
			label = id
		}
		stateKey := stringValue(toggle["state_key"])
		description := stringValue(toggle["description"])
		specs = append(specs, toggleSpec(
			id,
			label,
			firstNonEmpty(description, "Control an individual switch channel."),
			toggleState(state, stateKey),
			"set_toggle",
			map[string]any{"toggle_id": id, "on": true},
			"set_toggle",
			map[string]any{"toggle_id": id, "on": false},
		))
	}
	return specs
}

func toggleState(state models.DeviceStateSnapshot, keys ...string) *bool {
	for _, key := range keys {
		if key == "" || state.State == nil {
			continue
		}
		raw, ok := state.State[key]
		if !ok {
			continue
		}
		if value, ok := boolFromAny(raw); ok {
			return &value
		}
	}
	return nil
}

func boolFromAny(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case int:
		return typed != 0, true
	case int64:
		return typed != 0, true
	case float64:
		return typed != 0, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "on", "opened", "running":
			return true, true
		case "0", "false", "off", "closed", "stopped":
			return false, true
		default:
			if number, err := strconv.Atoi(typed); err == nil {
				return number != 0, true
			}
		}
	}
	return false, false
}

func cloneParams(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func hasCapability(device models.Device, capability string) bool {
	for _, item := range device.Capabilities {
		if item == capability {
			return true
		}
	}
	return false
}

func hasAnyCapability(device models.Device, capabilities ...string) bool {
	for _, capability := range capabilities {
		if hasCapability(device, capability) {
			return true
		}
	}
	return false
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func ParseCompoundControlID(value string) (string, string, error) {
	index := strings.LastIndex(value, ".")
	if index <= 0 || index == len(value)-1 {
		return "", "", errors.New("control id must look like <device_id>.<control_id>")
	}
	return value[:index], value[index+1:], nil
}
