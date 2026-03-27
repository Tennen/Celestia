package mapper

import (
	"fmt"
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
