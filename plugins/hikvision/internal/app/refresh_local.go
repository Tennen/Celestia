package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	lanclient "github.com/chentianyu/celestia/plugins/hikvision/internal/client"
)

func (p *Plugin) refreshAllLocal(ctx context.Context) error {
	entryIDs := p.entryIDs()
	if len(entryIDs) == 0 {
		return errors.New("no configured cameras")
	}
	var errs []string
	for _, entryID := range entryIDs {
		if err := p.refreshLocalEntry(ctx, entryID); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", entryID, err))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (p *Plugin) refreshLocalEntry(ctx context.Context, entryID string) error {
	p.mu.RLock()
	runtime := p.entries[entryID]
	if runtime == nil {
		p.mu.RUnlock()
		return errors.New("entry not found")
	}
	cfg := runtime.Config
	cam := runtime.Client
	connected := runtime.Connected
	p.mu.RUnlock()

	if cam == nil {
		return errors.New("local hikvision client is not available")
	}
	if !connected {
		if _, err := cam.Connect(ctx, cfg.LocalCameraConfig()); err != nil {
			state := buildLocalState(cfg, lanclient.CameraStatus{Connected: false}, err.Error())
			p.applyState(entryID, state, true)
			return err
		}
	}
	status, err := cam.Status(ctx)
	if err != nil {
		state := buildLocalState(cfg, lanclient.CameraStatus{Connected: false}, err.Error())
		p.applyState(entryID, state, true)
		return err
	}
	device := buildDevice(RuntimeModeLAN, cfg, nil, "")
	p.updateRuntimeDevice(entryID, device, nil, "")
	p.applyState(entryID, buildLocalState(cfg, status, ""), false)
	return nil
}
