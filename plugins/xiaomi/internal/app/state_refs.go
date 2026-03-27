package app

import (
	"context"

	"github.com/chentianyu/celestia/plugins/xiaomi/internal/mapper"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
)

func (r *accountRuntime) specInstance(ctx context.Context, urn string) (spec.Instance, error) {
	if item, ok := r.specs[urn]; ok {
		return item, nil
	}
	instance, err := r.client.SpecInstance(ctx, urn)
	if err != nil {
		return spec.Instance{}, err
	}
	r.specs[urn] = instance
	return instance, nil
}

type namedPropertyRef struct {
	name string
	ref  *mapper.PropertyRef
}

func propertyRefs(mapping *mapper.DeviceMapping) []namedPropertyRef {
	var refs []namedPropertyRef
	appendRef := func(name string, ref *mapper.PropertyRef) {
		if ref == nil || !stateReadable(ref) {
			return
		}
		refs = append(refs, namedPropertyRef{name: name, ref: ref})
	}
	appendRef("power", mapping.Power)
	appendRef("brightness", mapping.Brightness)
	appendRef("color_temp", mapping.ColorTemp)
	appendRef("target_temperature", mapping.TargetTemperature)
	appendRef("mode", mapping.Mode)
	appendRef("fan_speed", mapping.FanSpeed)
	appendRef("temperature", mapping.Temperature)
	appendRef("humidity", mapping.Humidity)
	appendRef("pump_power", mapping.PumpPower)
	appendRef("pump_level", mapping.PumpLevel)
	appendRef("light_power", mapping.LightPower)
	appendRef("light_brightness", mapping.LightBrightness)
	appendRef("light_mode", mapping.LightMode)
	appendRef("water_temperature", mapping.WaterTemperature)
	appendRef("filter_life", mapping.FilterLife)
	appendRef("volume", mapping.Volume)
	appendRef("mute", mapping.Mute)
	for _, toggle := range mapping.ToggleChannels {
		appendRef(toggle.StateKey, toggle.Ref)
	}
	return refs
}

func stateReadable(ref *mapper.PropertyRef) bool {
	if ref == nil {
		return false
	}
	return ref.Property.Readable() || ref.Property.Notifiable()
}
