// Command e2e-agent-probe is a minimal Agent that registers against the
// Croupier Server control-plane TCP listener and verifies the handshake.
//
// It backs the CI E2E (scripts/e2e/happy-path.sh): it sends a RegisterRequest
// as the first frame, asserts the RegisterResponse carries a non-empty session
// id, then sends a Heartbeat to confirm the post-register state machine and
// session routing work end to end.
//
// Exit code 0 on success, 1 on any failure (with a diagnostic on stderr), so
// the shell harness can branch on it directly. It reuses the production TCP
// transport (internal/transport/tcp.Client) rather than re-implementing the
// wire format, so a protocol drift between probe and server is impossible.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:19090", "server control-plane TCP address")
	agentID := flag.String("agent-id", "", "agent id (required)")
	gameID := flag.String("game-id", "", "game scope id")
	env := flag.String("env", "", "environment (e.g. dev/prod)")
	version := flag.String("version", "e2e-probe-1.0", "agent version reported at register")
	ttl := flag.Uint("ttl-seconds", 60, "requested session ttl in seconds")
	insecure := flag.Bool("insecure", true, "use plain TCP (skip TLS)")
	skipHeartbeat := flag.Bool("skip-heartbeat", false, "skip the post-register heartbeat probe")
	timeout := flag.Duration("timeout", 10*time.Second, "overall probe timeout")
	flag.Parse()

	if *agentID == "" {
		fail("-agent-id is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client, err := tcp.NewClient(&tcp.Config{
		Address:        *addr,
		Insecure:       *insecure,
		ConnectTimeout: 5 * time.Second,
	})
	if err != nil {
		fail("dial %s: %v", *addr, err)
	}
	defer client.Close()

	// 1. Register — must be the first frame on the session.
	regReq := &agentv1.RegisterRequest{
		AgentId:    *agentID,
		Version:    *version,
		GameId:     *gameID,
		Env:        *env,
		TtlSeconds: uint32(*ttl),
	}
	body, err := proto.Marshal(regReq)
	if err != nil {
		fail("marshal RegisterRequest: %v", err)
	}

	respMsgID, respBody, err := client.Call(ctx, protocol.MsgRegisterRequest, body)
	if err != nil {
		fail("register call: %v", err)
	}
	if respMsgID != protocol.MsgRegisterResponse {
		fail("expected RegisterResponse (0x%06X), got 0x%06X", protocol.MsgRegisterResponse, respMsgID)
	}

	regResp := &agentv1.RegisterResponse{}
	if err := proto.Unmarshal(respBody, regResp); err != nil {
		fail("unmarshal RegisterResponse: %v", err)
	}
	if regResp.GetSessionId() == "" {
		fail("register returned empty session id")
	}
	fmt.Printf("register ok: agent=%s session=%s expire_at=%d\n",
		*agentID, regResp.GetSessionId(), regResp.GetExpireAt())

	// 2. Heartbeat — validates the post-register state machine: only after
	//    registration may a Heartbeat flow, and its agent_id must match the
	//    session-bound id (server enforces this; a mismatch is rejected).
	if !*skipHeartbeat {
		hbBody, err := proto.Marshal(&agentv1.HeartbeatRequest{AgentId: *agentID})
		if err != nil {
			fail("marshal HeartbeatRequest: %v", err)
		}
		hbMsgID, _, err := client.Call(ctx, protocol.MsgHeartbeatRequest, hbBody)
		if err != nil {
			fail("heartbeat call: %v", err)
		}
		if hbMsgID != protocol.MsgHeartbeatResponse {
			fail("expected HeartbeatResponse (0x%06X), got 0x%06X",
				protocol.MsgHeartbeatResponse, hbMsgID)
		}
		fmt.Printf("heartbeat ok: agent=%s\n", *agentID)
	}

	fmt.Println("e2e-agent-probe: PASS")
}

// fail prints a diagnostic to stderr and exits non-zero so the shell harness
// treats the probe as failed.
func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "e2e-agent-probe: FAIL — "+format+"\n", args...)
	os.Exit(1)
}
