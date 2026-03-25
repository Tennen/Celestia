package pluginruntime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/pluginapi"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type Adapter interface {
	Manifest() models.PluginManifest
	ValidateConfig(context.Context, map[string]any) error
	Setup(context.Context, map[string]any) error
	Start(context.Context) error
	Stop(context.Context) error
	HealthCheck(context.Context) models.PluginHealth
	DiscoverDevices(context.Context) ([]models.Device, []models.DeviceStateSnapshot, error)
	ListDevices(context.Context) ([]models.Device, error)
	GetDeviceState(context.Context, string) (models.DeviceStateSnapshot, error)
	ExecuteCommand(context.Context, models.CommandRequest) (models.CommandResponse, error)
	Events() <-chan models.Event
}

type Server struct {
	adapter Adapter
}

func NewServer(adapter Adapter) *Server {
	return &Server{adapter: adapter}
}

func (s *Server) GetManifest(ctx context.Context, _ *emptypb.Empty) (*structpb.Struct, error) {
	return pluginapi.EncodeStruct(s.adapter.Manifest())
}

func (s *Server) ValidateConfig(ctx context.Context, in *structpb.Struct) (*structpb.Struct, error) {
	config := map[string]any{}
	if err := pluginapi.DecodeStruct(in, &config); err != nil {
		return nil, err
	}
	resp := map[string]any{"valid": true}
	if err := s.adapter.ValidateConfig(ctx, config); err != nil {
		resp["valid"] = false
		resp["error"] = err.Error()
	}
	return pluginapi.EncodeStruct(resp)
}

func (s *Server) Setup(ctx context.Context, in *structpb.Struct) (*structpb.Struct, error) {
	config := map[string]any{}
	if err := pluginapi.DecodeStruct(in, &config); err != nil {
		return nil, err
	}
	if err := s.adapter.Setup(ctx, config); err != nil {
		return nil, err
	}
	return pluginapi.EncodeStruct(map[string]any{"ok": true})
}

func (s *Server) Start(ctx context.Context, _ *emptypb.Empty) (*structpb.Struct, error) {
	if err := s.adapter.Start(ctx); err != nil {
		return nil, err
	}
	return pluginapi.EncodeStruct(map[string]any{"ok": true})
}

func (s *Server) Stop(ctx context.Context, _ *emptypb.Empty) (*structpb.Struct, error) {
	if err := s.adapter.Stop(ctx); err != nil {
		return nil, err
	}
	return pluginapi.EncodeStruct(map[string]any{"ok": true})
}

func (s *Server) HealthCheck(ctx context.Context, _ *emptypb.Empty) (*structpb.Struct, error) {
	return pluginapi.EncodeStruct(s.adapter.HealthCheck(ctx))
}

func (s *Server) DiscoverDevices(ctx context.Context, _ *emptypb.Empty) (*structpb.ListValue, error) {
	devices, _, err := s.adapter.DiscoverDevices(ctx)
	if err != nil {
		return nil, err
	}
	return pluginapi.EncodeList(devices)
}

func (s *Server) ListDevices(ctx context.Context, _ *emptypb.Empty) (*structpb.ListValue, error) {
	devices, err := s.adapter.ListDevices(ctx)
	if err != nil {
		return nil, err
	}
	return pluginapi.EncodeList(devices)
}

func (s *Server) GetDeviceState(ctx context.Context, in *structpb.Struct) (*structpb.Struct, error) {
	req := struct {
		DeviceID string `json:"device_id"`
	}{}
	if err := pluginapi.DecodeStruct(in, &req); err != nil {
		return nil, err
	}
	state, err := s.adapter.GetDeviceState(ctx, req.DeviceID)
	if err != nil {
		return nil, err
	}
	return pluginapi.EncodeStruct(state)
}

func (s *Server) ExecuteCommand(ctx context.Context, in *structpb.Struct) (*structpb.Struct, error) {
	var req models.CommandRequest
	if err := pluginapi.DecodeStruct(in, &req); err != nil {
		return nil, err
	}
	resp, err := s.adapter.ExecuteCommand(ctx, req)
	if err != nil {
		return nil, err
	}
	return pluginapi.EncodeStruct(resp)
}

func (s *Server) StreamEvents(_ *emptypb.Empty, stream pluginapi.PluginStreamEventsServer) error {
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case event, ok := <-s.adapter.Events():
			if !ok {
				return nil
			}
			payload, err := pluginapi.EncodeStruct(event)
			if err != nil {
				return err
			}
			if err := stream.Send(payload); err != nil {
				return err
			}
		}
	}
}

func Serve(adapter Adapter) error {
	port := os.Getenv("CELESTIA_PLUGIN_PORT")
	if port == "" {
		return errors.New("CELESTIA_PLUGIN_PORT is required")
	}
	addr := "127.0.0.1:" + port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	server := grpc.NewServer()
	pluginapi.RegisterPluginServer(server, NewServer(adapter))
	fmt.Printf("plugin=%s listening=%s\n", adapter.Manifest().ID, addr)
	return server.Serve(listener)
}

func NewHealth(pluginID, version string, status models.HealthState, message string) models.PluginHealth {
	return models.PluginHealth{
		PluginID:  pluginID,
		Status:    status,
		Message:   message,
		CheckedAt: time.Now().UTC(),
		Manifest:  version,
	}
}

