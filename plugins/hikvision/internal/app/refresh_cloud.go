package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ezvizcloud "github.com/chentianyu/celestia/plugins/hikvision/internal/cloud"
)

func (p *Plugin) refreshAllCloud(ctx context.Context) error {
	entryIDs := p.entryIDs()
	if len(entryIDs) == 0 {
		return errors.New("no configured cameras")
	}

	var (
		deviceIndex map[string]ezvizcloud.DeviceInfo
		cloudErr    error
	)
	if p.cloud != nil {
		deviceIndex, cloudErr = p.cloud.RefreshDevices(ctx)
		if cloudErr == nil {
			_ = p.syncCloudSessionConfig()
		}
	}

	var errs []string
	for _, entryID := range entryIDs {
		if err := p.refreshCloudEntry(entryID, deviceIndex, cloudErr); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", entryID, err))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (p *Plugin) refreshCloudEntry(entryID string, devices map[string]ezvizcloud.DeviceInfo, cloudErr error) error {
	p.mu.RLock()
	runtime := p.entries[entryID]
	cfg := p.config
	p.mu.RUnlock()
	if runtime == nil {
		return errors.New("entry not found")
	}

	var (
		info         *ezvizcloud.DeviceInfo
		entryErr     error
		controlBlock string
	)
	if cloudErr != nil {
		entryErr = cloudErr
	} else if runtime.Config.DeviceSerial != "" {
		if item, ok := devices[runtime.Config.DeviceSerial]; ok {
			copied := item
			info = &copied
		} else if cfg.Cloud.HasAuth() {
			entryErr = fmt.Errorf("camera %s was not returned by the Ezviz cloud account", runtime.Config.DeviceSerial)
		}
	}

	controlBlock = p.controlBlockedReason(cfg, runtime.Config, info, "")
	if entryErr != nil && controlBlock == "" {
		controlBlock = entryErr.Error()
	}
	if cloudErr != nil && cfg.Cloud.HasAuth() {
		controlBlock = cloudErr.Error()
	}

	stateError := ""
	if entryErr != nil && cfg.Cloud.HasAuth() {
		stateError = entryErr.Error()
	}
	device := buildDevice(RuntimeModeCloud, runtime.Config, info, controlBlock)
	state := buildCloudState(runtime.Config, info, cfg.Cloud.HasAuth(), controlBlock, stateError)
	p.updateRuntimeDevice(entryID, device, info, controlBlock)
	p.applyState(entryID, state, stateError != "")
	return entryErr
}
