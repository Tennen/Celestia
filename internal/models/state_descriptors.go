package models

type DeviceStateDescriptor struct {
	Label   string                `json:"label,omitempty"`
	Options []DeviceControlOption `json:"options,omitempty"`
	Hidden  bool                  `json:"hidden,omitempty"`
}
