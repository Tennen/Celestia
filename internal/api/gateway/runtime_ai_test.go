package gateway

import (
	"errors"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestBuildAIDeviceCatalog_UsesAliasAndCommandMetadata(t *testing.T) {
	device := models.Device{
		ID:   "petkit:feeder:1",
		Name: "Pet Feeder",
	}
	view := models.DeviceView{
		Device: models.Device{
			ID:          device.ID,
			Name:        "Kitchen Feeder",
			DefaultName: "Pet Feeder",
		},
		Controls: []models.DeviceControl{{
			ID:           "feed-once",
			Kind:         models.DeviceControlKindAction,
			Label:        "Feed Now",
			DefaultLabel: "Feed Once",
		}},
	}
	minPortions := 1.0
	stepPortions := 1.0
	specs := []models.DeviceControlSpec{{
		ID:    "feed-once",
		Kind:  models.DeviceControlKindAction,
		Label: "Feed Once",
		Command: &models.DeviceControlCommand{
			Action: "feed_once",
			Params: map[string]any{"portions": 1},
			ParamsSpec: []models.DeviceCommandParamSpec{{
				Name:    "portions",
				Type:    models.DeviceCommandParamTypeNumber,
				Default: 1,
				Min:     &minPortions,
				Step:    &stepPortions,
			}},
		},
	}}

	catalog := buildAIDeviceCatalog(device, view, specs)
	if catalog.device.Name != "Kitchen Feeder" {
		t.Fatalf("device name = %q, want Kitchen Feeder", catalog.device.Name)
	}
	if len(catalog.device.Aliases) != 1 || catalog.device.Aliases[0] != "Pet Feeder" {
		t.Fatalf("device aliases = %#v, want [Pet Feeder]", catalog.device.Aliases)
	}
	if len(catalog.device.Commands) != 1 {
		t.Fatalf("commands len = %d, want 1", len(catalog.device.Commands))
	}
	command := catalog.device.Commands[0]
	if command.Name != "Feed Now" {
		t.Fatalf("command name = %q, want Feed Now", command.Name)
	}
	if !containsAIName(command.Aliases, "Feed Once") || !containsAIName(command.Aliases, "feed_once") || !containsAIName(command.Aliases, "feed-once") {
		t.Fatalf("command aliases = %#v, missing expected aliases", command.Aliases)
	}
	if len(command.Params) != 1 || command.Params[0].Name != "portions" {
		t.Fatalf("command params = %#v, want portions", command.Params)
	}
	if command.Defaults != nil {
		t.Fatalf("command defaults = %#v, want nil because default is represented by the param schema", command.Defaults)
	}
}

func TestBuildAIDeviceCatalog_SkipsSharedActionAlias(t *testing.T) {
	device := models.Device{ID: "hikvision:camera:1", Name: "Front Door"}
	view := models.DeviceView{
		Device: models.Device{ID: device.ID, Name: "Front Door"},
		Controls: []models.DeviceControl{
			{ID: "ptz-up", Kind: models.DeviceControlKindAction, Label: "PTZ Up"},
			{ID: "ptz-down", Kind: models.DeviceControlKindAction, Label: "PTZ Down"},
		},
	}
	specs := []models.DeviceControlSpec{
		{
			ID:    "ptz-up",
			Kind:  models.DeviceControlKindAction,
			Label: "PTZ Up",
			Command: &models.DeviceControlCommand{
				Action: "ptz_move",
				Params: map[string]any{"direction": "up"},
			},
		},
		{
			ID:    "ptz-down",
			Kind:  models.DeviceControlKindAction,
			Label: "PTZ Down",
			Command: &models.DeviceControlCommand{
				Action: "ptz_move",
				Params: map[string]any{"direction": "down"},
			},
		},
	}

	catalog := buildAIDeviceCatalog(device, view, specs)
	for _, command := range catalog.device.Commands {
		if containsAIName(command.Aliases, "ptz_move") {
			t.Fatalf("shared action alias leaked into command aliases: %#v", command.Aliases)
		}
	}
}

func TestBuildAIDeviceCatalog_SkipsDisabledControls(t *testing.T) {
	device := models.Device{ID: "hikvision:camera:1", Name: "Front Door"}
	view := models.DeviceView{
		Device: models.Device{ID: device.ID, Name: "Front Door"},
		Controls: []models.DeviceControl{
			{
				ID:             "ptz-up",
				Kind:           models.DeviceControlKindAction,
				Label:          "PTZ Up",
				Disabled:       true,
				DisabledReason: "configure cloud.username and cloud.password to enable Ezviz PTZ control",
			},
		},
	}
	specs := []models.DeviceControlSpec{
		{
			ID:             "ptz-up",
			Kind:           models.DeviceControlKindAction,
			Label:          "PTZ Up",
			Disabled:       true,
			DisabledReason: "configure cloud.username and cloud.password to enable Ezviz PTZ control",
			Command: &models.DeviceControlCommand{
				Action: "ptz_move",
				Params: map[string]any{"direction": "up"},
			},
		},
	}

	catalog := buildAIDeviceCatalog(device, view, specs)
	if len(catalog.device.Commands) != 0 {
		t.Fatalf("commands len = %d, want 0 for disabled controls", len(catalog.device.Commands))
	}
}

func TestResolveAIDevice_AmbiguousAlias(t *testing.T) {
	catalogs := []aiDeviceCatalog{
		{device: AIDevice{ID: "dev-1", Name: "Kitchen Lamp"}, model: models.Device{Room: "Kitchen"}},
		{device: AIDevice{ID: "dev-2", Name: "Kitchen Lamp"}, model: models.Device{Room: "Kitchen"}},
	}

	_, err := resolveAIDevice(catalogs, "kitchen lamp")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}

	var ambiguous *AmbiguousReferenceError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected AmbiguousReferenceError, got %T", err)
	}
	if ambiguous.Field != "device" {
		t.Fatalf("ambiguous field = %q, want device", ambiguous.Field)
	}
	if len(ambiguous.Matches) != 2 {
		t.Fatalf("matches len = %d, want 2", len(ambiguous.Matches))
	}
	if ambiguous.Matches[0].Room != "Kitchen" {
		t.Fatalf("room = %q, want Kitchen", ambiguous.Matches[0].Room)
	}
}

