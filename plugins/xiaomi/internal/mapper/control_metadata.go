package mapper

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
)

func controlSpecs(mapping *DeviceMapping) []models.DeviceControlSpec {
	specs := toggleControlSpecs(mapping)
	specs = append(specs, valueControlSpecs(mapping)...)
	return specs
}

func valueControlSpecs(mapping *DeviceMapping) []models.DeviceControlSpec {
	var out []models.DeviceControlSpec
	appendControl := func(id, label, stateKey, action string, ref *PropertyRef) {
		if item, ok := propertyControlSpec(id, label, stateKey, action, ref); ok {
			out = append(out, item)
		}
	}
	appendControl("pump-level", "Pump Level", "pump_level", "set_pump_level", mapping.PumpLevel)
	appendControl("light-brightness", "Light Brightness", "light_brightness", "set_light_brightness", mapping.LightBrightness)
	appendControl("light-mode", "Light Mode", "light_mode", "set_light_mode", mapping.LightMode)
	return out
}

func propertyControlSpec(id, label, stateKey, action string, ref *PropertyRef) (models.DeviceControlSpec, bool) {
	if ref == nil {
		return models.DeviceControlSpec{}, false
	}
	spec := models.DeviceControlSpec{
		ID:       id,
		Label:    label,
		StateKey: stateKey,
		Command: &models.DeviceControlCommand{
			Action:     action,
			ValueParam: "value",
		},
	}
	if unit := strings.TrimSpace(ref.Property.Unit); unit != "" {
		spec.Unit = unit
	}
	if len(ref.Property.ValueList) > 0 {
		spec.Kind = models.DeviceControlKindSelect
		spec.Options = propertyOptions(ref.Property)
		return spec, true
	}
	if ref.Property.Format == "string" {
		return models.DeviceControlSpec{}, false
	}
	spec.Kind = models.DeviceControlKindNumber
	if min, max, step, ok := ref.Property.RangeBounds(); ok {
		spec.Min = &min
		spec.Max = &max
		spec.Step = &step
	}
	return spec, true
}

func propertyOptions(prop spec.Property) []models.DeviceControlOption {
	out := make([]models.DeviceControlOption, 0, len(prop.ValueList))
	useNumericValues := prop.HasDuplicateEnumDescriptions()
	for _, item := range prop.ValueList {
		label := strings.TrimSpace(item.Description)
		if label == "" {
			label = strconv.Itoa(item.Value)
		}
		value := normalizeControlValue(label)
		if useNumericValues {
			value = strconv.Itoa(item.Value)
			label = fmt.Sprintf("%s (%d)", label, item.Value)
		} else if value == "" {
			value = strconv.Itoa(item.Value)
		}
		out = append(out, models.DeviceControlOption{Value: value, Label: label})
	}
	return out
}

func normalizeControlValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}
