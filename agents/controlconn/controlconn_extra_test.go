package controlconn

// Send*/Pull* 请求包装补测：走帧级应答服务的往返面（告警上报、探针策略、
// synthetics 目标、capture 闸门、devops 白名单）、SendProbeEvent 单向事件、
// nil-conn 守卫，以及 TLS 配置面打明文端口的失败路径。

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport/tcp"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

// pairServer 按 msgID 查表应答的帧级服务（reqID 原样回传，应答 msgID = 请求+1，
// 与 wire 请求/应答对齐）。handler 返回 nil 消息表示单向事件、不回帧。
func pairServer(t *testing.T, handlers map[uint32]func(body []byte) (proto.Message, error)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for {
			frame, err := tcp.ReadFrame(conn)
			if err != nil {
				return
			}
			_, msgID, reqID, body, err := protocol.ParseMessageFromBody(frame)
			if err != nil {
				return
			}
			handle, ok := handlers[msgID]
			if !ok {
				return
			}
			resp, err := handle(body)
			if err != nil {
				return
			}
			if resp == nil {
				continue
			}
			out, err := proto.Marshal(resp)
			if err != nil {
				return
			}
			if err := tcp.WriteFrame(conn, protocol.NewMessageBody(msgID+1, reqID, out)); err != nil {
				return
			}
		}
	}()
	return ln.Addr().String()
}

func TestNilConnRequestWrappers(t *testing.T) {
	var c *Conn
	ctx := context.Background()

	if _, err := c.SendAlertReport(ctx, &opsv1.CaptureRuleHitReport{}); err == nil {
		t.Fatal("nil SendAlertReport must fail")
	}
	if err := c.SendProbeEvent(ctx, &opsv1.AlertEvent{}); err == nil {
		t.Fatal("nil SendProbeEvent must fail")
	}
	if _, err := c.PullProbePolicy(ctx, &opsv1.ProbePolicyConfigRequest{}); err == nil {
		t.Fatal("nil PullProbePolicy must fail")
	}
	if _, err := c.PullSyntheticsTargets(ctx, &opsv1.SyntheticsTargetsConfigRequest{}); err == nil {
		t.Fatal("nil PullSyntheticsTargets must fail")
	}
	if _, err := c.PullCaptureGates(ctx, &opsv1.CaptureGatesConfigRequest{}); err == nil {
		t.Fatal("nil PullCaptureGates must fail")
	}
	if _, err := c.PullDevopsAllowlist(ctx, &opsv1.DevopsAllowlistConfigRequest{}); err == nil {
		t.Fatal("nil PullDevopsAllowlist must fail")
	}
}