func TestResolveAICommandAcrossCatalogs_ByCommandAlias(t *testing.T) {
	catalogs := []aiDeviceCatalog{
		newAITestCatalog("dev-1", "Kitchen Lamp", "Kitchen", "Main Power", []string{"power"}, "set_power"),
		newAITestCatalog("dev-2", "Desk Fan", "Office", "Oscillate", []string{"oscillate"}, "set_oscillate"),
	}

	target, err := resolveAICommandAcrossCatalogs(catalogs, "power")
	if err != nil {
		t.Fatalf("resolveAICommandAcrossCatalogs() error = %v", err)
	}
	if target.catalog.device.ID != "dev-1" {
		t.Fatalf("device id = %q, want dev-1", target.catalog.device.ID)
	}
	if target.command.view.Name != "Main Power" {
		t.Fatalf("command name = %q, want Main Power", target.command.view.Name)
	}
}

func TestResolveAICommandAcrossCatalogs_Ambiguous(t *testing.T) {
	catalogs := []aiDeviceCatalog{
		newAITestCatalog("dev-1", "Kitchen Lamp", "Kitchen", "Power", []string{"power"}, "set_power"),
		newAITestCatalog("dev-2", "Desk Lamp", "Office", "Power", []string{"power"}, "set_power"),
	}

	_, err := resolveAICommandAcrossCatalogs(catalogs, "power")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}

	var ambiguous *AmbiguousReferenceError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected AmbiguousReferenceError, got %T", err)
	}
	if ambiguous.Field != "command" {
		t.Fatalf("field = %q, want command", ambiguous.Field)
	}
	if len(ambiguous.Matches) != 2 {
		t.Fatalf("matches len = %d, want 2", len(ambiguous.Matches))
	}
}

