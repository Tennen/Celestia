package mapper

import (
	"fmt"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
)

func discoverToggleChannels(services []serviceView) []ToggleChannel {
	channels := make([]ToggleChannel, 0, 4)
	labels := map[string]int{}
	for _, service := range services {
		for _, prop := range service.properties {
			if !prop.Property.Writable() || prop.Property.Format != "bool" {
				continue
			}
			label := strings.TrimSpace(prop.ServiceLabel)
			if label == "" {
				label = humanize(prop.ServiceName)
			}
			propertyLabel := strings.TrimSpace(prop.Property.Description)
			if propertyLabel == "" {
				propertyLabel = humanize(spec.PropertyName(prop.Property))
			}
			if !containsNormalized(spec.PropertyName(prop.Property), []string{"on", "switch-status", "power"}) &&
				!strings.EqualFold(propertyLabel, label) {
				label = strings.TrimSpace(label + " " + propertyLabel)
			}
			labels[label]++
			displayLabel := label
			if labels[label] > 1 {
				displayLabel = fmt.Sprintf("%s %d", label, labels[label])
			}
			copy := prop
			channels = append(channels, ToggleChannel{
				ID:       fmt.Sprintf("toggle-%d-%d", prop.ServiceIID, prop.Property.IID),
				Label:    displayLabel,
				StateKey: fmt.Sprintf("toggle_%d_%d", prop.ServiceIID, prop.Property.IID),
				Ref:      &copy,
			})
		}
	}
	return channels
}

func toggleControlSpecs(mapping *DeviceMapping) []models.DeviceControlSpec {
	specs := make([]models.DeviceControlSpec, 0, len(mapping.ToggleChannels)+4)
	for _, item := range mapping.ToggleChannels {
		specs = append(specs, models.DeviceControlSpec{
			ID:       item.ID,
			Kind:     models.DeviceControlKindToggle,
			Label:    item.Label,
			StateKey: item.StateKey,
			OnCommand: &models.DeviceControlCommand{
				Action: "set_toggle",
				Params: map[string]any{"toggle_id": item.ID, "on": true},
			},
			OffCommand: &models.DeviceControlCommand{
				Action: "set_toggle",
				Params: map[string]any{"toggle_id": item.ID, "on": false},
			},
		})
	}
	if mapping.Power != nil && len(mapping.ToggleChannels) == 0 {
		specs = append(specs, toggleControlSpec("power", "Power", "power", "set_power"))
	}
	if mapping.PumpPower != nil {
		specs = append(specs, toggleControlSpec("pump", "Pump", "pump_power", "set_pump_power"))
	}
	if mapping.LightPower != nil {
		specs = append(specs, toggleControlSpec("light", "Light", "light_power", "set_light_power"))
	}
	if mapping.Mute != nil {
		specs = append(specs, toggleControlSpec("mute", "Mute", "mute", "set_mute"))
	}
	return specs
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

func humanize(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "-", " ")
	if value == "" {
		return "Switch"
	}
	parts := strings.Fields(strings.ToLower(value))
	for i, item := range parts {
		parts[i] = strings.ToUpper(item[:1]) + item[1:]
	}
	return strings.Join(parts, " ")
}