func TestSendAlertReportRoundTrip(t *testing.T) {
	rules := make(chan string, 1)
	addr := pairServer(t, map[uint32]func([]byte) (proto.Message, error){
		protocol.MsgAlertEventRequest: func(body []byte) (proto.Message, error) {
			req := &opsv1.CaptureRuleHitReport{}
			if err := proto.Unmarshal(body, req); err != nil {
				return nil, err
			}
			rules <- req.Rule
			return &opsv1.CaptureRuleHitAck{Accepted: true}, nil
		},
	})
	conn, err := Dial(addr, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := conn.SendAlertReport(ctx, &opsv1.CaptureRuleHitReport{
		AgentId: "capture-01", Rule: "reconcile.missing_ledger",
	})
	if err != nil {
		t.Fatalf("SendAlertReport: %v", err)
	}
	if !resp.Accepted {
		t.Fatal("ack.Accepted = false")
	}
	select {
	case rule := <-rules:
		if rule != "reconcile.missing_ledger" {
			t.Fatalf("server saw rule %q", rule)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive alert report")
	}
}

func TestPullConfigRoundTrips(t *testing.T) {
	addr := pairServer(t, map[uint32]func([]byte) (proto.Message, error){
		protocol.MsgProbePolicyRequest: func([]byte) (proto.Message, error) {
			return &opsv1.ProbePolicyConfigResponse{Version: "12", PolicyJson: `{"intervalMs":5000}`}, nil
		},
		protocol.MsgSyntheticsTargetsConfigRequest: func([]byte) (proto.Message, error) {
			return &opsv1.SyntheticsTargetsConfigResponse{Version: "3", TargetsJson: `[{"name":"checkout"}]`}, nil
		},
		protocol.MsgCaptureConfigRequest: func([]byte) (proto.Message, error) {
			return &opsv1.CaptureGatesConfigResponse{Version: "5", GatesJson: `{"reconcile":{"enabled":true}}`}, nil
		},
		protocol.MsgDevopsAllowlistConfigRequest: func([]byte) (proto.Message, error) {
			return &opsv1.DevopsAllowlistConfigResponse{Version: "7", AllowlistJson: `{"hosts":["h1"]}`}, nil
		},
	})
	conn, err := Dial(addr, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	policy, err := conn.PullProbePolicy(ctx, &opsv1.ProbePolicyConfigRequest{AgentId: "probe-1"})
	if err != nil {
		t.Fatalf("PullProbePolicy: %v", err)
	}
	if policy.Version != "12" || policy.PolicyJson != `{"intervalMs":5000}` {
		t.Fatalf("policy = %+v", policy)
	}

	targets, err := conn.PullSyntheticsTargets(ctx, &opsv1.SyntheticsTargetsConfigRequest{AgentId: "probe-1"})
	if err != nil {
		t.Fatalf("PullSyntheticsTargets: %v", err)
	}
	if targets.Version != "3" || targets.TargetsJson != `[{"name":"checkout"}]` {
		t.Fatalf("targets = %+v", targets)
	}

	gates, err := conn.PullCaptureGates(ctx, &opsv1.CaptureGatesConfigRequest{AgentId: "capture-01"})
	if err != nil {
		t.Fatalf("PullCaptureGates: %v", err)
	}
	if gates.Version != "5" || gates.GatesJson != `{"reconcile":{"enabled":true}}` {
		t.Fatalf("gates = %+v", gates)
	}

	allow, err := conn.PullDevopsAllowlist(ctx, &opsv1.DevopsAllowlistConfigRequest{AgentId: "devops-1"})
	if err != nil {
		t.Fatalf("PullDevopsAllowlist: %v", err)
	}
	if allow.Version != "7" || allow.AllowlistJson != `{"hosts":["h1"]}` {
		t.Fatalf("allowlist = %+v", allow)
	}
}

func TestSendProbeEventNotify(t *testing.T) {
	events := make(chan *opsv1.AlertEvent, 1)
	addr := pairServer(t, map[uint32]func([]byte) (proto.Message, error){
		protocol.MsgAlertEvent: func(body []byte) (proto.Message, error) {
			ev := &opsv1.AlertEvent{}
			if err := proto.Unmarshal(body, ev); err != nil {
				return nil, err
			}
			events <- ev
			return nil, nil
		},
	})
	conn, err := Dial(addr, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := conn.SendProbeEvent(ctx, &opsv1.AlertEvent{Kind: "probe.window", AgentId: "probe-1"}); err != nil {
		t.Fatalf("SendProbeEvent: %v", err)
	}
	select {
	case ev := <-events:
		if ev.Kind != "probe.window" || ev.AgentId != "probe-1" {
			t.Fatalf("server saw %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not receive probe event")
	}
}

func TestDialTLSEnabledAgainstPlaintextFails(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	addr := ln.Addr().String()
	if _, err := Dial(addr, &TLS{
		Enabled:    true,
		CertFile:   "missing.crt",
		KeyFile:    "missing.key",
		CAFile:     "missing-ca.crt",
		ServerName: "croupier-control",
	}); err == nil {
		t.Fatal("TLS dial against plaintext listener must fail")
	}
}
