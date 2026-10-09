// Package controlconn capture-agent 上行控制连接（core/register.Conn 适配
// core/transport/tcp.Client）。语义对齐 internal/app/agent 的 tcpControlClient
// （Register/Heartbeat 走 agentv1 wire），但依赖面收在 agents/*→core/*→proto
// 不变量内：不 import internal/。
package controlconn

import (
	"context"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/core/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// TLS 上行通道 TLS 配置（对齐 configs/agent.yaml outboundTLS 块语义）。
// nil / Enabled=false → Insecure 裸连。
type TLS struct {
	Enabled            bool   `yaml:"enabled" json:"enabled"`
	CertFile           string `yaml:"certFile" json:"certFile"`
	KeyFile            string `yaml:"keyFile" json:"keyFile"`
	CAFile             string `yaml:"caFile" json:"caFile"`
	ServerName         string `yaml:"serverName" json:"serverName"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify" json:"insecureSkipVerify"`
}

// Conn 上行控制连接，实现 core/register.Conn。
type Conn struct {
	client *tcp.Client
}

// Dial 建立上行控制连接（未注册；注册由 core/register.Client 驱动）。
func Dial(addr string, tlsCfg *TLS) (*Conn, error) {
	cfg := &tcp.Config{
		Address:        normalizeAddr(addr),
		Insecure:       tlsCfg == nil || !tlsCfg.Enabled,
		ConnectTimeout: 5 * time.Second,
		RecvTimeout:    30 * time.Second,
		SendTimeout:    10 * time.Second,
	}
	if tlsCfg != nil && tlsCfg.Enabled {
		cfg.CertFile = tlsCfg.CertFile
		cfg.KeyFile = tlsCfg.KeyFile
		cfg.CAFile = tlsCfg.CAFile
		cfg.ServerName = tlsCfg.ServerName
		cfg.InsecureSkipVerify = tlsCfg.InsecureSkipVerify
	}
	client, err := tcp.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("dial control %s: %w", addr, err)
	}
	return &Conn{client: client}, nil
}

// Connected 连接是否存活。
func (c *Conn) Connected() bool { return c != nil && c.client != nil && !c.client.IsClosed() }

// Close 关闭连接（幂等）。
func (c *Conn) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

// Register 上线注册。
func (c *Conn) Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	resp := &agentv1.RegisterResponse{}
	if err := c.call(ctx, protocol.MsgRegisterRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Heartbeat 存活心跳。
func (c *Conn) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	resp := &agentv1.HeartbeatResponse{}
	if err := c.call(ctx, protocol.MsgHeartbeatRequest, req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Conn) call(ctx context.Context, msgID uint32, req proto.Message, resp proto.Message) error {
	if c == nil || c.client == nil {
		return fmt.Errorf("control conn not initialized")
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

// normalizeAddr 剥 tcp:// 前缀（对齐 sidecar 配置宽容度）。
func normalizeAddr(addr string) string {
	for _, p := range []string{"tcp://", "tcp:"} {
		if len(addr) >= len(p) && addr[:len(p)] == p {
			addr = addr[len(p):]
		}
	}
	return addr
}