func TestResolveAICommandInScope_ByRoom(t *testing.T) {
	catalogs := []aiDeviceCatalog{
		newAITestCatalog("dev-1", "Kitchen Lamp", "Kitchen", "Power", []string{"power"}, "set_power"),
		newAITestCatalog("dev-2", "Desk Lamp", "Office", "Power", []string{"power"}, "set_power"),
	}

	target, err := resolveAICommandInScope(catalogs, "Kitchen", "power")
	if err != nil {
		t.Fatalf("resolveAICommandInScope() error = %v", err)
	}
	if target.catalog.device.Name != "Kitchen Lamp" {
		t.Fatalf("device name = %q, want Kitchen Lamp", target.catalog.device.Name)
	}
}

func TestResolveAICommandInScope_AmbiguousInRoom(t *testing.T) {
	catalogs := []aiDeviceCatalog{
		newAITestCatalog("dev-1", "Kitchen Lamp", "Kitchen", "Power", []string{"power"}, "set_power"),
		newAITestCatalog("dev-2", "Kitchen Fan", "Kitchen", "Power", []string{"power"}, "set_power"),
	}

	_, err := resolveAICommandInScope(catalogs, "Kitchen", "power")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}

	var ambiguous *AmbiguousReferenceError
	if !errors.As(err, &ambiguous) {
		t.Fatalf("expected AmbiguousReferenceError, got %T", err)
	}
	if ambiguous.Field != "target" {
		t.Fatalf("field = %q, want target", ambiguous.Field)
	}
	if ambiguous.Value != "Kitchen.power" {
		t.Fatalf("value = %q, want Kitchen.power", ambiguous.Value)
	}
}

func TestAIResolvedCommandBuildRequest_ValidatesAndCoerces(t *testing.T) {
	command := aiResolvedCommand{
		view: AICommand{
			Name: "Feed Once",
			Params: []AICommandParam{{
				Name:    "portions",
				Type:    models.DeviceCommandParamTypeNumber,
				Default: 1,
			}},
		},
		kind: models.DeviceControlKindAction,
		command: &models.DeviceControlCommand{
			Action: "feed_once",
			Params: map[string]any{"portions": 1},
		},
	}

	action, params, err := command.buildRequest(map[string]any{"Portions": "2"})
	if err != nil {
		t.Fatalf("buildRequest() error = %v", err)
	}
	if action != "feed_once" {
		t.Fatalf("action = %q, want feed_once", action)
	}
	if value, ok := params["portions"].(float64); !ok || value != 2 {
		t.Fatalf("params = %#v, want portions=2", params)
	}

	if _, _, err := command.buildRequest(map[string]any{"unknown": 1}); err == nil {
		t.Fatal("expected unsupported parameter error, got nil")
	}
}

func TestAIResolvedToggleBuildRequest_RequiresOn(t *testing.T) {
	command := aiResolvedCommand{
		view: AICommand{Name: "Power"},
		kind: models.DeviceControlKindToggle,
		onCommand: &models.DeviceControlCommand{
			Action: "set_power",
			Params: map[string]any{"on": true},
		},
		offCommand: &models.DeviceControlCommand{
			Action: "set_power",
			Params: map[string]any{"on": false},
		},
	}

	action, params, err := command.buildRequest(map[string]any{"ON": "true"})
	if err != nil {
		t.Fatalf("toggle buildRequest() error = %v", err)
	}
	if action != "set_power" {
		t.Fatalf("action = %q, want set_power", action)
	}
	if on, ok := params["on"].(bool); !ok || !on {
		t.Fatalf("params = %#v, want on=true", params)
	}

	if _, _, err := command.buildRequest(map[string]any{"value": true}); err == nil {
		t.Fatal("expected unsupported parameter error, got nil")
	}
}

func containsAIName(values []string, want string) bool {
	for _, value := range values {
		if normalizeAIRef(value) == normalizeAIRef(want) {
			return true
		}
	}
	return false
}

func newAITestCatalog(deviceID, deviceName, room, commandName string, aliases []string, action string) aiDeviceCatalog {
	return aiDeviceCatalog{
		device: AIDevice{
			ID:   deviceID,
			Name: deviceName,
		},
		model: models.Device{
			ID:   deviceID,
			Name: deviceName,
			Room: room,
		},
		commands: []aiResolvedCommand{{
			view: AICommand{
				Name:    commandName,
				Aliases: aliases,
				Action:  action,
			},
			kind: models.DeviceControlKindAction,
			command: &models.DeviceControlCommand{
				Action: action,
			},
		}},
	}
}
