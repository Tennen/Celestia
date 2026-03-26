package coreapi

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/chentianyu/celestia/internal/pluginapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	serviceName = "celestia.core.v1.ConfigService"
	EnvCoreAddr = "CELESTIA_CORE_ADDR"
)

type PersistPluginConfigRequest struct {
	PluginID string         `json:"plugin_id"`
	Config   map[string]any `json:"config"`
}

type PersistPluginConfigResponse struct {
	OK bool `json:"ok"`
}

type ConfigServiceServer interface {
	PersistPluginConfig(context.Context, *structpb.Struct) (*structpb.Struct, error)
}

type ConfigServiceClient interface {
	PersistPluginConfig(context.Context, *structpb.Struct, ...grpc.CallOption) (*structpb.Struct, error)
}

type configServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewConfigServiceClient(cc grpc.ClientConnInterface) ConfigServiceClient {
	return &configServiceClient{cc: cc}
}

func (c *configServiceClient) PersistPluginConfig(ctx context.Context, in *structpb.Struct, opts ...grpc.CallOption) (*structpb.Struct, error) {
	out := new(structpb.Struct)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/PersistPluginConfig", in, out, opts...)
	return out, err
}

func RegisterConfigServiceServer(registrar grpc.ServiceRegistrar, server ConfigServiceServer) {
	registrar.RegisterService(&ConfigServiceDesc, server)
}

var ConfigServiceDesc = grpc.ServiceDesc{
	ServiceName: serviceName,
	HandlerType: (*ConfigServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "PersistPluginConfig", Handler: unaryHandlerPersistPluginConfig},
	},
}

func unaryHandlerPersistPluginConfig(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(structpb.Struct)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConfigServiceServer).PersistPluginConfig(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/PersistPluginConfig"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(ConfigServiceServer).PersistPluginConfig(ctx, req.(*structpb.Struct))
	}
	return interceptor(ctx, req, info, handler)
}

func PersistPluginConfig(ctx context.Context, pluginID string, config map[string]any) error {
	addr := strings.TrimSpace(os.Getenv(EnvCoreAddr))
	if addr == "" {
		return errors.New("core config service address is not available")
	}
	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	payload, err := pluginapi.EncodeStruct(PersistPluginConfigRequest{
		PluginID: pluginID,
		Config:   config,
	})
	if err != nil {
		return err
	}
	resp, err := NewConfigServiceClient(conn).PersistPluginConfig(ctx, payload)
	if err != nil {
		return err
	}
	var out PersistPluginConfigResponse
	if err := pluginapi.DecodeStruct(resp, &out); err != nil {
		return err
	}
	if !out.OK {
		return errors.New("core config service rejected plugin config persistence")
	}
	return nil
}
