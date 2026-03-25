package pluginapi

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const serviceName = "celestia.plugin.v1.PluginService"

type PluginServer interface {
	GetManifest(context.Context, *emptypb.Empty) (*structpb.Struct, error)
	ValidateConfig(context.Context, *structpb.Struct) (*structpb.Struct, error)
	Setup(context.Context, *structpb.Struct) (*structpb.Struct, error)
	Start(context.Context, *emptypb.Empty) (*structpb.Struct, error)
	Stop(context.Context, *emptypb.Empty) (*structpb.Struct, error)
	HealthCheck(context.Context, *emptypb.Empty) (*structpb.Struct, error)
	DiscoverDevices(context.Context, *emptypb.Empty) (*structpb.ListValue, error)
	ListDevices(context.Context, *emptypb.Empty) (*structpb.ListValue, error)
	GetDeviceState(context.Context, *structpb.Struct) (*structpb.Struct, error)
	ExecuteCommand(context.Context, *structpb.Struct) (*structpb.Struct, error)
	StreamEvents(*emptypb.Empty, PluginStreamEventsServer) error
}

type PluginClient interface {
	GetManifest(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.Struct, error)
	ValidateConfig(context.Context, *structpb.Struct, ...grpc.CallOption) (*structpb.Struct, error)
	Setup(context.Context, *structpb.Struct, ...grpc.CallOption) (*structpb.Struct, error)
	Start(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.Struct, error)
	Stop(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.Struct, error)
	HealthCheck(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.Struct, error)
	DiscoverDevices(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.ListValue, error)
	ListDevices(context.Context, *emptypb.Empty, ...grpc.CallOption) (*structpb.ListValue, error)
	GetDeviceState(context.Context, *structpb.Struct, ...grpc.CallOption) (*structpb.Struct, error)
	ExecuteCommand(context.Context, *structpb.Struct, ...grpc.CallOption) (*structpb.Struct, error)
	StreamEvents(context.Context, *emptypb.Empty, ...grpc.CallOption) (PluginStreamEventsClient, error)
}

type pluginClient struct {
	cc grpc.ClientConnInterface
}

func NewPluginClient(cc grpc.ClientConnInterface) PluginClient {
	return &pluginClient{cc: cc}
}

func (c *pluginClient) GetManifest(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*structpb.Struct, error) {
	out := new(structpb.Struct)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/GetManifest", in, out, opts...)
	return out, err
}

func (c *pluginClient) ValidateConfig(ctx context.Context, in *structpb.Struct, opts ...grpc.CallOption) (*structpb.Struct, error) {
	out := new(structpb.Struct)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/ValidateConfig", in, out, opts...)
	return out, err
}

func (c *pluginClient) Setup(ctx context.Context, in *structpb.Struct, opts ...grpc.CallOption) (*structpb.Struct, error) {
	out := new(structpb.Struct)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/Setup", in, out, opts...)
	return out, err
}

func (c *pluginClient) Start(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*structpb.Struct, error) {
	out := new(structpb.Struct)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/Start", in, out, opts...)
	return out, err
}

func (c *pluginClient) Stop(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*structpb.Struct, error) {
	out := new(structpb.Struct)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/Stop", in, out, opts...)
	return out, err
}

func (c *pluginClient) HealthCheck(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*structpb.Struct, error) {
	out := new(structpb.Struct)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/HealthCheck", in, out, opts...)
	return out, err
}

func (c *pluginClient) DiscoverDevices(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*structpb.ListValue, error) {
	out := new(structpb.ListValue)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/DiscoverDevices", in, out, opts...)
	return out, err
}

func (c *pluginClient) ListDevices(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*structpb.ListValue, error) {
	out := new(structpb.ListValue)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/ListDevices", in, out, opts...)
	return out, err
}

func (c *pluginClient) GetDeviceState(ctx context.Context, in *structpb.Struct, opts ...grpc.CallOption) (*structpb.Struct, error) {
	out := new(structpb.Struct)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/GetDeviceState", in, out, opts...)
	return out, err
}

func (c *pluginClient) ExecuteCommand(ctx context.Context, in *structpb.Struct, opts ...grpc.CallOption) (*structpb.Struct, error) {
	out := new(structpb.Struct)
	err := c.cc.Invoke(ctx, "/"+serviceName+"/ExecuteCommand", in, out, opts...)
	return out, err
}

