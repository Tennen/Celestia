package slash

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/core/audit"
	"github.com/chentianyu/celestia/internal/core/control"
	"github.com/chentianyu/celestia/internal/core/policy"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
)

type fakeCommandExecutor struct {
	calls []struct {
		device models.Device
		req    models.CommandRequest
	}
}

func (f *fakeCommandExecutor) ExecuteCommand(_ context.Context, device models.Device, req models.CommandRequest) (models.CommandResponse, error) {
	f.calls = append(f.calls, struct {
		device models.Device
		req    models.CommandRequest
	}{device: device, req: req})
	return models.CommandResponse{Accepted: true, Message: "accepted"}, nil
}

func newSlashHomeTestService(t *testing.T) (*Service, *fakeCommandExecutor) {
	t.Helper()
	ctx := context.Background()
	store, err := sqlitestore.New(filepath.Join(t.TempDir(), "celestia.db"))
	if err != nil {
		t.Fatalf("sqlite.New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}
	registrySvc := registry.New(store)
	stateSvc := state.New(store)
	device := models.Device{
		ID:             "xiaomi:light:kitchen",
		PluginID:       "xiaomi",
		VendorDeviceID: "light-kitchen",
		Kind:           models.DeviceKindLight,
		Name:           "Kitchen Light",
		Online:         true,
		Metadata: map[string]any{
			"controls": []models.DeviceControlSpec{
				{
					ID:       "power",
					Kind:     models.DeviceControlKindToggle,
					Label:    "Power",
					StateKey: "power",
					OnCommand: &models.DeviceControlCommand{
						Action: "set_power",
						Params: map[string]any{"on": true},
					},
					OffCommand: &models.DeviceControlCommand{
						Action: "set_power",
						Params: map[string]any{"on": false},
					},
				},
				{
					ID:       "brightness",
					Kind:     models.DeviceControlKindNumber,
					Label:    "Brightness",
					StateKey: "brightness",
					Command: &models.DeviceControlCommand{
						Action:     "set_brightness",
						ValueParam: "value",
					},
				},
			},
		},
	}
	if err := registrySvc.Upsert(ctx, []models.Device{device}); err != nil {
		t.Fatalf("registry.Upsert() error = %v", err)
	}
	if err := stateSvc.Upsert(ctx, []models.DeviceStateSnapshot{{
		DeviceID: device.ID,
		PluginID: device.PluginID,
		TS:       time.Now().UTC(),
		State: map[string]any{
			"power":      true,
			"brightness": 30,
		},
	}}); err != nil {
		t.Fatalf("state.Upsert() error = %v", err)
	}
	executor := &fakeCommandExecutor{}
	svc := New(store, registrySvc, stateSvc, control.New(), policy.New(), audit.New(store), nil, nil)
	svc.SetCommandExecutor(executor)
	return svc, executor
}

func TestRunHomeListShowsDeviceControls(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSlashHomeTestService(t)

	result, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: "/home list kitchen", Actor: "test"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	for _, want := range []string{"Kitchen Light", "Power", "Brightness"} {
		if !strings.Contains(result.Output, want) {
			t.Fatalf("Run() output = %q, want %q", result.Output, want)
		}
	}
	if result.Metadata["count"] != 1 {
		t.Fatalf("metadata count = %#v, want 1", result.Metadata["count"])
	}
}

func TestRunHomeToggleDispatchesResolvedCommand(t *testing.T) {
	ctx := context.Background()
	svc, executor := newSlashHomeTestService(t)

	result, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/home "Kitchen Light" Power off`, Actor: "admin"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if !strings.Contains(result.Output, "Home command accepted") {
		t.Fatalf("Run() output = %q, want accepted text", result.Output)
	}
	if len(executor.calls) != 1 {
		t.Fatalf("executor calls = %d, want 1", len(executor.calls))
	}
	call := executor.calls[0]
	if call.device.ID != "xiaomi:light:kitchen" || call.req.Action != "set_power" {
		t.Fatalf("executor call = %#v", call)
	}
	if got, ok := call.req.Params["on"].(bool); !ok || got {
		t.Fatalf("call.req.Params[on] = %#v, want false", call.req.Params["on"])
	}
}

func TestRunHomeNumberControlDispatchesValueParam(t *testing.T) {
	ctx := context.Background()
	svc, executor := newSlashHomeTestService(t)

	_, handled, err := svc.Run(ctx, models.ProjectInputRequest{Input: `/home "Kitchen Light" Brightness 60`, Actor: "admin"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !handled {
		t.Fatal("Run() handled = false, want true")
	}
	if len(executor.calls) != 1 {
		t.Fatalf("executor calls = %d, want 1", len(executor.calls))
	}
	call := executor.calls[0]
	if call.req.Action != "set_brightness" {
		t.Fatalf("call.req.Action = %q, want set_brightness", call.req.Action)
	}
	if got, ok := call.req.Params["value"].(float64); !ok || got != 60 {
		t.Fatalf("call.req.Params[value] = %#v, want 60", call.req.Params["value"])
	}
}
