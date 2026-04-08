package runtime

import (
	"context"

	"github.com/chentianyu/celestia/internal/core/audit"
	"github.com/chentianyu/celestia/internal/core/automation"
	"github.com/chentianyu/celestia/internal/core/capability"
	"github.com/chentianyu/celestia/internal/core/control"
	"github.com/chentianyu/celestia/internal/core/eventbus"
	oauthsvc "github.com/chentianyu/celestia/internal/core/oauth"
	"github.com/chentianyu/celestia/internal/core/pluginmgr"
	"github.com/chentianyu/celestia/internal/core/policy"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/core/vision"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

type Runtime struct {
	Store      storage.Store
	EventBus   *eventbus.Bus
	Registry   *registry.Service
	State      *state.Service
	Audit      *audit.Service
	Automation *automation.Service
	Capability *capability.Service
	Controls   *control.Service
	Policy     *policy.Service
	OAuth      *oauthsvc.Service
	PluginMgr  *pluginmgr.Manager
	Vision     *vision.Service
}

func New(store storage.Store) *Runtime {
	bus := eventbus.New()
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	policySvc := policy.New()
	auditSvc := audit.New(store)
	pluginMgr := pluginmgr.New(store, registrySvc, stateSvc, bus)
	visionSvc := vision.New(store, registrySvc, stateSvc, bus)
	automationSvc := automation.New(store, bus, registrySvc, stateSvc, policySvc, auditSvc, pluginMgr)
	return &Runtime{
		Store:      store,
		EventBus:   bus,
		Registry:   registrySvc,
		State:      stateSvc,
		Audit:      auditSvc,
		Automation: automationSvc,
		Capability: capability.New(automationSvc, visionSvc),
		Controls:   control.New(),
		Policy:     policySvc,
		OAuth:      oauthsvc.New(store),
		PluginMgr:  pluginMgr,
		Vision:     visionSvc,
	}
}

func (r *Runtime) Reconcile(ctx context.Context) error {
	return r.PluginMgr.Reconcile(ctx)
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	if r.Automation != nil {
		r.Automation.Close()
	}
	return r.PluginMgr.Shutdown(ctx)
}

func (r *Runtime) Dashboard(ctx context.Context) (models.DashboardSummary, error) {
	plugins, err := r.Store.ListPluginRecords(ctx)
	if err != nil {
		return models.DashboardSummary{}, err
	}
	devices, err := r.Store.ListDevices(ctx, storage.DeviceFilter{})
	if err != nil {
		return models.DashboardSummary{}, err
	}
	events, err := r.Store.CountEvents(ctx)
	if err != nil {
		return models.DashboardSummary{}, err
	}
	audits, err := r.Store.CountAudits(ctx)
	if err != nil {
		return models.DashboardSummary{}, err
	}
	summary := models.DashboardSummary{
		Plugins: len(plugins),
		Devices: len(devices),
		Events:  events,
		Audits:  audits,
	}
	for _, plugin := range plugins {
		if plugin.Status == models.PluginStatusEnabled {
			summary.EnabledPlugins++
		}
	}
	for _, device := range devices {
		if device.Online {
			summary.OnlineDevices++
		}
	}
	return summary, nil
}
