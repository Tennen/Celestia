package mapper

import (
	"fmt"
	"slices"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/cloud"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/spec"
)

type PropertyRef struct {
	ServiceName  string
	ServiceLabel string
	ServiceIID   int
	Property     spec.Property
}

type ActionRef struct {
	ServiceName string
	ServiceIID  int
	Action      spec.Action
	Inputs      []spec.Property
}

type DeviceMapping struct {
	Kind              models.DeviceKind
	ToggleChannels    []ToggleChannel
	Power             *PropertyRef
	Brightness        *PropertyRef
	ColorTemp         *PropertyRef
	TargetTemperature *PropertyRef
	Mode              *PropertyRef
	FanSpeed          *PropertyRef
	Temperature       *PropertyRef
	Humidity          *PropertyRef
	PumpPower         *PropertyRef
	LightPower        *PropertyRef
	LightBrightness   *PropertyRef
	LightMode         *PropertyRef
	WaterTemperature  *PropertyRef
	FilterLife        *PropertyRef
	Volume            *PropertyRef
	Mute              *PropertyRef
	Text              *PropertyRef
	NotifyAction      *ActionRef
}

type ToggleChannel struct {
	ID          string
	Label       string
	Description string
	StateKey    string
	Ref         *PropertyRef
}

func Build(raw cloud.DeviceRecord, instance spec.Instance, accountName string) (*models.Device, *DeviceMapping, error) {
	kind := inferKind(raw, instance)
	if kind == "" {
		return nil, nil, fmt.Errorf("unsupported xiaomi device kind for model %s (%s)", raw.Model, raw.URN)
	}
	mapping := &DeviceMapping{Kind: kind}
	services := make([]serviceView, 0, len(instance.Services))
	for _, service := range instance.Services {
		sv := serviceView{
			name:       spec.ServiceName(service),
			label:      strings.TrimSpace(service.Description),
			serviceIID: service.IID,
		}
		for _, prop := range service.Properties {
			sv.properties = append(sv.properties, PropertyRef{
				ServiceName:  sv.name,
				ServiceLabel: sv.label,
				ServiceIID:   service.IID,
				Property:     prop,
			})
		}
		propertyByIID := map[int]spec.Property{}
		for _, prop := range service.Properties {
			propertyByIID[prop.IID] = prop
		}
		for _, action := range service.Actions {
			ref := ActionRef{
				ServiceName: sv.name,
				ServiceIID:  service.IID,
				Action:      action,
			}
			for _, iid := range action.In {
				if prop, ok := propertyByIID[iid]; ok {
					ref.Inputs = append(ref.Inputs, prop)
				}
			}
			sv.actions = append(sv.actions, ref)
		}
		services = append(services, sv)
	}

	assignCommon(mapping, services)
	switch kind {
	case models.DeviceKindLight:
		assignLight(mapping, services)
	case models.DeviceKindSwitch:
		assignSwitch(mapping, services)
	case models.DeviceKindSensor:
		assignSensor(mapping, services)
	case models.DeviceKindClimate:
		assignClimate(mapping, services)
	case models.DeviceKindAquarium:
		assignAquarium(mapping, services)
	case models.DeviceKindSpeaker:
		assignSpeaker(mapping, services)
	}

	capabilities := mapping.capabilities()
	if len(capabilities) == 0 {
		return nil, nil, fmt.Errorf("no supported Xiaomi capabilities detected for %s", raw.Model)
	}

	deviceType := spec.DeviceType(raw.URN)
	device := &models.Device{
		ID:             fmt.Sprintf("xiaomi:%s:%s", raw.Region, raw.DID),
		PluginID:       "xiaomi",
		VendorDeviceID: raw.DID,
		Kind:           kind,
		Name:           raw.Name,
		Room:           raw.RoomName,
		Online:         raw.Online,
		Capabilities:   capabilities,
		Metadata: map[string]any{
			"account":    accountName,
			"region":     raw.Region,
			"home_id":    raw.HomeID,
			"home_name":  raw.HomeName,
			"room_id":    raw.RoomID,
			"room_name":  raw.RoomName,
			"miot_type":  deviceType,
			"miot_urn":   raw.URN,
			"model":      raw.Model,
			"voice_ctrl": raw.VoiceCtrl,
			"group_id":   raw.GroupID,
		},
	}
	if len(mapping.ToggleChannels) > 0 {
		device.Metadata["toggle_refs"] = toggleMetadata(mapping.ToggleChannels)
	}
	return device, mapping, nil
}

type serviceView struct {
	name       string
	label      string
	serviceIID int
	properties []PropertyRef
	actions    []ActionRef
}

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
	if len(channels) > 1 {
		mapping.ToggleChannels = channels
	}
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
	mapping.LightPower = firstWritableProperty(services, matchProperty{
		names:        []string{"on", "switch-status"},
		serviceHints: []string{"light", "lamp"},
		boolOnly:     true,
	})
	mapping.LightBrightness = firstWritableProperty(services, matchProperty{
		names:        []string{"brightness"},
		serviceHints: []string{"light", "lamp"},
	})
	mapping.LightMode = firstWritableProperty(services, matchProperty{
		names:        []string{"mode", "mode-a"},
		serviceHints: []string{"light", "lamp"},
	})
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

type matchProperty struct {
	names        []string
	serviceHints []string
	format       string
	boolOnly     bool
}

