package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/transport/tcp"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type v9ServeHandler struct {
	epoch uint64
}

func (h *v9ServeHandler) Handle(ctx context.Context, msgID uint32, _ uint32, body []byte) ([]byte, error) {
	switch msgID {
	case protocol.MsgServerHelloRequest:
		return ServeHelloHandler(PeerInfo{InstanceID: "owner-v9"}, h.epoch)(ctx, body), nil
	case protocol.MsgForwardInvokeReq:
		return ServeForwardHandler(h.epoch, func(ctx context.Context, req *ForwardedInvoke) (*ForwardedResult, error) {
			return &ForwardedResult{OK: true, Payload: []byte(`{"v9":true}`)}, nil
		})(ctx, body), nil
	default:
		return nil, errors.New("v9: unexpected msg")
	}
}

func TestNewTCPDialer_DialRefusedV9(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	dial := NewTCPDialer(&tcp.Config{Insecure: true, ConnectTimeout: time.Second})
	_, err = dial(context.Background(), addr, PeerInfo{InstanceID: "caller-v9"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dial peer")
}

func TestNewTCPDialer_DefaultTimeoutsHelloFailV9(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	dial := NewTCPDialer(&tcp.Config{Insecure: true})
	_, err = dial(context.Background(), ln.Addr().String(), PeerInfo{InstanceID: "caller-v9"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "peer hello")
}

func TestNewTCPDialer_RoundTripAndCloseV9(t *testing.T) {
	srv, err := tcp.NewServer(&tcp.Config{
		Address:     "127.0.0.1:0",
		Insecure:    true,
		RecvTimeout: 5 * time.Second,
		SendTimeout: 5 * time.Second,
	}, &v9ServeHandler{epoch: 3})
	require.NoError(t, err)
	go func() { _ = srv.Serve(context.Background()) }()
	defer func() { _ = srv.Close() }()

	dial := NewTCPDialer(&tcp.Config{
		Insecure:       true,
		ConnectTimeout: 3 * time.Second,
		RecvTimeout:    3 * time.Second,
		SendTimeout:    3 * time.Second,
	})
	conn, err := dial(context.Background(), srv.Addr(), PeerInfo{InstanceID: "caller-v9", Epoch: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 3, conn.epoch)

	respBody, err := conn.send(context.Background(), protocol.MsgForwardInvokeReq,
		mustJSON(t, &ForwardedInvoke{AgentID: "ag-1", Forwarded: true, CallerEpoch: 3}))
	require.NoError(t, err)
	var result ForwardedResult
	require.NoError(t, json.Unmarshal(respBody, &result))
	assert.True(t, result.OK)

	assert.NotPanics(t, func() { conn.close() })
}
