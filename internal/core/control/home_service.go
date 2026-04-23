package control

import (
	"context"
	"errors"
	"strings"

	"github.com/chentianyu/celestia/internal/core/audit"
	"github.com/chentianyu/celestia/internal/core/policy"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

type DeviceEnricher interface {
	EnrichDevice(models.Device) models.Device
}

type CommandExecutor interface {
	ExecuteCommand(context.Context, models.Device, models.CommandRequest) (models.CommandResponse, error)
}

type HomeService struct {
	store    storage.Store
	registry *registry.Service
	state    *state.Service
	controls *Service
	policy   *policy.Service
	audit    *audit.Service
	executor CommandExecutor
	enricher DeviceEnricher
}

func NewHomeService(
	store storage.Store,
	registrySvc *registry.Service,
	stateSvc *state.Service,
	controls *Service,
	policySvc *policy.Service,
	auditSvc *audit.Service,
	executor CommandExecutor,
	enricher DeviceEnricher,
) *HomeService {
	return &HomeService{
		store:    store,
		registry: registrySvc,
		state:    stateSvc,
		controls: controls,
		policy:   policySvc,
		audit:    auditSvc,
		executor: executor,
		enricher: enricher,
	}
}

func (s *HomeService) ListViews(ctx context.Context, filter HomeFilter) ([]models.DeviceView, error) {
	devices, views, err := s.loadDeviceViews(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]models.DeviceView, 0, len(devices))
	for _, device := range devices {
		view, ok := views[device.ID]
		if ok {
			out = append(out, view)
		}
	}
	return out, nil
}

func (s *HomeService) GetView(ctx context.Context, deviceID string) (models.DeviceView, error) {
	_, view, err := s.loadDeviceByID(ctx, deviceID)
	if err != nil {
		return models.DeviceView{}, err
	}
	return view, nil
}

func (s *HomeService) ListCatalog(ctx context.Context, filter HomeFilter) ([]HomeDevice, error) {
	catalogs, err := s.loadDeviceCatalogs(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]HomeDevice, 0, len(catalogs))
	for _, item := range catalogs {
		out = append(out, item.device)
	}
	return out, nil
}

func (s *HomeService) loadDeviceCatalogs(ctx context.Context, filter HomeFilter) ([]homeDeviceCatalog, error) {
	devices, views, err := s.loadDeviceViews(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]homeDeviceCatalog, 0, len(devices))
	for _, device := range devices {
		view, ok := views[device.ID]
		if !ok {
			continue
		}
		out = append(out, buildHomeDeviceCatalog(device, view, s.controls.Specs(device)))
	}
	return out, nil
}

func (s *HomeService) loadDeviceByID(ctx context.Context, deviceID string) (models.Device, models.DeviceView, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return models.Device{}, models.DeviceView{}, &ValidationError{Err: errors.New("device_id is required")}
	}
	device, ok, err := s.registry.Get(ctx, deviceID)
	if err != nil {
		return models.Device{}, models.DeviceView{}, err
	}
	if !ok {
		return models.Device{}, models.DeviceView{}, &NotFoundError{
			Field: "device",
			Value: deviceID,
			Err:   errors.New("device not found"),
		}
	}
	stateSnapshot, _, err := s.state.Get(ctx, device.ID)
	if err != nil {
		return models.Device{}, models.DeviceView{}, err
	}
	view, err := s.deviceView(ctx, device, stateSnapshot)
	if err != nil {
		return models.Device{}, models.DeviceView{}, err
	}
	return device, view, nil
}

func (s *HomeService) loadDeviceViews(ctx context.Context, filter HomeFilter) ([]models.Device, map[string]models.DeviceView, error) {
	devices, err := s.registry.List(ctx, storage.DeviceFilter{
		PluginID: filter.PluginID,
		Kind:     filter.Kind,
		Query:    filter.Query,
	})
	if err != nil {
		return nil, nil, err
	}
	states, err := s.state.List(ctx, storage.StateFilter{})
	if err != nil {
		return nil, nil, err
	}
	stateMap := make(map[string]models.DeviceStateSnapshot, len(states))
	for _, item := range states {
		stateMap[item.DeviceID] = item
	}
	views := make(map[string]models.DeviceView, len(devices))
	for _, device := range devices {
		view, err := s.deviceView(ctx, device, stateMap[device.ID])
		if err != nil {
			return nil, nil, err
		}
		views[device.ID] = view
	}
	return devices, views, nil
}

func (s *HomeService) deviceView(ctx context.Context, device models.Device, stateSnapshot models.DeviceStateSnapshot) (models.DeviceView, error) {
	if s.enricher != nil {
		device = s.enricher.EnrichDevice(device)
	}
	view := s.controls.BuildView(device, stateSnapshot)
	devicePref, _, err := s.store.GetDevicePreference(ctx, device.ID)
	if err != nil {
		return models.DeviceView{}, err
	}
	view.Device = applyHomeDevicePreference(view.Device, devicePref)
	prefs, err := s.store.ListDeviceControlPreferences(ctx, device.ID)
	if err != nil {
		return models.DeviceView{}, err
	}
	return s.controls.ApplyPreferences(view, prefs), nil
}

func applyHomeDevicePreference(device models.Device, pref models.DevicePreference) models.Device {
	alias := strings.TrimSpace(pref.Alias)
	if alias == "" {
		return device
	}
	device.DefaultName = device.Name
	device.Alias = alias
	device.Name = alias
	return device
}
