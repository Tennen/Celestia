package app

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const (
	petkitEndpointFamilyList   = "group/family/list"
	petkitEndpointDeviceDetail = "device_detail"
)

type requestAuthMode int

const (
	requestAuthPublic requestAuthMode = iota
	requestAuthSession
)

type sessionInfo struct {
	ID        string
	UserID    string
	ExpiresIn int
	Region    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type petkitDeviceInfo struct {
	DeviceID   int
	DeviceType string
	GroupID    int
	TypeCode   int
	UniqueID   string
	DeviceName string
	CreatedAt  int
	MAC        string
}

type Client struct {
	mu          sync.Mutex
	cfg         AccountConfig
	compat      CompatConfig
	httpClient  *http.Client
	baseURL     string
	session     *sessionInfo
	bleCounters map[int]int
	lastSyncErr error
}

func NewClient(cfg AccountConfig, compat CompatConfig) *Client {
	client := &Client{
		cfg:    cfg,
		compat: compat,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		bleCounters: map[int]int{},
	}
	if baseURL := strings.TrimSpace(cfg.SessionBaseURL); baseURL != "" {
		client.baseURL = strings.TrimRight(baseURL, "/") + "/"
	}
	if session, ok := storedSessionFromConfig(cfg); ok {
		client.session = session
	}
	return client
}

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
			snapshot, err := c.buildSnapshot(info, detail)
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

func (c *Client) buildSnapshot(info petkitDeviceInfo, detail map[string]any) (deviceSnapshot, error) {
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
	device := buildDevice(info, kind, detail, c.cfg.Name)
	state := buildState(info, kind, detail)
	return deviceSnapshot{
		Info:   info,
		Device: device,
		State: models.DeviceStateSnapshot{
			DeviceID: device.ID,
			PluginID: device.PluginID,
			TS:       time.Now().UTC(),
			State:    state,
		},
		Detail: detail,
	}, nil
}

func (c *Client) loadSnapshotByInfo(ctx context.Context, info petkitDeviceInfo) (deviceSnapshot, error) {
	detail, err := c.loadDeviceDetail(ctx, info)
	if err != nil {
		return deviceSnapshot{}, err
	}
	snapshot, err := c.buildSnapshot(info, detail)
	if err != nil {
		return deviceSnapshot{}, err
	}
	snapshot.AccountName = accountKey(c.cfg)
	snapshot.Client = c
	return snapshot, nil
}

func (c *Client) executeFeeder(ctx context.Context, snapshot deviceSnapshot, req models.CommandRequest) error {
	if req.Action != "feed_once" {
		return fmt.Errorf("action %q is not supported for Petkit feeder", req.Action)
	}
	portions := intFromAny(req.Params["portions"], 1)
	if portions <= 0 {
		return errors.New("portions must be greater than zero")
	}
	endpoint := "saveDailyFeed"
	if isFeederMini(snapshot.Info.DeviceType) || snapshot.Info.DeviceType == "feeder" {
		endpoint = "save_dailyfeed"
	}
	form := url.Values{}
	form.Set("day", time.Now().Format("20060102"))
	form.Set("deviceId", strconv.Itoa(snapshot.Info.DeviceID))
	form.Set("name", "")
	form.Set("time", "-1")
	form.Set("amount", strconv.Itoa(portions))
	_, err := c.postSessionForm(ctx, endpoint, form)
	return err
}

func (c *Client) executeLitter(ctx context.Context, snapshot deviceSnapshot, req models.CommandRequest) error {
	var actionName string
	switch req.Action {
	case "clean_now":
		actionName = "start_action"
	case "pause":
		actionName = "stop_action"
	case "resume":
		actionName = "continue_action"
	default:
		return fmt.Errorf("action %q is not supported for Petkit litter box", req.Action)
	}
	form := url.Values{}
	form.Set("id", strconv.Itoa(snapshot.Info.DeviceID))
	form.Set("type", actionName)
	kv, _ := json.Marshal(map[string]any{actionName: 0})
	form.Set("kv", string(kv))
	_, err := c.postSessionForm(ctx, "controlDevice", form)
	return err
}

func (c *Client) executeFountain(ctx context.Context, snapshot deviceSnapshot, req models.CommandRequest) error {
	var command FountainAction
	switch req.Action {
	case "set_power", "turn_on", "power_on":
		if boolValue(req.Params["on"], true) {
			command = FountainActionPowerOn
		} else {
			command = FountainActionPowerOff
		}
	case "turn_off", "power_off":
		command = FountainActionPowerOff
	case "pause":
		command = FountainActionPause
	case "resume", "continue":
		command = FountainActionContinue
	case "reset_filter":
		command = FountainActionResetFilter
	default:
		return fmt.Errorf("action %q is not supported for Petkit fountain", req.Action)
	}
	return c.sendFountainCommand(ctx, snapshot.Info, command)
}

func (c *Client) sendFountainCommand(ctx context.Context, info petkitDeviceInfo, command FountainAction) error {
	connected, err := c.openBleConnection(ctx, info)
	if err != nil {
		return err
	}
	if !connected {
		return errors.New("unable to establish Petkit fountain BLE relay connection")
	}
	defer func() {
		_ = c.closeBleConnection(context.Background(), info)
	}()

	commandData, ok := fountainCommandMap[command]
	if !ok {
		return fmt.Errorf("unsupported fountain command %q", command)
	}
	cmdCode, encoded := encodeBleCommand(commandData, c.nextBleCounter(info.DeviceID))
	form := url.Values{}
	form.Set("bleId", strconv.Itoa(info.DeviceID))
	form.Set("type", strconv.Itoa(info.TypeCode))
	form.Set("mac", info.MAC)
	form.Set("cmd", strconv.Itoa(cmdCode))
	form.Set("data", encoded)
	_, err = c.postSessionForm(ctx, "ble/controlDevice", form)
	return err
}

func (c *Client) openBleConnection(ctx context.Context, info petkitDeviceInfo) (bool, error) {
	groupID := info.GroupID
	resp, err := c.postSessionForm(ctx, "ble/ownSupportBleDevices", url.Values{
		"groupId": []string{strconv.Itoa(groupID)},
	})
	if err != nil {
		return false, err
	}
	if relays, ok := resp.([]any); ok && len(relays) == 0 {
		return false, nil
	}
	resp2, err := c.postSessionForm(ctx, "ble/connect", url.Values{
		"bleId": []string{strconv.Itoa(info.DeviceID)},
		"type":  []string{strconv.Itoa(info.TypeCode)},
		"mac":   []string{info.MAC},
	})
	if err != nil {
		return false, err
	}
	if val, ok := resp2.(map[string]any); ok {
		if state := intFromAny(val["state"], 0); state != 1 {
			return false, nil
		}
	}
	for attempts := 0; attempts < 12; attempts++ {
		resp, err := c.postSessionForm(ctx, "ble/poll", url.Values{
			"bleId": []string{strconv.Itoa(info.DeviceID)},
			"type":  []string{strconv.Itoa(info.TypeCode)},
			"mac":   []string{info.MAC},
		})
		if err != nil {
			return false, err
		}
		switch value := resp.(type) {
		case float64:
			if int(value) == int(BluetoothStateConnected) {
				return true, nil
			}
			if int(value) == int(BluetoothStateError) {
				return false, nil
			}
		case int:
			if value == int(BluetoothStateConnected) {
				return true, nil
			}
			if value == int(BluetoothStateError) {
				return false, nil
			}
		case map[string]any:
			state := intFromAny(value["state"], 0)
			if state == int(BluetoothStateConnected) {
				return true, nil
			}
			if state == int(BluetoothStateError) {
				return false, nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return false, nil
}

func (c *Client) closeBleConnection(ctx context.Context, info petkitDeviceInfo) error {
	_, err := c.postSessionForm(ctx, "ble/cancel", url.Values{
		"bleId": []string{strconv.Itoa(info.DeviceID)},
		"type":  []string{strconv.Itoa(info.TypeCode)},
		"mac":   []string{info.MAC},
	})
	return err
}

func (c *Client) nextBleCounter(deviceID int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	counter := c.bleCounters[deviceID] + 1
	if counter > 255 {
		counter = 1
	}
	c.bleCounters[deviceID] = counter
	return counter
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
	resp, err := c.getSessionJSON(ctx, petkitEndpointDeviceDetail, url.Values{
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

func (c *Client) postSessionForm(ctx context.Context, endpoint string, form url.Values) (any, error) {
	return c.doRequest(ctx, http.MethodPost, endpoint, nil, form, requestAuthSession)
}

func (c *Client) getSessionJSON(ctx context.Context, endpoint string, params url.Values) (any, error) {
	return c.doRequest(ctx, http.MethodGet, endpoint, params, nil, requestAuthSession)
}

func (c *Client) getPublicJSON(ctx context.Context, endpoint string, params url.Values) (any, error) {
	return c.doRequest(ctx, http.MethodGet, endpoint, params, nil, requestAuthPublic)
}

func (c *Client) doRequest(
	ctx context.Context,
	method string,
	endpoint string,
	params url.Values,
	form url.Values,
	authMode requestAuthMode,
) (any, error) {
	useSession := authMode == requestAuthSession
	for attempt := 0; attempt < 2; attempt++ {
		baseURL, session, err := c.snapshotTransport()
		if err != nil {
			return nil, err
		}
		reqURL := endpoint
		if !strings.HasPrefix(endpoint, "http") {
			reqURL = strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(endpoint, "/")
		}
		if params != nil && len(params) > 0 {
			join := "?"
			if strings.Contains(reqURL, "?") {
				join = "&"
			}
			reqURL += join + params.Encode()
		}

		var body io.Reader
		if form != nil {
			body = strings.NewReader(form.Encode())
		}
		req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", c.compat.AcceptLanguage)
		req.Header.Set("Accept-Encoding", "gzip, deflate")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", c.compat.UserAgent)
		req.Header.Set("X-Img-Version", "1")
		req.Header.Set("X-Locale", c.compat.Locale)
		req.Header.Set("X-Client", c.compatClientHeader())
		req.Header.Set("X-Hour", c.compat.HourMode)
		req.Header.Set("X-TimezoneId", c.cfg.Timezone)
		req.Header.Set("X-Api-Version", c.compat.APIVersion)
		req.Header.Set("X-Timezone", timezoneOffset(c.cfg.Timezone))
		if useSession && session != nil {
			req.Header.Set("F-Session", session.ID)
			req.Header.Set("X-Session", session.ID)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		bodyBytes, err := readPetkitBody(resp)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized && useSession && attempt == 0 {
			if err := c.login(ctx); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode >= 400 {
			if len(bodyBytes) == 0 {
				return nil, fmt.Errorf("petkit request failed with status %d", resp.StatusCode)
			}
			return nil, fmt.Errorf("petkit request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
		}
		var payload any
		if len(bodyBytes) == 0 {
			return nil, nil
		}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return nil, fmt.Errorf("invalid Petkit response: %w", err)
		}
		switch value := payload.(type) {
		case map[string]any:
			if code, message, ok := petkitAPIError(value); ok {
				if code == 5 && useSession && attempt == 0 {
					if err := c.login(ctx); err != nil {
						return nil, err
					}
					continue
				}
				return nil, fmt.Errorf("petkit api error map[code:%d msg:%s]", code, message)
			}
			if result, ok := value["result"]; ok {
				return result, nil
			}
			if sessionValue, ok := value["session"]; ok {
				return sessionValue, nil
			}
			return value, nil
		default:
			return payload, nil
		}
	}
	return nil, errors.New("petkit request failed after re-authentication")
}

func petkitAPIError(payload map[string]any) (int, string, bool) {
	if errObj, ok := payload["error"].(map[string]any); ok {
		code := intFromAny(errObj["code"], 0)
		message := firstNonEmpty(
			stringFromAny(errObj["msg"], ""),
			stringFromAny(errObj["message"], ""),
			stringFromAny(errObj["desc"], ""),
		)
		if code != 0 || message != "" {
			return code, message, true
		}
	}
	code := intFromAny(payload["code"], 0)
	message := firstNonEmpty(
		stringFromAny(payload["msg"], ""),
		stringFromAny(payload["message"], ""),
		stringFromAny(payload["desc"], ""),
	)
	if code != 0 || message != "" {
		return code, message, true
	}
	return 0, "", false
}

func readPetkitBody(resp *http.Response) ([]byte, error) {
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	switch {
	case strings.Contains(encoding, "gzip"):
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	case strings.Contains(encoding, "deflate"):
		reader, err := zlib.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return io.ReadAll(reader)
	default:
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if len(bodyBytes) >= 2 && bodyBytes[0] == 0x1f && bodyBytes[1] == 0x8b {
			reader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
			if err != nil {
				return nil, err
			}
			defer reader.Close()
			return io.ReadAll(reader)
		}
		return bodyBytes, nil
	}
}

func (c *Client) snapshotTransport() (string, *sessionInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.baseURL == "" {
		c.baseURL = c.defaultBaseURLForRegion()
	}
	return c.baseURL, c.session, nil
}

func (c *Client) CurrentSession() (string, sessionInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil || c.session.ID == "" || strings.TrimSpace(c.baseURL) == "" {
		return "", sessionInfo{}, false
	}
	return c.baseURL, *c.session, true
}

func (c *Client) ensureSession(ctx context.Context) error {
	c.mu.Lock()
	session := c.session
	baseURL := c.baseURL
	c.mu.Unlock()
	if session != nil && time.Until(session.ExpiresAt) > time.Minute && baseURL != "" {
		return nil
	}
	return c.login(ctx)
}

func (c *Client) login(ctx context.Context) error {
	baseURL, err := c.resolveBaseURL(ctx)
	if err != nil {
		return err
	}
	form := url.Values{}
	form.Set("oldVersion", c.compat.APIVersion)
	form.Set("client", c.compatClientPayload(c.cfg.Timezone))
	form.Set("encrypt", "1")
	form.Set("region", c.cfg.Region)
	form.Set("username", c.cfg.Username)
	form.Set("password", md5Hex(c.cfg.Password))
	result, err := c.doRequest(ctx, http.MethodPost, baseURL+"user/login", nil, form, requestAuthPublic)
	if err != nil {
		return err
	}
	sessionMap, ok := result.(map[string]any)
	if !ok {
		return errors.New("unexpected Petkit login response")
	}
	sessionVal, ok := sessionMap["session"].(map[string]any)
	if !ok {
		return errors.New("missing Petkit session in login response")
	}
	session, err := parseSession(sessionVal, c.cfg.Region)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.baseURL = baseURL
	c.session = session
	c.mu.Unlock()
	return nil
}

func (c *Client) resolveBaseURL(ctx context.Context) (string, error) {
	if strings.EqualFold(c.cfg.Region, "cn") || strings.EqualFold(c.cfg.Region, "china") {
		return c.compat.ChinaBaseURL, nil
	}
	resp, err := c.getPublicJSON(ctx, strings.TrimRight(c.compat.PassportBaseURL, "/")+"/v1/regionservers", nil)
	if err != nil {
		return "", err
	}
	list, ok := parseRegionServerList(resp)
	if !ok {
		return "", errors.New("unexpected Petkit region server response")
	}
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := strings.ToLower(stringFromAny(entry["id"], ""))
		name := strings.ToLower(stringFromAny(entry["name"], ""))
		if id == c.cfg.Region || name == c.cfg.Region {
			gateway := stringFromAny(entry["gateway"], "")
			if gateway == "" {
				break
			}
			return strings.TrimRight(gateway, "/") + "/", nil
		}
	}
	return "", fmt.Errorf("Petkit region %q not found", c.cfg.Region)
}

func parseRegionServerList(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case map[string]any:
		list, ok := typed["list"].([]any)
		if ok {
			return list, true
		}
	}
	return nil, false
}

func parseFamily(value any) (petkitFamily, bool) {
	entry, ok := value.(map[string]any)
	if !ok {
		return petkitFamily{}, false
	}
	family := petkitFamily{}
	if rawDevices, ok := entry["deviceList"].([]any); ok {
		for _, raw := range rawDevices {
			device, ok := parseDeviceRecord(raw)
			if !ok {
				continue
			}
			family.DeviceList = append(family.DeviceList, device)
		}
	}
	if rawPets, ok := entry["petList"].([]any); ok {
		for _, raw := range rawPets {
			pet, ok := parsePetRecord(raw)
			if !ok {
				continue
			}
			family.PetList = append(family.PetList, pet)
		}
	}
	return family, true
}

func parseDeviceRecord(value any) (petkitDeviceInfo, bool) {
	entry, ok := value.(map[string]any)
	if !ok {
		return petkitDeviceInfo{}, false
	}
	deviceID := intFromAny(entry["deviceId"], 0)
	deviceType := strings.ToLower(stringFromAny(entry["deviceType"], ""))
	groupID := intFromAny(entry["groupId"], 0)
	typeCode := intFromAny(entry["typeCode"], 0)
	uniqueID := stringFromAny(entry["uniqueId"], "")
	if uniqueID == "" {
		uniqueID = strings.ToLower(fmt.Sprintf("%s-%d", deviceType, deviceID))
	}
	return petkitDeviceInfo{
		DeviceID:   deviceID,
		DeviceType: deviceType,
		GroupID:    groupID,
		TypeCode:   typeCode,
		UniqueID:   uniqueID,
		DeviceName: stringFromAny(entry["deviceName"], ""),
		CreatedAt:  intFromAny(entry["createdAt"], 0),
	}, true
}

func parsePetRecord(value any) (petkitPetInfo, bool) {
	entry, ok := value.(map[string]any)
	if !ok {
		return petkitPetInfo{}, false
	}
	return petkitPetInfo{
		ID:   intFromAny(entry["petId"], 0),
		Name: stringFromAny(entry["petName"], ""),
		SN:   stringFromAny(entry["sn"], ""),
	}, true
}

func buildDeviceInfo(info petkitDeviceInfo) (petkitDeviceInfo, bool) {
	if info.DeviceID == 0 || info.DeviceType == "" {
		return petkitDeviceInfo{}, false
	}
	return info, true
}

func buildDevice(info petkitDeviceInfo, kind models.DeviceKind, detail map[string]any, accountLabel string) models.Device {
	name := info.DeviceName
	if name == "" {
		name = strings.Title(strings.ReplaceAll(info.DeviceType, "_", " "))
	}
	state := buildState(info, kind, detail)
	caps := capabilitiesForKind(kind)
	return models.Device{
		ID:             fmt.Sprintf("petkit:%s:%d", info.DeviceType, info.DeviceID),
		PluginID:       "petkit",
		VendorDeviceID: strconv.Itoa(info.DeviceID),
		Kind:           kind,
		Name:           name,
		Room:           "",
		Online:         boolFromAny(state["online"], true),
		Capabilities:   caps,
		Metadata: map[string]any{
			"account":     accountLabel,
			"group_id":    info.GroupID,
			"device_type": info.DeviceType,
			"type_code":   info.TypeCode,
			"unique_id":   info.UniqueID,
			"created_at":  info.CreatedAt,
			"source":      "petkit-cloud",
		},
	}
}

func buildState(info petkitDeviceInfo, kind models.DeviceKind, detail map[string]any) map[string]any {
	state := map[string]any{
		"online": true,
		"raw":    detail,
	}
	switch kind {
	case models.DeviceKindPetFeeder:
		stateMap := mapFromAny(detail["state"])
		if stateMap == nil {
			stateMap = detail
		}
		state["food_level"] = intFromAny(firstAny(stateMap, "food", "foodLevel"), 0)
		state["battery_power"] = intFromAny(firstAny(stateMap, "batteryPower"), 0)
		state["feeding"] = intFromAny(firstAny(stateMap, "feeding"), 0)
		state["error_code"] = stringFromAny(firstAny(stateMap, "errorCode"), "")
		state["error_msg"] = stringFromAny(firstAny(stateMap, "errorMsg"), "")
		state["status"] = feederStatusFromDetail(detail)
	case models.DeviceKindPetLitterBox:
		stateMap := mapFromAny(detail["state"])
		if stateMap == nil {
			stateMap = detail
		}
		state["waste_level"] = intFromAny(firstAny(stateMap, "sandPercent"), 0)
		state["box_full"] = boolFromAny(firstAny(stateMap, "boxFull"), false)
		state["low_power"] = boolFromAny(firstAny(stateMap, "lowPower"), false)
		state["error_code"] = stringFromAny(firstAny(stateMap, "errorCode"), "")
		state["error_msg"] = stringFromAny(firstAny(stateMap, "errorMsg"), "")
		state["status"] = litterStatusFromDetail(detail)
		state["last_usage"] = stringFromAny(firstAny(stateMap, "lastOutTime"), "")
	case models.DeviceKindPetFountain:
		statusMap := mapFromAny(detail["status"])
		if statusMap == nil {
			statusMap = detail
		}
		state["power_status"] = intFromAny(firstAny(statusMap, "powerStatus"), 0)
		state["run_status"] = intFromAny(firstAny(statusMap, "runStatus"), 0)
		state["suspend_status"] = intFromAny(firstAny(statusMap, "suspendStatus"), 0)
		state["detect_status"] = intFromAny(firstAny(statusMap, "detectStatus"), 0)
		state["filter_percent"] = intFromAny(firstAny(detail, "filterPercent"), 0)
		state["water_pump_run_time"] = intFromAny(firstAny(detail, "waterPumpRunTime"), 0)
		state["relay_status"] = "cloud_ble"
	default:
		state["supported"] = false
	}
	return state
}

func capabilitiesForKind(kind models.DeviceKind) []string {
	switch kind {
	case models.DeviceKindPetFeeder:
		return []string{"feed_once", "food_level", "online", "error"}
	case models.DeviceKindPetLitterBox:
		return []string{"clean_now", "pause", "resume", "waste_level", "online", "error", "last_usage"}
	case models.DeviceKindPetFountain:
		return []string{"set_power", "turn_on", "turn_off", "pause", "resume", "reset_filter", "relay_status"}
	default:
		return nil
	}
}

func kindFromPetkitType(deviceType string) (models.DeviceKind, bool) {
	switch strings.ToLower(deviceType) {
	case "feeder", "feedermini", "d3", "d4", "d4s", "d4h", "d4sh":
		return models.DeviceKindPetFeeder, true
	case "t3", "t4", "t5", "t6", "t7":
		return models.DeviceKindPetLitterBox, true
	case "w4", "w5", "ctw2", "ctw3":
		return models.DeviceKindPetFountain, true
	default:
		return "", false
	}
}

func feederStatusFromDetail(detail map[string]any) string {
	stateMap := mapFromAny(detail["state"])
	if stateMap == nil {
		stateMap = detail
	}
	if intFromAny(firstAny(stateMap, "feeding"), 0) > 0 {
		return "feeding"
	}
	if stringFromAny(firstAny(stateMap, "errorCode"), "") != "" {
		return "error"
	}
	return "idle"
}

func litterStatusFromDetail(detail map[string]any) string {
	stateMap := mapFromAny(detail["state"])
	if stateMap == nil {
		stateMap = detail
	}
	if stringFromAny(firstAny(stateMap, "errorCode"), "") != "" {
		return "error"
	}
	workState := mapFromAny(firstAny(stateMap, "workState"))
	if workState != nil && intFromAny(firstAny(workState, "workProcess"), 0) > 0 {
		return "cleaning"
	}
	return "idle"
}

func isFeederMini(deviceType string) bool {
	return strings.EqualFold(deviceType, "feedermini")
}

func encodeBleCommand(command []int, counter int) (int, string) {
	if len(command) == 0 {
		return 0, ""
	}
	if len(command) > 2 {
		command = append([]int{command[0], command[1], counter}, command[2:]...)
	} else {
		command = append(command, counter)
	}
	bleData := append([]int{250, 252, 253}, command...)
	bleData = append(bleData, 251)
	buf := make([]byte, len(bleData))
	for i, value := range bleData {
		buf[i] = byte(value)
	}
	return command[0], url.QueryEscape(base64Encode(buf))
}

func base64Encode(b []byte) string {
	return strings.TrimSpace(base64.StdEncoding.EncodeToString(b))
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (c *Client) compatClientHeader() string {
	if value := strings.TrimSpace(c.compat.ClientHeader); value != "" {
		return value
	}
	return fmt.Sprintf("%s(%s;%s)", c.compat.Platform, c.compat.OSVersion, c.compat.ModelName)
}

func (c *Client) compatClientPayload(timezone string) string {
	return fmt.Sprintf(
		`{"locale":"%s","name":"%s","osVersion":"%s","phoneBrand":"%s","platform":"%s","source":"%s","version":"%s","timezoneId":"%s"}`,
		c.compat.Locale,
		c.compat.ModelName,
		c.compat.OSVersion,
		c.compat.PhoneBrand,
		c.compat.Platform,
		c.compat.Source,
		c.compat.APIVersion,
		timezone,
	)
}

func (c *Client) defaultBaseURLForRegion() string {
	if strings.EqualFold(c.cfg.Region, "cn") || strings.EqualFold(c.cfg.Region, "china") {
		return strings.TrimRight(c.compat.ChinaBaseURL, "/") + "/"
	}
	return strings.TrimRight(c.compat.PassportBaseURL, "/") + "/"
}

func parseSession(session map[string]any, region string) (*sessionInfo, error) {
	sessionID := stringFromAny(session["id"], "")
	if sessionID == "" {
		return nil, errors.New("missing Petkit session id")
	}
	userID := stringFromAny(session["userId"], "")
	expiresIn := intFromAny(session["expiresIn"], 0)
	createdAt := time.Now().UTC()
	if raw := stringFromAny(session["createdAt"], ""); raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			createdAt = parsed
		}
	}
	return &sessionInfo{
		ID:        sessionID,
		UserID:    userID,
		ExpiresIn: expiresIn,
		Region:    region,
		CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(time.Duration(expiresIn) * time.Second),
	}, nil
}

func storedSessionFromConfig(cfg AccountConfig) (*sessionInfo, bool) {
	sessionID := strings.TrimSpace(cfg.SessionID)
	if sessionID == "" {
		return nil, false
	}
	expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(cfg.SessionExpiresAt))
	if err != nil || expiresAt.IsZero() || time.Now().UTC().After(expiresAt) {
		return nil, false
	}
	createdAt := time.Now().UTC()
	if raw := strings.TrimSpace(cfg.SessionCreatedAt); raw != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, raw); parseErr == nil {
			createdAt = parsed
		}
	}
	return &sessionInfo{
		ID:        sessionID,
		UserID:    strings.TrimSpace(cfg.SessionUserID),
		Region:    cfg.Region,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
		ExpiresIn: int(expiresAt.Sub(createdAt).Seconds()),
	}, true
}

var fountainCommandMap = map[FountainAction][]int{
	FountainActionPause:       {220, 1, 3, 0, 1, 0, 2},
	FountainActionContinue:    {220, 1, 3, 0, 1, 1, 2},
	FountainActionResetFilter: {222, 1, 0, 0},
	FountainActionPowerOff:    {220, 1, 3, 0, 0, 1, 1},
	FountainActionPowerOn:     {220, 1, 3, 0, 1, 1, 1},
}

type FountainAction string

const (
	FountainActionPause       FountainAction = "Pause"
	FountainActionContinue    FountainAction = "Continue"
	FountainActionResetFilter FountainAction = "Reset Filter"
	FountainActionPowerOff    FountainAction = "Power Off"
	FountainActionPowerOn     FountainAction = "Power On"
)

const (
	BluetoothStateError     = -1
	BluetoothStateConnected = 1
)

type petkitFamily struct {
	DeviceList []petkitDeviceInfo
	PetList    []petkitPetInfo
}

type petkitPetInfo struct {
	ID   int
	Name string
	SN   string
}

func stringFromAny(value any, fallback string) string {
	switch raw := value.(type) {
	case string:
		if raw != "" {
			return raw
		}
	case fmt.Stringer:
		if raw.String() != "" {
			return raw.String()
		}
	}
	return fallback
}

func intFromAny(value any, fallback int) int {
	switch raw := value.(type) {
	case int:
		return raw
	case int32:
		return int(raw)
	case int64:
		return int(raw)
	case float64:
		return int(raw)
	case json.Number:
		if v, err := raw.Int64(); err == nil {
			return int(v)
		}
	}
	return fallback
}

func boolFromAny(value any, fallback bool) bool {
	switch raw := value.(type) {
	case bool:
		return raw
	case int:
		return raw != 0
	case float64:
		return raw != 0
	case string:
		switch strings.ToLower(raw) {
		case "1", "true", "yes":
			return true
		case "0", "false", "no":
			return false
		}
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstAny(detail map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := detail[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return nil
	}
	if raw, ok := value.(map[string]any); ok {
		return raw
	}
	return nil
}
