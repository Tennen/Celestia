package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (c *Client) Sync(ctx context.Context) ([]deviceSnapshot, error) {
	if err := c.ensureSession(ctx); err != nil {
		return nil, err
	}
	families, err := c.loadFamilies(ctx)
	if err != nil {
		return nil, err
	}
	snapshots := make([]deviceSnapshot, 0, len(families)*4)
	var firstErr error
	for _, family := range families {
		for _, item := range family.DeviceList {
			info, ok := buildDeviceInfo(item)
			if !ok {
				continue
			}
			detail, err := c.loadDeviceDetail(ctx, info)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			records := map[string]any(nil)
			if kind, supported := kindFromPetkitType(info.DeviceType); supported && kind == models.DeviceKindPetFeeder {
				records, err = c.loadFeederRecords(ctx, info)
				if err != nil && firstErr == nil {
					firstErr = err
				}
			}
			snapshot, err := c.buildSnapshot(info, detail, records)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			snapshot.AccountName = accountKey(c.cfg)
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

func (c *Client) RefreshDevice(ctx context.Context, snapshot deviceSnapshot) (deviceSnapshot, error) {
	return c.loadSnapshotByInfo(ctx, snapshot.Info)
}

func (c *Client) RefreshDeviceByID(ctx context.Context, deviceID string) (deviceSnapshot, error) {
	snapshots, err := c.Sync(ctx)
	if err != nil {
		return deviceSnapshot{}, err
	}
	for _, snapshot := range snapshots {
		if snapshot.Device.ID == deviceID {
			return snapshot, nil
		}
	}
	return deviceSnapshot{}, errors.New("device not found")
}

func (c *Client) ExecuteCommand(ctx context.Context, snapshot deviceSnapshot, req models.CommandRequest) error {
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

func (c *Client) buildSnapshot(info petkitDeviceInfo, detail map[string]any, records map[string]any) (deviceSnapshot, error) {
	kind, ok := kindFromPetkitType(info.DeviceType)
	if !ok {
		return deviceSnapshot{}, fmt.Errorf("unsupported Petkit device type %q", info.DeviceType)
	}
	if mac := stringFromAny(detail["mac"], ""); mac != "" {
		info.MAC = mac
		if info.UniqueID == "" {
			info.UniqueID = mac
		}
	}
	device := buildDevice(info, kind, detail, records, c.cfg.Name)
	state := buildState(info, kind, detail, records)
	latestEvent := (*deviceOccurredEvent)(nil)
	if kind == models.DeviceKindPetFeeder && records != nil {
		latestEvent = latestFeederOccurredEvent(records)
	}
	return deviceSnapshot{
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

func (c *Client) loadSnapshotByInfo(ctx context.Context, info petkitDeviceInfo) (deviceSnapshot, error) {
	detail, err := c.loadDeviceDetail(ctx, info)
	if err != nil {
		return deviceSnapshot{}, err
	}
	records := map[string]any(nil)
	if kind, supported := kindFromPetkitType(info.DeviceType); supported && kind == models.DeviceKindPetFeeder {
		records, err = c.loadFeederRecords(ctx, info)
		if err != nil {
			return deviceSnapshot{}, err
		}
	}
	snapshot, err := c.buildSnapshot(info, detail, records)
	if err != nil {
		return deviceSnapshot{}, err
	}
	snapshot.AccountName = accountKey(c.cfg)
	snapshot.Client = c
	return snapshot, nil
}

func (c *Client) loadFamilies(ctx context.Context) ([]petkitFamily, error) {
	resp, err := c.getSessionJSON(ctx, petkitEndpointFamilyList, nil)
	if err != nil {
		return nil, err
	}
	rawList, ok := resp.([]any)
	if !ok {
		return nil, errors.New("unexpected Petkit family list response")
	}
	families := make([]petkitFamily, 0, len(rawList))
	for _, item := range rawList {
		family, ok := parseFamily(item)
		if !ok {
			continue
		}
		families = append(families, family)
	}
	return families, nil
}

func (c *Client) loadDeviceDetail(ctx context.Context, info petkitDeviceInfo) (map[string]any, error) {
	resp, err := c.postTypedSessionJSON(ctx, info.DeviceType, petkitEndpointDeviceDetail, url.Values{
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
