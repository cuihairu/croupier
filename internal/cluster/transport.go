package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cuihairu/croupier/internal/transport/tcp"
	"github.com/cuihairu/croupier/pkg/protocol"
)

// helloRequest 是互联握手消息（MsgServerHelloRequest 的 JSON body）。
type helloRequest struct {
	InstanceID string `json:"instanceId"`
	Epoch      uint64 `json:"epoch"`
	Role       string `json:"role"` // 固定 "server"
}

type helloResponse struct {
	InstanceID string `json:"instanceId"`
	Epoch      uint64 `json:"epoch"`
}

// NewTCPDialer 返回基于 transport/tcp 基座的建连函数。
// TLS 配置与 Agent 链路共用 CA（server 角色证书校验在握手层复核）。
func NewTCPDialer(cfg *tcp.Config) dialFunc {
	return func(ctx context.Context, addr string, self PeerInfo) (*peerConn, error) {
		dialCfg := *cfg
		dialCfg.Address = addr
		if dialCfg.ConnectTimeout <= 0 {
			dialCfg.ConnectTimeout = 5 * time.Second
		}
		if dialCfg.RecvTimeout <= 0 {
			dialCfg.RecvTimeout = 60 * time.Second
		}
		client, err := tcp.NewClient(&dialCfg)
		if err != nil {
			return nil, fmt.Errorf("dial peer: %w", err)
		}

		conn := &peerConn{
			addr: addr,
			send: func(ctx context.Context, msgID uint32, body []byte) ([]byte, error) {
				_, respBody, err := client.Call(ctx, msgID, body)
				return respBody, err
			},
			close: func() { client.Close() },
		}

		// 握手：声明 server 角色 + 自身身份。
		hello, _ := json.Marshal(helloRequest{
			InstanceID: self.InstanceID, Epoch: self.Epoch, Role: "server",
		})
		_, respBody, err := client.Call(ctx, protocol.MsgServerHelloRequest, hello)
		if err != nil {
			client.Close()
			return nil, fmt.Errorf("peer hello: %w", err)
		}
		var resp helloResponse
		_ = json.Unmarshal(respBody, &resp)
		conn.epoch = resp.Epoch
		return conn, nil
	}
}

// ServeHelloHandler 返回互联端口的 MsgServerHelloRequest 处理：
// 校验握手角色必须是 server（Agent 证书连入会被 mTLS 层拒绝，这里是
// 应用层第二道防线），并回应自身身份。
func ServeHelloHandler(self PeerInfo, epoch uint64) func(ctx context.Context, body []byte) []byte {
	return func(ctx context.Context, body []byte) []byte {
		var req helloRequest
		if err := json.Unmarshal(body, &req); err != nil || req.Role != "server" {
			resp, _ := json.Marshal(helloResponse{})
			return resp
		}
		resp, _ := json.Marshal(helloResponse{InstanceID: self.InstanceID, Epoch: epoch})
		return resp
	}
}
