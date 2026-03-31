package pluginmgr

import (
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginapi"
	"github.com/chentianyu/celestia/internal/storage"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestSyncDevicesRemovesStaleDevices(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "celestia.db")
	store, err := sqlitestore.New(dbPath)
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
	manager := New(store, registrySvc, stateSvc, eventbus.New())
	runtime := &managedPlugin{
		record: models.PluginInstallRecord{PluginID: "hikvision"},
		client: &fakePluginClient{
			discoveries: [][]models.Device{
				{
					{ID: "hikvision:camera:front-door", PluginID: "hikvision", Name: "Front Door"},
				},
				{
					{ID: "hikvision:camera:driveway", PluginID: "hikvision", Name: "Driveway"},
				},
			},
		},
	}

	if err := manager.syncDevices(ctx, runtime); err != nil {
		t.Fatalf("first syncDevices() error = %v", err)
	}
	if err := manager.syncDevices(ctx, runtime); err != nil {
		t.Fatalf("second syncDevices() error = %v", err)
	}

	devices, err := store.ListDevices(ctx, storage.DeviceFilter{PluginID: "hikvision"})
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("devices len = %d, want 1", len(devices))
	}
	if devices[0].ID != "hikvision:camera:driveway" {
		t.Fatalf("device id = %q, want driveway device only", devices[0].ID)
	}

	if _, ok, err := store.GetDeviceState(ctx, "hikvision:camera:front-door"); err != nil {
		t.Fatalf("GetDeviceState(old) error = %v", err)
	} else if ok {
		t.Fatal("old device state should be removed after sync reconciliation")
	}
}

type fakePluginClient struct {
	discoveries [][]models.Device
	discoverIdx int
}

func (f *fakePluginClient) GetManifest(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.Struct, error) {
	return pluginapi.EncodeStruct(models.PluginManifest{})
}

func (f *fakePluginClient) ValidateConfig(context.Context, *structpb.Struct, ...grpc.CallOption) (*structpb.Struct, error) {
	return pluginapi.EncodeStruct(map[string]any{"valid": true})
}

func (f *fakePluginClient) Setup(context.Context, *structpb.Struct, ...grpc.CallOption) (*structpb.Struct, error) {
	return pluginapi.EncodeStruct(map[string]any{"ok": true})
}

func (f *fakePluginClient) Start(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.Struct, error) {
	return pluginapi.EncodeStruct(map[string]any{"ok": true})
}

func (f *fakePluginClient) Stop(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.Struct, error) {
	return pluginapi.EncodeStruct(map[string]any{"ok": true})
}

func (f *fakePluginClient) HealthCheck(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.Struct, error) {
	return pluginapi.EncodeStruct(models.PluginHealth{})
}

func (f *fakePluginClient) DiscoverDevices(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.ListValue, error) {
	idx := f.discoverIdx
	if idx >= len(f.discoveries) {
		idx = len(f.discoveries) - 1
	}
	f.discoverIdx++
	return pluginapi.EncodeList(f.discoveries[idx])
}

func (f *fakePluginClient) ListDevices(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.ListValue, error) {
	return pluginapi.EncodeList(nil)
}

func (f *fakePluginClient) GetDeviceState(_ context.Context, in *structpb.Struct, _ ...grpc.CallOption) (*structpb.Struct, error) {
	req := struct {
		DeviceID string `json:"device_id"`
	}{}
	if err := pluginapi.DecodeStruct(in, &req); err != nil {
		return nil, err
	}
	return pluginapi.EncodeStruct(models.DeviceStateSnapshot{
		DeviceID: req.DeviceID,
		PluginID: "hikvision",
		TS:       time.Now().UTC(),
		State:    map[string]any{"connected": false},
	})
}

func (f *fakePluginClient) ExecuteCommand(context.Context, *structpb.Struct, ...grpc.CallOption) (*structpb.Struct, error) {
	return pluginapi.EncodeStruct(models.CommandResponse{})
}

func (f *fakePluginClient) StreamEvents(context.Context, *emptypb.Empty, ...grpc.CallOption) (pluginapi.PluginStreamEventsClient, error) {
	return &fakeStreamEventsClient{}, nil
}

type fakeStreamEventsClient struct {
	grpc.ClientStream
}

func (f *fakeStreamEventsClient) Recv() (*structpb.Struct, error) {
	return nil, io.EOF
}
