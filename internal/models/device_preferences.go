package models

import "time"

type DevicePreference struct {
	DeviceID  string    `json:"device_id"`
	Alias     string    `json:"alias,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}
