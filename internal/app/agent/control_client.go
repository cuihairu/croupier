package agent

import (
	"context"
	"fmt"

	"github.com/cuihairu/croupier/internal/nng"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

type controlClient interface {
	Connected() bool
	Close() error
	Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error)
	Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error)
	RegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error)
}

type tcpControlClient struct {
	client *tcptr.Client
}

func newControlClient(kind, addr string) (controlClient, error) {
	switch normalizeTransportKind(kind) {
	case "tcp":
		client, err := tcptr.NewClient(&tcptr.Config{Address: addr, Insecure: true})
		if err != nil {
			return nil, err
		}
		return &tcpControlClient{client: client}, nil
	default:
		client := nng.NewClient(addr)
		client.SetLogger(defaultNNGLogger{})
		if err := client.Dial(); err != nil {
			return nil, err
		}
		return client, nil
	}
}

func (c *tcpControlClient) Connected() bool {
	return c != nil && c.client != nil && !c.client.IsClosed()
}

func (c *tcpControlClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

func (c *tcpControlClient) Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	resp := &agentv1.RegisterResponse{}
	if err := c.call(ctx, protocol.MsgRegisterRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *tcpControlClient) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	resp := &agentv1.HeartbeatResponse{}
	if err := c.call(ctx, protocol.MsgHeartbeatRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *tcpControlClient) RegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	resp := &agentv1.RegisterCapabilitiesResponse{}
	if err := c.call(ctx, protocol.MsgRegisterCapabilitiesReq, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *tcpControlClient) call(ctx context.Context, msgID uint32, req proto.Message, resp proto.Message) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("tcp control client not initialized")
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	_, respData, err := c.client.Call(ctx, msgID, data)
	if err != nil {
		return err
	}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

type defaultNNGLogger struct{}

func (defaultNNGLogger) Debug(msg string, args ...any) {}
func (defaultNNGLogger) Info(msg string, args ...any)  {}
func (defaultNNGLogger) Warn(msg string, args ...any)  {}
func (defaultNNGLogger) Error(msg string, args ...any) {}