func (c *pluginClient) StreamEvents(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (PluginStreamEventsClient, error) {
	stream, err := c.cc.NewStream(ctx, &PluginServiceDesc.Streams[0], "/"+serviceName+"/StreamEvents", opts...)
	if err != nil {
		return nil, err
	}
	client := &pluginStreamEventsClient{ClientStream: stream}
	if err := client.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := client.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return client, nil
}

type PluginStreamEventsClient interface {
	Recv() (*structpb.Struct, error)
	grpc.ClientStream
}

type pluginStreamEventsClient struct {
	grpc.ClientStream
}

func (c *pluginStreamEventsClient) Recv() (*structpb.Struct, error) {
	m := new(structpb.Struct)
	if err := c.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type PluginStreamEventsServer interface {
	Send(*structpb.Struct) error
	grpc.ServerStream
}

type pluginStreamEventsServer struct {
	grpc.ServerStream
}

func (s *pluginStreamEventsServer) Send(m *structpb.Struct) error {
	return s.ServerStream.SendMsg(m)
}

func RegisterPluginServer(registrar grpc.ServiceRegistrar, server PluginServer) {
	registrar.RegisterService(&PluginServiceDesc, server)
}

var PluginServiceDesc = grpc.ServiceDesc{
	ServiceName: serviceName,
	HandlerType: (*PluginServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "GetManifest", Handler: unaryHandlerGetManifest},
		{MethodName: "ValidateConfig", Handler: unaryHandlerValidateConfig},
		{MethodName: "Setup", Handler: unaryHandlerSetup},
		{MethodName: "Start", Handler: unaryHandlerStart},
		{MethodName: "Stop", Handler: unaryHandlerStop},
		{MethodName: "HealthCheck", Handler: unaryHandlerHealthCheck},
		{MethodName: "DiscoverDevices", Handler: unaryHandlerDiscoverDevices},
		{MethodName: "ListDevices", Handler: unaryHandlerListDevices},
		{MethodName: "GetDeviceState", Handler: unaryHandlerGetDeviceState},
		{MethodName: "ExecuteCommand", Handler: unaryHandlerExecuteCommand},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamEvents",
			Handler:       streamHandlerEvents,
			ServerStreams: true,
		},
	},
}

func unaryHandlerGetManifest(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(emptypb.Empty)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServer).GetManifest(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/GetManifest"}
	handler := func(ctx context.Context, req any) (any, error) { return srv.(PluginServer).GetManifest(ctx, req.(*emptypb.Empty)) }
	return interceptor(ctx, req, info, handler)
}

func unaryHandlerValidateConfig(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(structpb.Struct)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServer).ValidateConfig(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/ValidateConfig"}
	handler := func(ctx context.Context, req any) (any, error) { return srv.(PluginServer).ValidateConfig(ctx, req.(*structpb.Struct)) }
	return interceptor(ctx, req, info, handler)
}

func unaryHandlerSetup(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(structpb.Struct)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServer).Setup(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/Setup"}
	handler := func(ctx context.Context, req any) (any, error) { return srv.(PluginServer).Setup(ctx, req.(*structpb.Struct)) }
	return interceptor(ctx, req, info, handler)
}

func unaryHandlerStart(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(emptypb.Empty)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServer).Start(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/Start"}
	handler := func(ctx context.Context, req any) (any, error) { return srv.(PluginServer).Start(ctx, req.(*emptypb.Empty)) }
	return interceptor(ctx, req, info, handler)
}

func unaryHandlerStop(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(emptypb.Empty)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServer).Stop(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/Stop"}
	handler := func(ctx context.Context, req any) (any, error) { return srv.(PluginServer).Stop(ctx, req.(*emptypb.Empty)) }
	return interceptor(ctx, req, info, handler)
}

func unaryHandlerHealthCheck(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(emptypb.Empty)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServer).HealthCheck(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/HealthCheck"}
	handler := func(ctx context.Context, req any) (any, error) { return srv.(PluginServer).HealthCheck(ctx, req.(*emptypb.Empty)) }
	return interceptor(ctx, req, info, handler)
}

func unaryHandlerDiscoverDevices(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(emptypb.Empty)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServer).DiscoverDevices(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/DiscoverDevices"}
	handler := func(ctx context.Context, req any) (any, error) { return srv.(PluginServer).DiscoverDevices(ctx, req.(*emptypb.Empty)) }
	return interceptor(ctx, req, info, handler)
}

func unaryHandlerListDevices(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(emptypb.Empty)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServer).ListDevices(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/ListDevices"}
	handler := func(ctx context.Context, req any) (any, error) { return srv.(PluginServer).ListDevices(ctx, req.(*emptypb.Empty)) }
	return interceptor(ctx, req, info, handler)
}

func unaryHandlerGetDeviceState(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(structpb.Struct)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServer).GetDeviceState(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/GetDeviceState"}
	handler := func(ctx context.Context, req any) (any, error) { return srv.(PluginServer).GetDeviceState(ctx, req.(*structpb.Struct)) }
	return interceptor(ctx, req, info, handler)
}

func unaryHandlerExecuteCommand(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := new(structpb.Struct)
	if err := dec(req); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServer).ExecuteCommand(ctx, req)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + serviceName + "/ExecuteCommand"}
	handler := func(ctx context.Context, req any) (any, error) { return srv.(PluginServer).ExecuteCommand(ctx, req.(*structpb.Struct)) }
	return interceptor(ctx, req, info, handler)
}

func streamHandlerEvents(srv any, stream grpc.ServerStream) error {
	req := new(emptypb.Empty)
	if err := stream.RecvMsg(req); err != nil {
		return err
	}
	return srv.(PluginServer).StreamEvents(req, &pluginStreamEventsServer{ServerStream: stream})
}

