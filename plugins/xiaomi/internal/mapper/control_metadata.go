package mapper

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
)

func valueControlMetadata(mapping *DeviceMapping) []map[string]any {
	var out []map[string]any
	appendControl := func(id, label, description, stateKey, action string, ref *PropertyRef) {
		if item := propertyControlMetadata(id, label, description, stateKey, action, ref); item != nil {
			out = append(out, item)
		}
	}
	appendControl("pump-level", "Pump Level", "Adjust the aquarium pump level.", "pump_level", "set_pump_level", mapping.PumpLevel)
	appendControl("light-brightness", "Light Brightness", "Adjust the aquarium light brightness.", "light_brightness", "set_light_brightness", mapping.LightBrightness)
	appendControl("light-mode", "Light Mode", "Set the aquarium light mode.", "light_mode", "set_light_mode", mapping.LightMode)
	return out
}

func propertyControlMetadata(id, label, description, stateKey, action string, ref *PropertyRef) map[string]any {
	if ref == nil {
		return nil
	}
	meta := map[string]any{
		"id":          id,
		"label":       label,
		"description": description,
		"state_key":   stateKey,
		"action":      action,
		"value_param": "value",
	}
	if unit := strings.TrimSpace(ref.Property.Unit); unit != "" {
		meta["unit"] = unit
	}
	if len(ref.Property.ValueList) > 0 {
		meta["kind"] = "select"
		meta["options"] = propertyOptions(ref.Property)
		return meta
	}
	if ref.Property.Format == "string" {
		return nil
	}
	meta["kind"] = "number"
	if min, max, step, ok := ref.Property.RangeBounds(); ok {
		meta["min"] = min
		meta["max"] = max
		meta["step"] = step
	}
	return meta
}

func propertyOptions(prop spec.Property) []map[string]any {
	out := make([]map[string]any, 0, len(prop.ValueList))
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
		out = append(out, map[string]any{
			"value": value,
			"label": label,
		})
	}
	return out
}

func normalizeControlValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}
