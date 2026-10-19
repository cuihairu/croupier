//go:build !no_go_plugin

package pluginhost

import (
	"context"
	"time"

	"github.com/hashicorp/go-plugin"

	"github.com/cuihairu/croupier/agents/synthetics/pluginhost/protocol"
	"google.golang.org/grpc"
)

// CheckInput is the host→plugin check call payload.
type CheckInput struct {
	// Target is the config target identity, informational.
	Target string
	// Timeout is the remaining host-side deadline.
	Timeout time.Duration
	// PayloadJSON is the optional plugin-specific payload.
	PayloadJSON string
}

// CheckOutput is the plugin→host check result; the host maps it onto the
// shared probe result so plugin outcomes traverse the existing
// AlertEvent / probe_events chain unchanged.
type CheckOutput struct {
	OK      bool
	Status  int
	Latency time.Duration
	Error   string
	Detail  string
	At      time.Time
}

// Probe is the language-neutral contract a probe plugin implements. Go
// plugins embed it via Serve; non-Go plugins implement the generated
// ProbePlugin gRPC service over the same contract.
type Probe interface {
	Describe(ctx context.Context) (name, version string, err error)
	Check(ctx context.Context, in CheckInput) (CheckOutput, error)
}

// probeGRPCPlugin adapts Probe to the go-plugin gRPC interface for both
// the server (plugin) and client (host) sides.
type probeGRPCPlugin struct {
	plugin.Plugin
	impl Probe
}

func (p *probeGRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	protocol.RegisterProbePluginServer(s, &probeServer{impl: p.impl})
	return nil
}

func (p *probeGRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &probeClient{api: protocol.NewProbePluginClient(c)}, nil
}

// probeServer serves the generated contract on the plugin side.
type probeServer struct {
	protocol.UnimplementedProbePluginServer
	impl Probe
}

func (s *probeServer) Describe(ctx context.Context, _ *protocol.DescribeRequest) (*protocol.DescribeResponse, error) {
	name, version, err := s.impl.Describe(ctx)
	if err != nil {
		return nil, err
	}
	return &protocol.DescribeResponse{Name: name, Version: version}, nil
}

func (s *probeServer) Check(ctx context.Context, req *protocol.CheckRequest) (*protocol.CheckResult, error) {
	out, err := s.impl.Check(ctx, CheckInput{
		Target:      req.GetTarget(),
		Timeout:     time.Duration(req.GetTimeoutMs()) * time.Millisecond,
		PayloadJSON: req.GetPayloadJson(),
	})
	if err != nil {
		return nil, err
	}
	return &protocol.CheckResult{
		Ok:        out.OK,
		Status:    int32(out.Status),
		LatencyMs: out.Latency.Milliseconds(),
		Error:     out.Error,
		Detail:    out.Detail,
		AtUnixMs:  out.At.UnixMilli(),
	}, nil
}

// probeClient consumes the generated contract on the host side.
type probeClient struct {
	api protocol.ProbePluginClient
}

func (c *probeClient) Describe(ctx context.Context) (string, string, error) {
	resp, err := c.api.Describe(ctx, &protocol.DescribeRequest{})
	if err != nil {
		return "", "", err
	}
	return resp.GetName(), resp.GetVersion(), nil
}

func (c *probeClient) Check(ctx context.Context, in CheckInput) (CheckOutput, error) {
	resp, err := c.api.Check(ctx, &protocol.CheckRequest{
		Target:      in.Target,
		TimeoutMs:   in.Timeout.Milliseconds(),
		PayloadJson: in.PayloadJSON,
	})
	if err != nil {
		return CheckOutput{}, err
	}
	return CheckOutput{
		OK:      resp.GetOk(),
		Status:  int(resp.GetStatus()),
		Latency: time.Duration(resp.GetLatencyMs()) * time.Millisecond,
		Error:   resp.GetError(),
		Detail:  resp.GetDetail(),
		At:      time.UnixMilli(resp.GetAtUnixMs()),
	}, nil
}

// Serve serves a Go plugin implementing Probe over the go-plugin local
// gRPC mode. Plugin authors call this from their main function.
func Serve(impl Probe) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         map[string]plugin.Plugin{PluginName: &probeGRPCPlugin{impl: impl}},
		GRPCServer:      plugin.DefaultGRPCServer,
	})
}
