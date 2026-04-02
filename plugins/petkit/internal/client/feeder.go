package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const (
	petkitEndpointAdded                = "added"
	petkitEndpointCallPet              = "callPet"
	petkitEndpointCancelRealtimeFeed   = "cancelRealtimeFeed"
	petkitEndpointCancelRealtimeFeedV1 = "cancel_realtime_feed"
	petkitEndpointDesiccantReset       = "desiccantReset"
	petkitEndpointDesiccantResetV1     = "desiccant_reset"
	petkitEndpointDailyFeedAndEat      = "dailyFeedAndEat"
	petkitEndpointDailyFeeds           = "dailyFeeds"
	petkitEndpointDailyFeedsLegacy     = "dailyfeeds"
	petkitEndpointFeedStatistic        = "feedStatistic"
	petkitEndpointGetDeviceRecord      = "getDeviceRecord"
	petkitEndpointPlaySound            = "playSound"
	petkitEndpointSaveDailyFeed        = "saveDailyFeed"
	petkitEndpointSaveDailyFeedV1      = "save_dailyfeed"
)

func isFreshElement(deviceType string) bool {
	return strings.EqualFold(deviceType, "feeder")
}

// IsDualHopperFeeder returns true for feeder models with two hoppers.
func IsDualHopperFeeder(deviceType string) bool {
	switch strings.ToLower(strings.TrimSpace(deviceType)) {
	case "d4s", "d4h", "d4sh":
		return true
	default:
		return false
	}
}

// SupportsFeederFoodReplenished returns true for feeder models that support food replenished notification.
func SupportsFeederFoodReplenished(deviceType string) bool {
	switch strings.ToLower(strings.TrimSpace(deviceType)) {
	case "d4s", "d4h", "d4sh":
		return true
	default:
		return false
	}
}

func supportsFeederPlaySound(deviceType string) bool {
	switch strings.ToLower(strings.TrimSpace(deviceType)) {
	case "d3", "d4h", "d4sh":
		return true
	default:
		return false
	}
}

// SupportsFeederCallPet returns true for feeder models that support call pet.
func SupportsFeederCallPet(deviceType string) bool {
	return strings.EqualFold(deviceType, "d3")
}

func feederManualFeedEndpoint(deviceType string) string {
	if isFeederMini(deviceType) || isFreshElement(deviceType) {
		return petkitEndpointSaveDailyFeedV1
	}
	return petkitEndpointSaveDailyFeed
}

func feederResetDesiccantEndpoint(deviceType string) string {
	if isFeederMini(deviceType) || isFreshElement(deviceType) {
		return petkitEndpointDesiccantResetV1
	}
	return petkitEndpointDesiccantReset
}

func feederCancelFeedEndpoint(deviceType string) string {
	if isFreshElement(deviceType) {
		return petkitEndpointCancelRealtimeFeedV1
	}
	return petkitEndpointCancelRealtimeFeed
}

func feederRecordEndpoint(deviceType string) string {
	switch strings.ToLower(strings.TrimSpace(deviceType)) {
	case "d3":
		return petkitEndpointDailyFeedAndEat
	case "d4":
		return petkitEndpointFeedStatistic
	case "d4s":
		return petkitEndpointDailyFeeds
	case "feedermini":
		return petkitEndpointDailyFeedsLegacy
	default:
		return petkitEndpointGetDeviceRecord
	}
}

func feederRecordParams(info PetkitDeviceInfo, requestDate time.Time) url.Values {
	date := requestDate.Format("20060102")
	if strings.EqualFold(info.DeviceType, "d4") {
		return url.Values{
			"date":     []string{date},
			"type":     []string{strconv.Itoa(info.TypeCode)},
			"deviceId": []string{strconv.Itoa(info.DeviceID)},
		}
	}
	return url.Values{
		"days":     []string{date},
		"deviceId": []string{strconv.Itoa(info.DeviceID)},
	}
}

// LoadFeederRecords fetches the feeder feed records for the given device.
func (c *Client) LoadFeederRecords(ctx context.Context, info PetkitDeviceInfo) (map[string]any, error) {
	resp, err := c.postTypedSessionJSON(ctx, info.DeviceType, feederRecordEndpoint(info.DeviceType), feederRecordParams(info, time.Now()))
	if err != nil {
		return nil, err
	}
	records, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected Petkit feeder records response")
	}
	return records, nil
}

func feederManualFeedID(detail map[string]any) string {
	manualFeed := mapFromAny(detail["manualFeed"])
	if manualFeed == nil {
		return ""
	}
	switch value := firstAny(manualFeed, "id").(type) {
	case string:
		return value
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case float64:
		return strconv.Itoa(int(value))
	default:
		return ""
	}
}

func feederSelectedSoundID(detail map[string]any) (int, bool) {
	settings := mapFromAny(detail["settings"])
	if settings == nil {
		return 0, false
	}
	value := firstAny(settings, "selectedSound")
	if value == nil {
		return 0, false
	}
	return intFromAny(value, 0), true
}

