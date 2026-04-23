package runtime

import (
	"context"

	"github.com/chentianyu/celestia/internal/core/agent"
	"github.com/chentianyu/celestia/internal/core/audit"
	"github.com/chentianyu/celestia/internal/core/automation"
	"github.com/chentianyu/celestia/internal/core/capability"
	"github.com/chentianyu/celestia/internal/core/control"
	"github.com/chentianyu/celestia/internal/core/eventbus"
	oauthsvc "github.com/chentianyu/celestia/internal/core/oauth"
	"github.com/chentianyu/celestia/internal/core/pluginmgr"
	"github.com/chentianyu/celestia/internal/core/policy"
	"github.com/chentianyu/celestia/internal/core/project/input"
	"github.com/chentianyu/celestia/internal/core/project/slash"
	"github.com/chentianyu/celestia/internal/core/project/touchpoint"
	"github.com/chentianyu/celestia/internal/core/project/voice"
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
	Home       *control.HomeService
	Policy     *policy.Service
	OAuth      *oauthsvc.Service
	PluginMgr  *pluginmgr.Manager
	Vision     *vision.Service
	Agent      *agent.Service
	Slash      *slash.Service
	Input      *input.Service
	Touchpoint *touchpoint.Service
}

func New(store storage.Store) *Runtime {
	bus := eventbus.New()
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	policySvc := policy.New()
	auditSvc := audit.New(store)
	pluginMgr := pluginmgr.New(store, registrySvc, stateSvc, bus)
	visionSvc := vision.New(store, registrySvc, stateSvc, bus)
	agentSvc := agent.New(store, bus)
	controlSvc := control.New()
	homeSvc := control.NewHomeService(store, registrySvc, stateSvc, controlSvc, policySvc, auditSvc, pluginMgr, visionSvc)
	slashSvc := slash.New(homeSvc, agentSvc)
	inputSvc := input.New(agentSvc, slashSvc)
	voiceSvc := voice.New(agentSvc)
	touchpointSvc := touchpoint.New(agentSvc, agentSvc)
	touchpointSvc.SetInputRunner(inputSvc)
	touchpointSvc.SetVoiceProvider(voiceSvc)
	automationSvc := automation.New(store, bus, registrySvc, stateSvc, policySvc, auditSvc, pluginMgr)
	automationSvc.SetAgentRuntime(inputSvc)
	automationSvc.SetWeComRuntime(touchpointSvc)
	return &Runtime{
		Store:      store,
		EventBus:   bus,
		Registry:   registrySvc,
		State:      stateSvc,
		Audit:      auditSvc,
		Automation: automationSvc,
		Capability: capability.New(automationSvc, visionSvc),
		Controls:   controlSvc,
		Home:       homeSvc,
		Policy:     policySvc,
		OAuth:      oauthsvc.New(store),
		PluginMgr:  pluginMgr,
		Vision:     visionSvc,
		Agent:      agentSvc,
		Slash:      slashSvc,
		Input:      inputSvc,
		Touchpoint: touchpointSvc,
	}
}

func (r *Runtime) Reconcile(ctx context.Context) error {
	if r.Vision != nil {
		if err := r.Vision.Init(ctx); err != nil {
			return err
		}
	}
	if r.Agent != nil {
		if err := r.Agent.Init(ctx); err != nil {
			return err
		}
	}
	if r.Touchpoint != nil {
		if err := r.Touchpoint.Init(ctx); err != nil {
			return err
		}
	}
	return r.PluginMgr.Reconcile(ctx)
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	if r.Automation != nil {
		r.Automation.Close()
	}
	if r.Vision != nil {
		r.Vision.Close()
	}
	if r.Touchpoint != nil {
		r.Touchpoint.Close()
	}
	if r.Agent != nil {
		r.Agent.Close()
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
