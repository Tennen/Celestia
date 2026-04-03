package app

import (
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/haier/internal/client"
)

func buildStateDescriptors(model client.DigitalModel) map[string]models.DeviceStateDescriptor {
	descriptors := make(map[string]models.DeviceStateDescriptor, len(model.Attributes)+2)
	for _, attribute := range model.Attributes {
		key := strings.TrimSpace(attribute.Name)
		if key == "" {
			continue
		}
		options := make([]models.DeviceControlOption, 0, len(attribute.Options))
		seen := map[string]bool{}
		for _, option := range attribute.Options {
			value := strings.TrimSpace(option.Value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			options = append(options, models.DeviceControlOption{
				Value: value,
				Label: firstNonEmpty(strings.TrimSpace(option.Label), value),
			})
		}
		label := strings.TrimSpace(attribute.Description)
		if label == "" && len(options) == 0 {
			continue
		}
		descriptors[key] = models.DeviceStateDescriptor{
			Label:   firstNonEmpty(label, key),
			Options: options,
		}
	}
	aliasStateDescriptor(descriptors, "prCode", "program")
	aliasStateDescriptor(descriptors, "prPhase", "phase")
	hideStateDescriptor(descriptors, "prCode")
	hideStateDescriptor(descriptors, "prPhase")
	return descriptors
}

func aliasStateDescriptor(descriptors map[string]models.DeviceStateDescriptor, sourceKey, targetKey string) {
	descriptor, ok := descriptors[sourceKey]
	if !ok {
		return
	}
	descriptor.Hidden = false
	descriptors[targetKey] = descriptor
}

func hideStateDescriptor(descriptors map[string]models.DeviceStateDescriptor, key string) {
	descriptor, ok := descriptors[key]
	if !ok {
		return
	}
	descriptor.Hidden = true
	descriptors[key] = descriptor
}