func (c *Client) executeFeederAction(ctx context.Context, snapshot DeviceSnapshot, req models.CommandRequest) error {
	switch req.Action {
	case "feed_once", "manual_feed":
		return c.sendFeederManualFeed(ctx, snapshot.Info, req.Params)
	case "manual_feed_dual":
		return c.sendFeederManualFeedDual(ctx, snapshot.Info, req.Params)
	case "cancel_manual_feed":
		return c.cancelFeederManualFeed(ctx, snapshot)
	case "reset_desiccant":
		return c.resetFeederDesiccant(ctx, snapshot.Info)
	case "food_replenished":
		return c.markFeederFoodReplenished(ctx, snapshot.Info)
	case "play_sound":
		return c.playFeederSound(ctx, snapshot, req.Params)
	case "call_pet":
		return c.callFeederPet(ctx, snapshot.Info)
	default:
		return fmt.Errorf("action %q is not supported for Petkit feeder", req.Action)
	}
}

func (c *Client) sendFeederManualFeed(ctx context.Context, info PetkitDeviceInfo, params map[string]any) error {
	amount := intFromAny(params["amount"], 0)
	if amount <= 0 {
		amount = intFromAny(params["portions"], 1)
	}
	if amount <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	form := baseFeederCommandForm(info.DeviceID)
	form.Set("amount", strconv.Itoa(amount))
	_, err := c.postTypedSessionForm(ctx, info.DeviceType, feederManualFeedEndpoint(info.DeviceType), form)
	return err
}

func (c *Client) sendFeederManualFeedDual(ctx context.Context, info PetkitDeviceInfo, params map[string]any) error {
	if !IsDualHopperFeeder(info.DeviceType) {
		return fmt.Errorf("manual_feed_dual is not supported for feeder type %s", info.DeviceType)
	}
	amount1 := intFromAny(params["amount1"], 0)
	amount2 := intFromAny(params["amount2"], 0)
	if amount1 <= 0 && amount2 <= 0 {
		return fmt.Errorf("amount1 or amount2 must be greater than zero")
	}
	form := baseFeederCommandForm(info.DeviceID)
	if amount1 > 0 {
		form.Set("amount1", strconv.Itoa(amount1))
	}
	if amount2 > 0 {
		form.Set("amount2", strconv.Itoa(amount2))
	}
	_, err := c.postTypedSessionForm(ctx, info.DeviceType, feederManualFeedEndpoint(info.DeviceType), form)
	return err
}

func (c *Client) cancelFeederManualFeed(ctx context.Context, snapshot DeviceSnapshot) error {
	form := url.Values{
		"day":      []string{time.Now().Format("20060102")},
		"deviceId": []string{strconv.Itoa(snapshot.Info.DeviceID)},
	}
	if IsDualHopperFeeder(snapshot.Info.DeviceType) {
		manualFeedID := feederManualFeedID(snapshot.Detail)
		if manualFeedID == "" {
			return fmt.Errorf("manual feed id unavailable for feeder type %s", snapshot.Info.DeviceType)
		}
		form.Set("id", manualFeedID)
	}
	_, err := c.postTypedSessionForm(ctx, snapshot.Info.DeviceType, feederCancelFeedEndpoint(snapshot.Info.DeviceType), form)
	return err
}

func (c *Client) resetFeederDesiccant(ctx context.Context, info PetkitDeviceInfo) error {
	_, err := c.postTypedSessionForm(ctx, info.DeviceType, feederResetDesiccantEndpoint(info.DeviceType), url.Values{
		"deviceId": []string{strconv.Itoa(info.DeviceID)},
	})
	return err
}

func (c *Client) markFeederFoodReplenished(ctx context.Context, info PetkitDeviceInfo) error {
	if !SupportsFeederFoodReplenished(info.DeviceType) {
		return fmt.Errorf("food_replenished is not supported for feeder type %s", info.DeviceType)
	}
	_, err := c.postTypedSessionForm(ctx, info.DeviceType, petkitEndpointAdded, url.Values{
		"deviceId": []string{strconv.Itoa(info.DeviceID)},
		"noRemind": []string{"3"},
	})
	return err
}

func (c *Client) playFeederSound(ctx context.Context, snapshot DeviceSnapshot, params map[string]any) error {
	if !supportsFeederPlaySound(snapshot.Info.DeviceType) {
		return fmt.Errorf("play_sound is not supported for feeder type %s", snapshot.Info.DeviceType)
	}
	soundID := intFromAny(params["sound_id"], 0)
	if soundID <= 0 {
		if selected, ok := feederSelectedSoundID(snapshot.Detail); ok {
			soundID = selected
		}
	}
	if soundID <= 0 {
		return fmt.Errorf("sound_id is required for feeder type %s", snapshot.Info.DeviceType)
	}
	_, err := c.postTypedSessionForm(ctx, snapshot.Info.DeviceType, petkitEndpointPlaySound, url.Values{
		"soundId":  []string{strconv.Itoa(soundID)},
		"deviceId": []string{strconv.Itoa(snapshot.Info.DeviceID)},
	})
	return err
}

