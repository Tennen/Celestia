package mapper

import (
	"fmt"
	"strings"

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
				ID:          fmt.Sprintf("toggle-%d-%d", prop.ServiceIID, prop.Property.IID),
				Label:       displayLabel,
				Description: fmt.Sprintf("Control %s.", strings.ToLower(displayLabel)),
				StateKey:    fmt.Sprintf("toggle_%d_%d", prop.ServiceIID, prop.Property.IID),
				Ref:         &copy,
			})
		}
	}
	return channels
}

func toggleMetadata(channels []ToggleChannel) []map[string]any {
	out := make([]map[string]any, 0, len(channels))
	for _, item := range channels {
		out = append(out, map[string]any{
			"id":          item.ID,
			"label":       item.Label,
			"description": item.Description,
			"state_key":   item.StateKey,
		})
	}
	return out
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
