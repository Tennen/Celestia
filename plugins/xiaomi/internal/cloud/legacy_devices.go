package cloud

import (
	"context"
	"net/http"
	"sort"

	"github.com/chentianyu/celestia/internal/xiaomi/oauth"
)

func (c *Client) legacyListDevices(ctx context.Context, selectedHomeIDs []string) ([]DeviceRecord, error) {
	if err := c.ensureLegacySession(ctx); err != nil {
		return nil, err
	}
	homeInfos, err := c.legacyHomeInfos(ctx)
	if err != nil {
		return nil, err
	}

	selected := map[string]bool{}
	for _, id := range selectedHomeIDs {
		selected[id] = true
	}

	deviceMeta := map[string]DeviceRecord{}
	homeEntries := make([]legacyHomeEntry, 0, len(homeInfos.HomeList))
	for homeID, home := range homeInfos.HomeList {
		if len(selected) > 0 && !selected[homeID] {
			continue
		}
		homeEntries = append(homeEntries, legacyHomeEntry{
			HomeID:   homeID,
			UID:      home.UID,
			HomeName: home.HomeName,
			GroupID:  home.GroupID,
		})
		for _, did := range home.DIDs {
			deviceMeta[did] = DeviceRecord{
				DID:      did,
				HomeID:   homeID,
				HomeName: home.HomeName,
				RoomID:   homeID,
				RoomName: home.HomeName,
				GroupID:  home.GroupID,
				Region:   oauth.NormalizeRegion(c.cfg.Region),
			}
		}
		for roomID, room := range home.RoomInfo {
			for _, did := range room.DIDs {
				deviceMeta[did] = DeviceRecord{
					DID:      did,
					HomeID:   homeID,
					HomeName: home.HomeName,
					RoomID:   roomID,
					RoomName: room.RoomName,
					GroupID:  home.GroupID,
					Region:   oauth.NormalizeRegion(c.cfg.Region),
				}
			}
		}
	}
	if len(deviceMeta) == 0 {
		return nil, nil
	}

	deviceInfos, err := c.legacyDeviceInfos(ctx, homeEntries)
	if err != nil {
		return nil, err
	}

	devices := make([]DeviceRecord, 0, len(deviceInfos))
	for did, info := range deviceInfos {
		meta, ok := deviceMeta[did]
		if !ok {
			continue
		}
		meta.Name = stringValue(info["name"])
		meta.UID = stringValue(info["uid"])
		meta.URN = stringValue(info["spec_type"])
		meta.Model = stringValue(info["model"])
		meta.Online = boolValue(info["isOnline"])
		meta.VoiceCtrl = intValue(info["voice_ctrl"])
		if meta.RoomName == "" {
			meta.RoomName = meta.HomeName
		}
		devices = append(devices, meta)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	return devices, nil
}

type legacyHomeEntry struct {
	HomeID   string
	UID      string
	HomeName string
	GroupID  string
}

func (c *Client) legacyHomeInfos(ctx context.Context) (homeInfosResponse, error) {
	var body struct {
		Result struct {
			HomeList []struct {
				ID       any      `json:"id"`
				UID      any      `json:"uid"`
				Name     string   `json:"name"`
				DIDs     []string `json:"dids"`
				RoomList []struct {
					ID   any      `json:"id"`
					Name string   `json:"name"`
					DIDs []string `json:"dids"`
				} `json:"roomlist"`
			} `json:"homelist"`
		} `json:"result"`
	}
	if err := c.legacyAPIJSON(ctx, http.MethodPost, "v2/homeroom/gethome_merged", map[string]any{
		"fg":              true,
		"fetch_share":     true,
		"fetch_share_dev": true,
		"fetch_cariot":    true,
		"limit":           300,
		"app_ver":         7,
		"plat_form":       0,
	}, &body); err != nil {
		return homeInfosResponse{}, err
	}
	out := homeInfosResponse{
		HomeList:      map[string]homeInfo{},
		ShareHomeList: map[string]homeInfo{},
	}
	for _, item := range body.Result.HomeList {
		homeID := stringify(item.ID)
		out.HomeList[homeID] = parseHomeInfo(item.Name, item.UID, item.DIDs, item.RoomList, homeID)
	}
	return out, nil
}

func (c *Client) legacyDeviceInfos(ctx context.Context, homes []legacyHomeEntry) (map[string]map[string]any, error) {
	result := map[string]map[string]any{}
	for _, home := range homes {
		start := ""
		for {
			var body struct {
				Result struct {
					DeviceInfo []map[string]any `json:"device_info"`
					HasMore    bool             `json:"has_more"`
					MaxDID     string           `json:"max_did"`
				} `json:"result"`
			}
			if err := c.legacyAPIJSON(ctx, http.MethodPost, "v2/home/home_device_list", map[string]any{
				"home_owner":         home.UID,
				"home_id":            home.HomeID,
				"limit":              300,
				"start_did":          start,
				"get_split_device":   false,
				"support_smart_home": true,
				"get_cariot_device":  true,
				"get_third_device":   true,
			}, &body); err != nil {
				return nil, err
			}
			for _, item := range body.Result.DeviceInfo {
				did := stringValue(item["did"])
				if did == "" {
					continue
				}
				result[did] = item
			}
			if !body.Result.HasMore || body.Result.MaxDID == "" {
				break
			}
			start = body.Result.MaxDID
		}
	}
	return result, nil
}
