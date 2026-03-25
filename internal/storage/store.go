package storage

import (
	"context"

	"github.com/chentianyu/celestia/internal/models"
)

type Store interface {
	EnsureSchema(context.Context) error

	UpsertPluginRecord(context.Context, models.PluginInstallRecord) error
	GetPluginRecord(context.Context, string) (models.PluginInstallRecord, bool, error)
	ListPluginRecords(context.Context) ([]models.PluginInstallRecord, error)
	DeletePluginRecord(context.Context, string) error

	UpsertDevice(context.Context, models.Device) error
	GetDevice(context.Context, string) (models.Device, bool, error)
	ListDevices(context.Context, DeviceFilter) ([]models.Device, error)
	DeleteDevicesByPlugin(context.Context, string) error

	UpsertDeviceState(context.Context, models.DeviceStateSnapshot) error
	GetDeviceState(context.Context, string) (models.DeviceStateSnapshot, bool, error)
	ListDeviceStates(context.Context, StateFilter) ([]models.DeviceStateSnapshot, error)

	AppendEvent(context.Context, models.Event) error
	ListEvents(context.Context, EventFilter) ([]models.Event, error)
	CountEvents(context.Context) (int, error)

	AppendAudit(context.Context, models.AuditRecord) error
	ListAudits(context.Context, AuditFilter) ([]models.AuditRecord, error)
	CountAudits(context.Context) (int, error)
}

type DeviceFilter struct {
	PluginID string
	Kind     string
	Query    string
}

type StateFilter struct {
	PluginID string
	DeviceID string
}

type EventFilter struct {
	PluginID string
	DeviceID string
	Limit    int
}

type AuditFilter struct {
	DeviceID string
	Limit    int
}

