package svc

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

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	fe := pgproto3.NewFrontend(conn, conn)
	startup := &pgproto3.StartupMessage{ProtocolVersion: 196608, Parameters: map[string]string{"user": "u", "database": "game_x"}}
	fe.Send(startup)
	require.NoError(t, fe.Flush())

	for i := 0; i < 5; i++ {
		msg, err := fe.Receive()
		if err != nil {
			t.Fatalf("receive %d: %v", i, err)
		}
		t.Logf("msg %d: %T %+v", i, msg, msg)
		if _, ok := msg.(*pgproto3.ReadyForQuery); ok {
			break
		}
	}

	// simple query
	fe.Send(&pgproto3.Query{String: ";"})
	require.NoError(t, fe.Flush())
	for i := 0; i < 5; i++ {
		msg, err := fe.Receive()
		if err != nil {
			t.Fatalf("q receive %d: %v", i, err)
		}
		t.Logf("q msg %d: %T", i, msg)
		if _, ok := msg.(*pgproto3.ReadyForQuery); ok {
			break
		}
	}

}