func (c *Client) callFeederPet(ctx context.Context, info PetkitDeviceInfo) error {
	if !SupportsFeederCallPet(info.DeviceType) {
		return fmt.Errorf("call_pet is not supported for feeder type %s", info.DeviceType)
	}
	_, err := c.postTypedSessionForm(ctx, info.DeviceType, petkitEndpointCallPet, url.Values{
		"deviceId": []string{strconv.Itoa(info.DeviceID)},
	})
	return err
}

func baseFeederCommandForm(deviceID int) url.Values {
	return url.Values{
		"day":      []string{time.Now().Format("20060102")},
		"deviceId": []string{strconv.Itoa(deviceID)},
		"name":     []string{""},
		"time":     []string{"-1"},
	}
}

// LatestFeederOccurredEvent extracts the most recent feed event from feeder records.
func LatestFeederOccurredEvent(records map[string]any) *DeviceOccurredEvent {
	feedGroups, ok := records["feed"].([]any)
	if !ok {
		return nil
	}

	var latest map[string]any
	var latestTS time.Time
	for _, rawGroup := range feedGroups {
		group, ok := rawGroup.(map[string]any)
		if !ok {
			continue
		}
		items, ok := group["items"].([]any)
		if !ok {
			continue
		}
		for _, rawItem := range items {
			item, ok := rawItem.(map[string]any)
			if !ok {
				continue
			}
			ts := petkitRecordTimestamp(firstAny(item, "timestamp", "time", "completedAt", "startTime"))
			if latest == nil || ts.After(latestTS) {
				latest = item
				latestTS = ts
			}
		}
	}
	if latest == nil {
		return nil
	}

	eventName := firstNonEmpty(
		stringFromAny(firstAny(latest, "enumEventType"), ""),
		stringFromAny(firstAny(latest, "event"), ""),
	)
	if eventName == "" {
		switch {
		case firstAny(latest, "amount1", "amount2") != nil:
			eventName = "manual_feed_dual"
		case firstAny(latest, "amount") != nil:
			eventName = "manual_feed"
		default:
			eventName = "feed"
		}
	}

	key := firstNonEmpty(
		stringFromAny(firstAny(latest, "eventId"), ""),
		stringFromAny(firstAny(latest, "id"), ""),
	)
	if key == "" {
		key = fmt.Sprintf(
			"%s:%d:%d:%d:%d",
			eventName,
			latestTS.Unix(),
			intFromAny(firstAny(latest, "amount"), 0),
			intFromAny(firstAny(latest, "amount1"), 0),
			intFromAny(firstAny(latest, "amount2"), 0),
		)
	}

	payload := map[string]any{
		"event":       eventName,
		"record_type": "feed",
		"source":      "petkit-cloud",
		"raw":         latest,
	}
	if latestTS.IsZero() {
		latestTS = time.Now().UTC()
	}
	if eventID := stringFromAny(firstAny(latest, "eventId"), ""); eventID != "" {
		payload["event_id"] = eventID
	}
	if value := firstAny(latest, "amount"); value != nil {
		payload["amount"] = intFromAny(value, 0)
	}
	if value := firstAny(latest, "amount1"); value != nil {
		payload["amount1"] = intFromAny(value, 0)
	}
	if value := firstAny(latest, "amount2"); value != nil {
		payload["amount2"] = intFromAny(value, 0)
	}
	if state := mapFromAny(latest["state"]); state != nil {
		if value := firstAny(state, "realAmount"); value != nil {
			payload["real_amount"] = intFromAny(value, 0)
		}
		if value := firstAny(state, "realAmount1"); value != nil {
			payload["real_amount1"] = intFromAny(value, 0)
		}
		if value := firstAny(state, "realAmount2"); value != nil {
			payload["real_amount2"] = intFromAny(value, 0)
		}
		if value := firstAny(state, "result"); value != nil {
			payload["result"] = intFromAny(value, 0)
		}
	}

	return &DeviceOccurredEvent{
		Key:     key,
		TS:      latestTS,
		Payload: payload,
	}
}

func petkitRecordTimestamp(value any) time.Time {
	switch raw := value.(type) {
	case int:
		return unixPetkitTime(int64(raw))
	case int64:
		return unixPetkitTime(raw)
	case float64:
		return unixPetkitTime(int64(raw))
	case string:
		number, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return time.Time{}
		}
		return unixPetkitTime(number)
	default:
		return time.Time{}
	}
}

func unixPetkitTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	if value >= 1_000_000_000_000 {
		return time.UnixMilli(value).UTC()
	}
	return time.Unix(value, 0).UTC()
}
