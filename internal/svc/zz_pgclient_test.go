package svc

// 用 pgproto3 前端直接驱动 fakePGServer，验证其在两种场景下的 wire 行为：
//  1. failPing 未耗尽：启动阶段收到 FATAL 3D000（库不存在，与真实 PG 一致）；
//  2. failPing 已耗尽：完整握手（AuthOk + ParameterStatus + RFQ）与 simple query 应答。

import (
	"net"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/require"
)

func TestDebugPGFakeHandshake(t *testing.T) {
	s, addr := startFakePGServer(t)
	s.mu.Lock()
	s.failPing = 1
	s.mu.Unlock()

	// 场景 1：首个连接在启动阶段被拒（database does not exist）。
	rejected, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	fe1 := pgproto3.NewFrontend(rejected, rejected)
	_ = rejected.SetDeadline(time.Now().Add(3 * time.Second))
	fe1.Send(&pgproto3.StartupMessage{ProtocolVersion: 196608, Parameters: map[string]string{"user": "u", "database": "game_x"}})
	require.NoError(t, fe1.Flush())
	msg, err := fe1.Receive()
	require.NoError(t, err)
	errResp, ok := msg.(*pgproto3.ErrorResponse)
	require.True(t, ok, "expected ErrorResponse, got %T", msg)
	require.Equal(t, "3D000", errResp.Code)
	require.Contains(t, errResp.Message, "does not exist")
	_ = rejected.Close()

	// 场景 2：后续连接正常握手并应答 simple query。
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	fe := pgproto3.NewFrontend(conn, conn)
	startup := &pgproto3.StartupMessage{ProtocolVersion: 196608, Parameters: map[string]string{"user": "u", "database": "game_x"}}
	fe.Send(startup)
	require.NoError(t, fe.Flush())

	sawAuthOk := false
	sawParams := false
	for i := 0; i < 8; i++ {
		msg, err := fe.Receive()
		if err != nil {
			t.Fatalf("receive %d: %v", i, err)
		}
		switch m := msg.(type) {
		case *pgproto3.AuthenticationOk:
			sawAuthOk = true
		case *pgproto3.ParameterStatus:
			if m.Name == "standard_conforming_strings" && m.Value == "on" {
				sawParams = true
			}
		}
		if _, ok := msg.(*pgproto3.ReadyForQuery); ok {
			break
		}
	}
	require.True(t, sawAuthOk, "missing AuthenticationOk")
	require.True(t, sawParams, "missing standard_conforming_strings=on")

	// simple query
	fe.Send(&pgproto3.Query{String: ";"})
	require.NoError(t, fe.Flush())
	for i := 0; i < 5; i++ {
		msg, err := fe.Receive()
		if err != nil {
			t.Fatalf("q receive %d: %v", i, err)
		}
		if _, ok := msg.(*pgproto3.ReadyForQuery); ok {
			break
		}
	}
}
