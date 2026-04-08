package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chentianyu/celestia/internal/coreapi"
)

func (p *Plugin) syncCloudSessionConfig() error {
	if p.cloud == nil {
		return nil
	}
	sessionCfg, ok := p.cloud.CurrentConfig()
	if !ok {
		return nil
	}

	var (
		snapshot Config
		changed  bool
	)
	p.mu.Lock()
	if p.config.Cloud.SessionID != sessionCfg.SessionID {
		p.config.Cloud.SessionID = sessionCfg.SessionID
		changed = true
	}
	if p.config.Cloud.RefreshSessionID != sessionCfg.RefreshSessionID {
		p.config.Cloud.RefreshSessionID = sessionCfg.RefreshSessionID
		changed = true
	}
	if p.config.Cloud.APIURL != sessionCfg.APIURL {
		p.config.Cloud.APIURL = sessionCfg.APIURL
		changed = true
	}
	if p.config.Cloud.UserName != sessionCfg.UserName {
		p.config.Cloud.UserName = sessionCfg.UserName
		changed = true
	}
	if changed {
		snapshot = p.config
	}
	p.mu.Unlock()

	if !changed {
		return nil
	}
	payload, err := configMap(snapshot)
	if err != nil {
		return err
	}
	persistCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := coreapi.PersistPluginConfig(persistCtx, pluginID, payload); err != nil {
		return fmt.Errorf("persist hikvision cloud session config: %w", err)
	}
	return nil
}

func configMap(cfg Config) (map[string]any, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
