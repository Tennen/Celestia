package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

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
	BluetoothStateNoState      = 0
	BluetoothStateNotConnected = 1
	BluetoothStateConnecting   = 2
	BluetoothStateConnected    = 3
	BluetoothStateError        = 4
)
