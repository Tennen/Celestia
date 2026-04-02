package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

// Sync fetches all device snapshots for this account.
func (c *Client) Sync(ctx context.Context) ([]DeviceSnapshot, error) {
	if err := c.EnsureSession(ctx); err != nil {
		return nil, err
	}
	families, err := c.loadFamilies(ctx)
	if err != nil {
		return nil, err
	}
	snapshots := make([]DeviceSnapshot, 0, len(families)*4)
	var firstErr error
	for _, family := range families {
		for _, item := range family.DeviceList {
			info, ok := BuildDeviceInfo(item)
			if !ok {
				continue
			}
			detail, err := c.LoadDeviceDetail(ctx, info)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			records := map[string]any(nil)
			if kind, supported := KindFromPetkitType(info.DeviceType); supported && kind == models.DeviceKindPetFeeder {
				records, err = c.LoadFeederRecords(ctx, info)
				if err != nil && firstErr == nil {
					firstErr = err
				}
			}
			snapshot, err := c.BuildSnapshot(info, detail, records)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			snapshot.AccountName = AccountKey(c.Cfg)
			snapshot.Client = c
			snapshots = append(snapshots, snapshot)
		}
	}
	sort.SliceStable(snapshots, func(i, j int) bool {
		return snapshots[i].Device.ID < snapshots[j].Device.ID
	})
	if len(snapshots) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return snapshots, nil
}

// RefreshDevice refreshes a single device snapshot.
func (c *Client) RefreshDevice(ctx context.Context, snapshot DeviceSnapshot) (DeviceSnapshot, error) {
	return c.loadSnapshotByInfo(ctx, snapshot.Info)
}

// RefreshDeviceByID refreshes a device snapshot by device ID.
func (c *Client) RefreshDeviceByID(ctx context.Context, deviceID string) (DeviceSnapshot, error) {
	snapshots, err := c.Sync(ctx)
	if err != nil {
		return DeviceSnapshot{}, err
	}
	for _, snapshot := range snapshots {
		if snapshot.Device.ID == deviceID {
			return snapshot, nil
		}
	}
	return DeviceSnapshot{}, errors.New("device not found")
}

// ExecuteCommand dispatches a command to the appropriate device handler.
func (c *Client) ExecuteCommand(ctx context.Context, snapshot DeviceSnapshot, req models.CommandRequest) error {
	switch snapshot.Device.Kind {
	case models.DeviceKindPetFeeder:
		return c.executeFeeder(ctx, snapshot, req)
	case models.DeviceKindPetLitterBox:
		return c.executeLitter(ctx, snapshot, req)
	case models.DeviceKindPetFountain:
		return c.executeFountain(ctx, snapshot, req)
	default:
		return fmt.Errorf("unsupported device kind %s", snapshot.Device.Kind)
	}
}

// BuildSnapshot constructs a DeviceSnapshot from raw API data.
func (c *Client) BuildSnapshot(info PetkitDeviceInfo, detail map[string]any, records map[string]any) (DeviceSnapshot, error) {
	kind, ok := KindFromPetkitType(info.DeviceType)
	if !ok {
		return DeviceSnapshot{}, fmt.Errorf("unsupported Petkit device type %q", info.DeviceType)
	}
	if mac := stringFromAny(detail["mac"], ""); mac != "" {
		info.MAC = mac
		if info.UniqueID == "" {
			info.UniqueID = mac
		}
	}
	device := BuildDevice(info, kind, detail, records, c.Cfg.Name)
	state := BuildState(info, kind, detail, records)
	latestEvent := (*DeviceOccurredEvent)(nil)
	if kind == models.DeviceKindPetFeeder && records != nil {
		latestEvent = LatestFeederOccurredEvent(records)
	}
	return DeviceSnapshot{
		Info:   info,
		Device: device,
		State: models.DeviceStateSnapshot{
			DeviceID: device.ID,
			PluginID: device.PluginID,
			TS:       time.Now().UTC(),
			State:    state,
		},
		Detail:      detail,
		Records:     records,
		LatestEvent: latestEvent,
	}, nil
}

func (c *Client) loadSnapshotByInfo(ctx context.Context, info PetkitDeviceInfo) (DeviceSnapshot, error) {
	detail, err := c.LoadDeviceDetail(ctx, info)
	if err != nil {
		return DeviceSnapshot{}, err
	}
	records := map[string]any(nil)
	if kind, supported := KindFromPetkitType(info.DeviceType); supported && kind == models.DeviceKindPetFeeder {
		records, err = c.LoadFeederRecords(ctx, info)
		if err != nil {
			return DeviceSnapshot{}, err
		}
	}
	snapshot, err := c.BuildSnapshot(info, detail, records)
	if err != nil {
		return DeviceSnapshot{}, err
	}
	snapshot.AccountName = AccountKey(c.Cfg)
	snapshot.Client = c
	return snapshot, nil
}

func (c *Client) loadFamilies(ctx context.Context) ([]PetkitFamily, error) {
	resp, err := c.getSessionJSON(ctx, PetkitEndpointFamilyList, nil)
	if err != nil {
		return nil, err
	}
	rawList, ok := resp.([]any)
	if !ok {
		return nil, errors.New("unexpected Petkit family list response")
	}
	families := make([]PetkitFamily, 0, len(rawList))
	for _, item := range rawList {
		family, ok := ParseFamily(item)
		if !ok {
			continue
		}
		families = append(families, family)
	}
	return families, nil
}

// LoadDeviceDetail fetches the device detail from the Petkit API.
func (c *Client) LoadDeviceDetail(ctx context.Context, info PetkitDeviceInfo) (map[string]any, error) {
	resp, err := c.postTypedSessionJSON(ctx, info.DeviceType, PetkitEndpointDeviceDetail, url.Values{
		"id": []string{strconv.Itoa(info.DeviceID)},
	})
	if err != nil {
		return nil, err
	}
	detail, ok := resp.(map[string]any)
	if !ok {
		return nil, errors.New("unexpected Petkit device detail response")
	}
	return detail, nil
}

// AccountKey returns a stable key for an account config.
func AccountKey(cfg AccountConfig) string {
	return accountKeyString(cfg.Name + "|" + cfg.Username + "|" + cfg.Region)
}

func accountKeyString(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer(" ", "-", "/", "-", "\\", "-", "|", "-").Replace(value)
	return value
}
