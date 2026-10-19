//go:build !no_go_plugin

package funcplugin

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-plugin"

	"github.com/cuihairu/croupier/agents/sidecar/funcplugin/declare"
	"github.com/cuihairu/croupier/agents/sidecar/funcplugin/protocol"
	coreplugin "github.com/cuihairu/croupier/core/pluginhost"
	"google.golang.org/grpc"
)

const (
	// PluginName is the go-plugin dispense key both sides agree on.
	PluginName = "function"
	// ProtocolVersion of the handshake. The wire contract is versioned
	// separately from the Agent↔Server TCP protocol and from the
	// synthetics probe plugin contract.
	ProtocolVersion = 1
	// CookieKey / CookieValue authenticate that the subprocess really is
	// a Croupier function adapter plugin and not an arbitrary binary
	// launched by mistake.
	CookieKey   = "CROUPIER_SIDECAR_FUNC_PLUGIN"
	CookieValue = "croupier-sidecar-function-plugin"
)

// Handshake is shared by host and plugin processes to authenticate and
// pin the protocol before any gRPC traffic.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  ProtocolVersion,
	MagicCookieKey:   CookieKey,
	MagicCookieValue: CookieValue,
}

// contract pins the function IPC contract onto the shared resident host
// engine (core/pluginhost) — the host-reuse seam of todo.md:35.
func contract() coreplugin.Contract[FuncPlugin] {
	return coreplugin.Contract[FuncPlugin]{
		Key:       PluginName,
		Handshake: Handshake,
		GRPCServer: func(impl FuncPlugin) plugin.Plugin {
			return &funcGRPCPlugin{impl: impl}
		},
		Client: func(raw any) (FuncPlugin, error) {
			p, ok := raw.(FuncPlugin)
			if !ok {
				return nil, fmt.Errorf("dispensed client does not implement FuncPlugin")
			}
			return p, nil
		},
		Describe: func(ctx context.Context, impl FuncPlugin) (string, string, error) {
			return impl.Describe(ctx)
		},
	}
}

// funcGRPCPlugin adapts FuncPlugin to the go-plugin gRPC interface for
// both the server (plugin) and client (host) sides.
type funcGRPCPlugin struct {
	plugin.Plugin
	impl FuncPlugin
}

func (p *funcGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	protocol.RegisterFunctionPluginServer(s, &funcServer{impl: p.impl})
	return nil
}

func (p *funcGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &funcClient{api: protocol.NewFunctionPluginClient(c)}, nil
}

// funcServer serves the generated contract on the plugin side.
type funcServer struct {
	protocol.UnimplementedFunctionPluginServer
	impl FuncPlugin
}

func (s *funcServer) Describe(ctx context.Context, _ *protocol.DescribeRequest) (*protocol.DescribeResponse, error) {
	name, version, err := s.impl.Describe(ctx)
	if err != nil {
		return nil, err
	}
	return &protocol.DescribeResponse{Name: name, Version: version}, nil
}

func (s *funcServer) ListFunctions(ctx context.Context, _ *protocol.ListFunctionsRequest) (*protocol.ListFunctionsResponse, error) {
	descs, err := s.impl.ListFunctions(ctx)
	if err != nil {
		return nil, err
	}
	out := &protocol.ListFunctionsResponse{Functions: make([]*protocol.FuncDescriptor, 0, len(descs))}
	for _, d := range descs {
		out.Functions = append(out.Functions, &protocol.FuncDescriptor{
			Id:           d.ID,
			Summary:      d.Summary,
			ParamSchema:  d.ParamSchema,
			ResultSchema: d.ResultSchema,
			ErrorSchema:  d.ErrorSchema,
			RiskLevel:    d.RiskLevel,
		})
	}
	return out, nil
}

func (s *funcServer) Invoke(ctx context.Context, req *protocol.InvokeRequest) (*protocol.InvokeResponse, error) {
	out, err := s.impl.Invoke(ctx, InvokeInput{
		FunctionID: req.GetFunctionId(),
		ParamsJSON: req.GetParamsJson(),
		Timeout:    time.Duration(req.GetTimeoutMs()) * time.Millisecond,
	})
	if err != nil {
		return nil, err
	}
	return &protocol.InvokeResponse{
		Ok:           out.OK,
		ResultJson:   out.ResultJSON,
		ErrorCode:    out.ErrorCode,
		ErrorMessage: out.ErrorMessage,
		AtUnixMs:     out.At.UnixMilli(),
	}, nil
}

// funcClient consumes the generated contract on the host side.
type funcClient struct {
	api protocol.FunctionPluginClient
}

func (c *funcClient) Describe(ctx context.Context) (string, string, error) {
	resp, err := c.api.Describe(ctx, &protocol.DescribeRequest{})
	if err != nil {
		return "", "", err
	}
	return resp.GetName(), resp.GetVersion(), nil
}

func (c *funcClient) ListFunctions(ctx context.Context) ([]declare.Descriptor, error) {
	resp, err := c.api.ListFunctions(ctx, &protocol.ListFunctionsRequest{})
	if err != nil {
		return nil, err
	}
	descs := make([]declare.Descriptor, 0, len(resp.GetFunctions()))
	for _, d := range resp.GetFunctions() {
		descs = append(descs, declare.Descriptor{
			ID:           d.GetId(),
			Summary:      d.GetSummary(),
			ParamSchema:  d.GetParamSchema(),
			ResultSchema: d.GetResultSchema(),
			ErrorSchema:  d.GetErrorSchema(),
			RiskLevel:    d.GetRiskLevel(),
		})
	}
	return descs, nil
}

func (c *funcClient) Invoke(ctx context.Context, in InvokeInput) (InvokeOutput, error) {
	resp, err := c.api.Invoke(ctx, &protocol.InvokeRequest{
		FunctionId: in.FunctionID,
		ParamsJson: in.ParamsJSON,
		TimeoutMs:  in.Timeout.Milliseconds(),
	})
	if err != nil {
		return InvokeOutput{}, err
	}
	return InvokeOutput{
		OK:           resp.GetOk(),
		ResultJSON:   resp.GetResultJson(),
		ErrorCode:    resp.GetErrorCode(),
		ErrorMessage: resp.GetErrorMessage(),
		At:           time.UnixMilli(resp.GetAtUnixMs()),
	}, nil
}

// Serve serves a Go function adapter plugin over the go-plugin local
// gRPC mode. Plugin authors call this from their main function.
func Serve(impl FuncPlugin) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         map[string]plugin.Plugin{PluginName: &funcGRPCPlugin{impl: impl}},
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
