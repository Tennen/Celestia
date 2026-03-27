package mapper

import (
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/cloud"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
)

func inferKind(raw cloud.DeviceRecord, instance spec.Instance) models.DeviceKind {
	deviceType := spec.DeviceType(raw.URN)
	model := strings.ToLower(raw.Model)
	switch {
	case containsAny(deviceType, model, "speaker", "wifispeaker"):
		return models.DeviceKindSpeaker
	case containsAny(deviceType, model, "fish", "bowl", "aquarium", "tank"):
		return models.DeviceKindAquarium
	case containsAny(deviceType, model, "light"):
		return models.DeviceKindLight
	case containsAny(deviceType, model, "switch", "outlet", "plug"):
		return models.DeviceKindSwitch
	case containsAny(deviceType, model, "air-conditioner", "air-condition-outlet", "thermostat", "fan"):
		return models.DeviceKindClimate
	case containsAny(deviceType, model, "sensor", "temperature-humidity", "motion"):
		return models.DeviceKindSensor
	}
	if hasNamedProperty(instance, "brightness") {
		return models.DeviceKindLight
	}
	if hasNamedProperty(instance, "target-temperature") {
		return models.DeviceKindClimate
	}
	if hasNamedProperty(instance, "relative-humidity") || hasNamedProperty(instance, "humidity") {
		return models.DeviceKindSensor
	}
	return ""
}

func hasNamedProperty(instance spec.Instance, name string) bool {
	for _, service := range instance.Services {
		for _, prop := range service.Properties {
			if spec.PropertyName(prop) == name {
				return true
			}
		}
	}
	return false
}

func assignCommon(mapping *DeviceMapping, services []serviceView) {
	mapping.Power = firstWritableProperty(services, matchProperty{
		names: []string{"on", "switch-status", "power"},
	})
}

func assignLight(mapping *DeviceMapping, services []serviceView) {
	mapping.Brightness = firstWritableProperty(services, matchProperty{names: []string{"brightness"}})
	mapping.ColorTemp = firstWritableProperty(services, matchProperty{names: []string{"color-temperature", "color-temp"}})
}

func assignSwitch(mapping *DeviceMapping, services []serviceView) {
	channels := discoverToggleChannels(services)
	if len(channels) == 0 {
		return
	}
	mapping.Power = channels[0].Ref
	mapping.ToggleChannels = channels
}

func assignSensor(mapping *DeviceMapping, services []serviceView) {
	mapping.Temperature = firstReadableProperty(services, matchProperty{names: []string{"temperature"}})
	mapping.Humidity = firstReadableProperty(services, matchProperty{names: []string{"relative-humidity", "humidity"}})
}

func assignClimate(mapping *DeviceMapping, services []serviceView) {
	mapping.TargetTemperature = firstWritableProperty(services, matchProperty{names: []string{"target-temperature"}})
	mapping.Mode = firstWritableProperty(services, matchProperty{names: []string{"mode", "mode-a"}})
	mapping.FanSpeed = firstWritableProperty(services, matchProperty{names: []string{"fan-level", "fan-speed"}})
	mapping.Temperature = firstReadableProperty(services, matchProperty{names: []string{"temperature"}})
}

func assignAquarium(mapping *DeviceMapping, services []serviceView) {
	mapping.PumpPower = firstWritableProperty(services, matchProperty{
		names:        []string{"pump", "water-pump", "filter", "circulation"},
		serviceHints: []string{"pump", "filter", "fish", "water"},
		boolOnly:     true,
	})
	mapping.PumpLevel = firstWritableProperty(services, matchProperty{
		names:        []string{"pump-flux", "mode", "mode-a", "fan-level", "fan-speed", "gear", "water-flow", "flow-level", "flow-rate", "pump-level"},
		serviceHints: []string{"pump", "filter", "circulation", "fish", "water"},
		excludeKinds: []string{"bool"},
	})
	mapping.LightPower = firstWritableProperty(services, matchProperty{
		serviceNames: []string{"light"},
		names:        []string{"on", "switch-status"},
		boolOnly:     true,
	})
	if mapping.LightPower == nil {
		mapping.LightPower = firstWritableProperty(services, matchProperty{
			names:        []string{"on", "switch-status"},
			serviceHints: []string{"light", "lamp"},
			boolOnly:     true,
		})
	}
	mapping.LightBrightness = firstWritableProperty(services, matchProperty{
		serviceNames: []string{"light"},
		names:        []string{"brightness"},
	})
	if mapping.LightBrightness == nil {
		mapping.LightBrightness = firstWritableProperty(services, matchProperty{
			names:        []string{"brightness"},
			serviceHints: []string{"light", "lamp"},
		})
	}
	mapping.LightMode = firstWritableProperty(services, matchProperty{
		serviceNames: []string{"light"},
		names:        []string{"mode", "mode-a"},
	})
	if mapping.LightMode == nil {
		mapping.LightMode = firstWritableProperty(services, matchProperty{
			names:        []string{"mode", "mode-a"},
			serviceHints: []string{"light", "lamp"},
		})
	}
	mapping.WaterTemperature = firstReadableProperty(services, matchProperty{
		names:        []string{"temperature", "water-temperature"},
		serviceHints: []string{"fish", "water"},
	})
	mapping.FilterLife = firstReadableProperty(services, matchProperty{
		names:        []string{"filter-used-time", "filter-life", "filter-level", "remaining-percentage", "percentage"},
		serviceHints: []string{"filter"},
	})
	if mapping.Power == nil {
		mapping.Power = firstWritableProperty(services, matchProperty{
			names:        []string{"on", "switch-status"},
			serviceHints: []string{"fish", "aquarium", "tank"},
			boolOnly:     true,
		})
	}
}

func assignSpeaker(mapping *DeviceMapping, services []serviceView) {
	mapping.Volume = firstWritableProperty(services, matchProperty{
		names:        []string{"volume", "speaker-volume", "volume-level"},
		serviceHints: []string{"speaker"},
	})
	mapping.Mute = firstWritableProperty(services, matchProperty{
		names:        []string{"mute"},
		serviceHints: []string{"speaker"},
		boolOnly:     true,
	})
	mapping.Text = firstWritableProperty(services, matchProperty{
		format:       "string",
		serviceHints: []string{"speaker", "intelligent-speaker", "message", "text"},
	})
	mapping.NotifyAction = firstAction(services, matchAction{
		serviceHints: []string{"speaker", "intelligent-speaker", "play-control", "message", "text"},
		requireInput: true,
		stringInput:  true,
	})
}
