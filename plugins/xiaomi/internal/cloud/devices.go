package cloud

import (
	"context"
	"sort"

	"github.com/chentianyu/celestia/internal/xiaomi/oauth"
)

func (c *Client) ListDevices(ctx context.Context, selectedHomeIDs []string) ([]DeviceRecord, error) {
	if c.usesLegacyAuth() {
		return c.legacyListDevices(ctx, selectedHomeIDs)
	}
	if err := c.ensureOAuthToken(ctx); err != nil {
		return nil, err
	}
	homeInfos, err := c.oauthHomeInfos(ctx)
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, id := range selectedHomeIDs {
		selected[id] = true
	}

	deviceMeta := map[string]DeviceRecord{}
	for _, bucket := range []map[string]homeInfo{homeInfos.HomeList, homeInfos.ShareHomeList} {
		for homeID, home := range bucket {
			if len(selected) > 0 && !selected[homeID] {
				continue
			}
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
	}
	if len(deviceMeta) == 0 {
		return nil, nil
	}

	deviceInfos, err := c.oauthDeviceInfos(ctx, sortedKeys(deviceMeta))
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
		meta.URN = stringValue(info["urn"])
		meta.Model = stringValue(info["model"])
		meta.Online = boolValue(info["online"])
		meta.VoiceCtrl = intValue(info["voice_ctrl"])
		if meta.RoomName == "" {
			meta.RoomName = meta.HomeName
		}
		devices = append(devices, meta)
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	return devices, nil
}

type homeInfosResponse struct {
	HomeList      map[string]homeInfo
	ShareHomeList map[string]homeInfo
}

type homeInfo struct {
	UID      string
	HomeName string
	GroupID  string
	DIDs     []string
	RoomInfo map[string]roomInfo
}

type roomInfo struct {
	RoomName string
	DIDs     []string
}

func (c *Client) oauthHomeInfos(ctx context.Context) (homeInfosResponse, error) {
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
			ShareHomeList []struct {
				ID       any      `json:"id"`
				UID      any      `json:"uid"`
				Name     string   `json:"name"`
				DIDs     []string `json:"dids"`
				RoomList []struct {
					ID   any      `json:"id"`
					Name string   `json:"name"`
					DIDs []string `json:"dids"`
				} `json:"roomlist"`
			} `json:"share_home_list"`
		} `json:"result"`
	}
	if err := c.oauthAPIJSON(ctx, "/app/v2/homeroom/gethome", map[string]any{
		"limit":           150,
		"fetch_share":     true,
		"fetch_share_dev": true,
		"plat_form":       0,
		"app_ver":         9,
	}, &body); err != nil {
		return homeInfosResponse{}, err
	}
	out := homeInfosResponse{
		HomeList:      map[string]homeInfo{},
		ShareHomeList: map[string]homeInfo{},
	}
	for _, item := range body.Result.HomeList {
		out.HomeList[stringify(item.ID)] = parseHomeInfo(item.Name, item.UID, item.DIDs, item.RoomList, stringify(item.ID))
	}
	for _, item := range body.Result.ShareHomeList {
		out.ShareHomeList[stringify(item.ID)] = parseHomeInfo(item.Name, item.UID, item.DIDs, item.RoomList, stringify(item.ID))
	}
	return out, nil
}

func parseHomeInfo(name string, uid any, dids []string, rooms []struct {
	ID   any      `json:"id"`
	Name string   `json:"name"`
	DIDs []string `json:"dids"`
}, homeID string) homeInfo {
	info := homeInfo{
		UID:      stringify(uid),
		HomeName: name,
		GroupID:  stringify(uid) + ":" + homeID,
		DIDs:     dids,
		RoomInfo: map[string]roomInfo{},
	}
	for _, room := range rooms {
		info.RoomInfo[stringify(room.ID)] = roomInfo{
			RoomName: room.Name,
			DIDs:     room.DIDs,
		}
	}
	return info
}

func (c *Client) oauthDeviceInfos(ctx context.Context, dids []string) (map[string]map[string]any, error) {
	result := map[string]map[string]any{}
	start := ""
	for {
		payload := map[string]any{
			"limit":            200,
			"get_split_device": true,
			"get_third_device": true,
			"dids":             dids,
		}
		if start != "" {
			payload["start_did"] = start
		}
		var body struct {
			Result struct {
				List         []map[string]any `json:"list"`
				HasMore      bool             `json:"has_more"`
				NextStartDID string           `json:"next_start_did"`
			} `json:"result"`
		}
		if err := c.oauthAPIJSON(ctx, "/app/v2/home/device_list_page", payload, &body); err != nil {
			return nil, err
		}
		for _, item := range body.Result.List {
			did := stringValue(item["did"])
			if did == "" {
				continue
			}
			result[did] = map[string]any{
				"did":        did,
				"uid":        item["uid"],
				"name":       item["name"],
				"urn":        item["spec_type"],
				"model":      item["model"],
				"online":     item["isOnline"],
				"voice_ctrl": item["voice_ctrl"],
			}
		}
		if !body.Result.HasMore || body.Result.NextStartDID == "" {
			break
		}
		start = body.Result.NextStartDID
	}
	return result, nil
}

func sortedKeys(items map[string]DeviceRecord) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
