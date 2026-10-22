package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	transportcore "github.com/cuihairu/croupier/core/transport"
	tcptr "github.com/cuihairu/croupier/core/transport/tcp"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
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
	SendMetricEvent(ctx context.Context, reqBody []byte) error
	SendAlertEvent(ctx context.Context, reqBody []byte) error
	// PullProbePolicy 采样策略下行拉取（request-response；评审简档 §13.3 C）。
	PullProbePolicy(ctx context.Context, reqBody []byte) ([]byte, error)
	// PullProbeDBTargets db 探针目标下行拉取（R4：server 按归属派生，
	// request-response；version = 目标集内容 hash，空集也有 hash）。
	PullProbeDBTargets(ctx context.Context, reqBody []byte) ([]byte, error)
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

func newControlClient(kind, addr string, tlsCfg *tlsutil.ClientTLSConfig) (controlClient, error) {
	// Only TCP transport supported
	client, err := tcptr.NewClient(controlTCPConfig(addr, tlsCfg))
	if err != nil {
		return nil, err
	}
	return &tcpControlClient{client: client}, nil
}

// newMuxControlClient creates a bidirectional TCP session using MuxConn.
// The localHandler processes inbound requests from the Server (e.g., Invoke, StartTask).
func newMuxControlClient(addr string, localHandler transportcore.Handler, tlsCfg *tlsutil.ClientTLSConfig) (*muxControlClient, error) {
	conn, err := tcptr.Dial(controlTCPConfig(addr, tlsCfg))
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

func (c *tcpControlClient) SendMetricEvent(ctx context.Context, reqBody []byte) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("tcp control client not initialized")
	}
	_, _, err := c.client.Call(ctx, protocol.MsgMetricEvent, reqBody)
	return err
}

// SendAlertEvent 单发探测/告警流事件（review §11 已决 2：独立 QoS 通道，
// 不复用指标批量面）。
func (c *tcpControlClient) SendAlertEvent(ctx context.Context, reqBody []byte) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("tcp control client not initialized")
	}
	_, _, err := c.client.Call(ctx, protocol.MsgAlertEvent, reqBody)
	return err
}

// PullProbePolicy 拉取采样策略下行配置（MsgProbePolicyRequest/Response；
// version 空串 = 未发布，agent 保持现状）。
func (c *tcpControlClient) PullProbePolicy(ctx context.Context, reqBody []byte) ([]byte, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("tcp control client not initialized")
	}
	_, respData, err := c.client.Call(ctx, protocol.MsgProbePolicyRequest, reqBody)
	return respData, err
}

// PullProbeDBTargets 拉取 server 派生的 db 探针目标（R4：按归属 agent 派生，
// 拉取即真值；version = 目标集内容 hash，空集也有 hash）。
func (c *tcpControlClient) PullProbeDBTargets(ctx context.Context, reqBody []byte) ([]byte, error) {
	if c == nil || c.client == nil {
		return nil, fmt.Errorf("tcp control client not initialized")
	}
	_, respData, err := c.client.Call(ctx, protocol.MsgProbeDBTargetsRequest, reqBody)
	return respData, err
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
	return c != nil && c.mux != nil && !c.mux.IsClosed()
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

func (c *muxControlClient) SendMetricEvent(ctx context.Context, reqBody []byte) error {
	if c == nil || c.mux == nil {
		return fmt.Errorf("mux control client not initialized")
	}
	return c.mux.Send(ctx, protocol.MsgMetricEvent, reqBody)
}

// SendAlertEvent 单发探测/告警流事件（流事件走 Send 不等响应）。
func (c *muxControlClient) SendAlertEvent(ctx context.Context, reqBody []byte) error {
	if c == nil || c.mux == nil {
		return fmt.Errorf("mux control client not initialized")
	}
	return c.mux.Send(ctx, protocol.MsgAlertEvent, reqBody)
}

// PullProbePolicy 拉取采样策略下行配置（request-response）。
func (c *muxControlClient) PullProbePolicy(ctx context.Context, reqBody []byte) ([]byte, error) {
	if c == nil || c.mux == nil {
		return nil, fmt.Errorf("mux control client not initialized")
	}
	_, respData, err := c.mux.Call(ctx, protocol.MsgProbePolicyRequest, reqBody)
	return respData, err
}

// PullProbeDBTargets 拉取 server 派生的 db 探针目标（request-response）。
func (c *muxControlClient) PullProbeDBTargets(ctx context.Context, reqBody []byte) ([]byte, error) {
	if c == nil || c.mux == nil {
		return nil, fmt.Errorf("mux control client not initialized")
	}
	_, respData, err := c.mux.Call(ctx, protocol.MsgProbeDBTargetsRequest, reqBody)
	return respData, err
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

func controlTCPConfig(addr string, tlsCfg *tlsutil.ClientTLSConfig) *tcptr.Config {
	cfg := &tcptr.Config{
		Address:        normalizeTCPAddr(addr),
		Insecure:       tlsCfg == nil,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    30 * time.Second,
		SendTimeout:    10 * time.Second,
	}
	if tlsCfg != nil {
		cfg.CertFile = tlsCfg.CertFile
		cfg.KeyFile = tlsCfg.KeyFile
		cfg.CAFile = tlsCfg.CAFile
		cfg.ServerName = tlsCfg.ServerName
		cfg.InsecureSkipVerify = tlsCfg.InsecureSkipVerify
	}
	return cfg
}
