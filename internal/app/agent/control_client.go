package agent

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
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
	SendTaskEvent(ctx context.Context, reqBody []byte) error
}

// tcpControlClient wraps a simple TCP request-response client (no multiplexing).
type tcpControlClient struct {
	client *tcptr.Client
}

// muxControlClient wraps a MuxConn for bidirectional TCP session.
// It runs a read loop in the background and supports both outbound
// requests (Register, Heartbeat, TaskEvent) and inbound requests from Server
// (Invoke, StartTask, Ops).
type muxControlClient struct {
	mux    *tcptr.MuxConn
	cancel context.CancelFunc
}

func newControlClient(kind, addr string) (controlClient, error) {
	// Only TCP transport supported
	client, err := tcptr.NewClient(&tcptr.Config{Address: addr, Insecure: true})
	if err != nil {
		return nil, err
	}
	return &tcpControlClient{client: client}, nil
}

// newMuxControlClient creates a bidirectional TCP session using MuxConn.
// The localHandler processes inbound requests from the Server (e.g., Invoke, StartTask).
func newMuxControlClient(addr string, localHandler transportcore.Handler) (*muxControlClient, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.Dial("tcp", normalizeTCPAddr(addr))
	if err != nil {
		return nil, fmt.Errorf("dial upstream %s: %w", addr, err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	muxCfg := &tcptr.Config{
		RecvTimeout: 30 * time.Second,
		SendTimeout: 10 * time.Second,
	}

	mux := tcptr.NewMuxConn(conn, muxCfg, localHandler)

	// Start the read loop in background
	go func() {
		if err := mux.Run(ctx); err != nil {
			_ = err // connection ended
		}
	}()

	return &muxControlClient{
		mux:    mux,
		cancel: cancel,
	}, nil
}

// --- tcpControlClient ---

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

func (c *tcpControlClient) SendTaskEvent(ctx context.Context, reqBody []byte) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("tcp control client not initialized")
	}
	_, _, err := c.client.Call(ctx, protocol.MsgTaskEvent, reqBody)
	return err
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

// --- muxControlClient ---

func (c *muxControlClient) Connected() bool {
	return c != nil && c.mux != nil
}

func (c *muxControlClient) Close() error {
	if c == nil {
		return nil
	}
	if c.cancel != nil {
		c.cancel()
	}
	if c.mux != nil {
		return c.mux.Close()
	}
	return nil
}

func (c *muxControlClient) Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	resp := &agentv1.RegisterResponse{}
	if err := c.call(ctx, protocol.MsgRegisterRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *muxControlClient) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	resp := &agentv1.HeartbeatResponse{}
	if err := c.call(ctx, protocol.MsgHeartbeatRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *muxControlClient) RegisterCapabilities(ctx context.Context, req *agentv1.RegisterCapabilitiesRequest) (*agentv1.RegisterCapabilitiesResponse, error) {
	resp := &agentv1.RegisterCapabilitiesResponse{}
	if err := c.call(ctx, protocol.MsgRegisterCapabilitiesReq, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *muxControlClient) SendTaskEvent(ctx context.Context, reqBody []byte) error {
	if c == nil || c.mux == nil {
		return fmt.Errorf("mux control client not initialized")
	}
	return c.mux.Send(ctx, protocol.MsgTaskEvent, reqBody)
}

func (c *muxControlClient) call(ctx context.Context, msgID uint32, req proto.Message, resp proto.Message) error {
	if c == nil || c.mux == nil {
		return fmt.Errorf("mux control client not initialized")
	}
	data, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	_, respData, err := c.mux.Call(ctx, msgID, data)
	if err != nil {
		return err
	}
	if err := proto.Unmarshal(respData, resp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}
	return nil
}

// normalizeTCPAddr strips the tcp:// prefix from an address.
func normalizeTCPAddr(addr string) string {
	return strings.TrimPrefix(strings.TrimSpace(addr), "tcp://")
}