type matchAction struct {
	serviceHints []string
	requireInput bool
	stringInput  bool
}

func firstWritableProperty(services []serviceView, match matchProperty) *PropertyRef {
	return firstProperty(services, match, func(prop spec.Property) bool { return prop.Writable() })
}

func firstReadableProperty(services []serviceView, match matchProperty) *PropertyRef {
	return firstProperty(services, match, func(prop spec.Property) bool { return prop.Readable() || prop.Notifiable() })
}

func firstProperty(services []serviceView, match matchProperty, access func(spec.Property) bool) *PropertyRef {
	for _, service := range services {
		for _, prop := range service.properties {
			if !access(prop.Property) {
				continue
			}
			if matchPropertyRef(prop, match) {
				copy := prop
				return &copy
			}
		}
	}
	return nil
}

func firstAction(services []serviceView, match matchAction) *ActionRef {
	for _, service := range services {
		for _, action := range service.actions {
			if matchActionRef(action, match) {
				copy := action
				return &copy
			}
		}
	}
	return nil
}

func matchPropertyRef(prop PropertyRef, match matchProperty) bool {
	if match.boolOnly && prop.Property.Format != "bool" {
		return false
	}
	if match.format != "" && prop.Property.Format != match.format {
		return false
	}
	name := spec.PropertyName(prop.Property)
	if len(match.names) > 0 && !containsNormalized(name, match.names) {
		return false
	}
	if len(match.serviceHints) > 0 && !containsAny(prop.ServiceName, strings.ToLower(prop.Property.Description), match.serviceHints...) {
		return false
	}
	return true
}

func discoverToggleChannels(services []serviceView) []ToggleChannel {
	channels := make([]ToggleChannel, 0, 4)
	labels := map[string]int{}
	for _, service := range services {
		for _, prop := range service.properties {
			if !prop.Property.Writable() || prop.Property.Format != "bool" {
				continue
			}
			if !matchPropertyRef(prop, matchProperty{names: []string{"on", "switch-status", "power"}}) {
				continue
			}
			label := strings.TrimSpace(prop.ServiceLabel)
			if label == "" {
				label = humanize(prop.ServiceName)
			}
			labels[label]++
			displayLabel := label
			if labels[label] > 1 {
				displayLabel = fmt.Sprintf("%s %d", label, labels[label])
			}
			index := len(channels) + 1
			copy := prop
			channels = append(channels, ToggleChannel{
				ID:          fmt.Sprintf("switch-%d", index),
				Label:       displayLabel,
				Description: fmt.Sprintf("Control %s.", strings.ToLower(displayLabel)),
				StateKey:    fmt.Sprintf("switch_%d", index),
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

func matchActionRef(action ActionRef, match matchAction) bool {
	if match.requireInput && len(action.Inputs) == 0 {
		return false
	}
	if match.stringInput {
		hasString := false
		for _, input := range action.Inputs {
			if input.Format == "string" {
				hasString = true
				break
			}
		}
		if !hasString {
			return false
		}
	}
	if len(match.serviceHints) == 0 {
		return true
	}
	actionName := spec.ActionName(action.Action)
	return containsAny(action.ServiceName, actionName, match.serviceHints...)
}

func containsAny(haystackA, haystackB string, needles ...string) bool {
	return containsAnyImpl([]string{haystackA, haystackB}, needles)
}

func containsAnyImpl(haystacks []string, needles []string) bool {
	for _, haystack := range haystacks {
		haystack = strings.ToLower(haystack)
		for _, needle := range needles {
			needle = strings.ToLower(needle)
			if strings.Contains(haystack, needle) {
				return true
			}
		}
	}
	return false
}

func containsNormalized(value string, names []string) bool {
	value = strings.ToLower(value)
	for _, name := range names {
		if value == strings.ToLower(name) {
			return true
		}
	}
	return false
}

func (m DeviceMapping) capabilities() []string {
	var out []string
	appendOnce := func(value string) {
		if value == "" || slices.Contains(out, value) {
			return
		}
		out = append(out, value)
	}
	if m.Power != nil {
		appendOnce("power")
	}
	if m.Brightness != nil {
		appendOnce("brightness")
	}
	if m.ColorTemp != nil {
		appendOnce("color_temp")
	}
	if m.TargetTemperature != nil {
		appendOnce("target_temperature")
	}
	if m.Mode != nil {
		appendOnce("mode")
	}
	if m.FanSpeed != nil {
		appendOnce("fan_speed")
	}
	if m.Temperature != nil {
		appendOnce("temperature")
	}
	if m.Humidity != nil {
		appendOnce("humidity")
	}
	if m.PumpPower != nil {
		appendOnce("pump_power")
	}
	if m.LightPower != nil {
		appendOnce("light_power")
	}
	if m.LightBrightness != nil {
		appendOnce("light_brightness")
	}
	if m.LightMode != nil {
		appendOnce("light_mode")
	}
	if m.WaterTemperature != nil {
		appendOnce("water_temperature")
	}
	if m.FilterLife != nil {
		appendOnce("filter_life")
	}
	if m.Volume != nil {
		appendOnce("volume")
	}
	if m.Mute != nil {
		appendOnce("mute")
	}
	if m.NotifyAction != nil || m.Text != nil {
		appendOnce("voice_push")
	}
	if len(m.ToggleChannels) > 1 {
		appendOnce("toggle_channels")
	}
	return out
}
