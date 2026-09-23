package main

// 测试形态对齐 tools/adapters/http/adapter_run_test.go：fake agent（tcptr
// server）直测 run() 的装配链路（env 缺失/建连失败/注册失败/垃圾响应/全链
// 成功 + 心跳），main 壳用子进程法验证 log.Fatalf 退出语义。

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

func TestRunPROMURLRequired(t *testing.T) {
	t.Setenv("PROM_URL", "")
	if err := run(context.Background()); err == nil || !strings.Contains(err.Error(), "PROM_URL required") {
		t.Fatalf("run err = %v, want PROM_URL required", err)
	}
}

func TestRunUnreachableAgentReturnsClientError(t *testing.T) {
	t.Setenv("PROM_URL", "http://127.0.0.1:1")
	t.Setenv("AGENT_ADDR", "127.0.0.1:1")
	if err := run(context.Background()); err == nil || !strings.Contains(err.Error(), "failed to create TCP client") {
		t.Fatalf("run err = %v, want TCP client failure", err)
	}
}

func TestMainFatalsWhenRunFails(t *testing.T) {
	if os.Getenv("PROM_ADAPTER_CHILD") == "1" {
		main() // should log.Fatalf and exit(1) before returning
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run", "TestMainFatalsWhenRunFails$")
	cmd.Env = append(os.Environ(), "PROM_ADAPTER_CHILD=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("child exit err = %v, want exit status 1", err)
	}
	if !strings.Contains(stderr.String(), "PROM_URL required") {
		t.Fatalf("child stderr = %q, want PROM_URL required", stderr.String())
	}
}

func TestRunRegisterCallFailure(t *testing.T) {
	// 接受连接但在应答 provider-connect 前即关闭，使 run() 内 Call 失败。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	t.Setenv("PROM_URL", "http://127.0.0.1:1")
	t.Setenv("AGENT_ADDR", ln.Addr().String())
	if err := run(context.Background()); err == nil || !strings.Contains(err.Error(), "failed to register with agent") {
		t.Fatalf("run err = %v, want register failure", err)
	}
}

func TestRunGarbageConnectResponseFailsUnmarshal(t *testing.T) {
	addr, stop := fakeAgentHandler(t, func(msgID uint32, body []byte) ([]byte, error) {
		if msgID == protocol.MsgProviderConnectRequest {
			return []byte{0xff, 0xff, 0xff, 0xff}, nil
		}
		return nil, nil
	})
	defer stop()

	t.Setenv("PROM_URL", "http://127.0.0.1:1")
	t.Setenv("AGENT_ADDR", addr)
	if err := run(context.Background()); err == nil || !strings.Contains(err.Error(), "failed to parse ProviderConnectResponse") {
		t.Fatalf("run err = %v, want unmarshal failure", err)
	}
}

func TestBuildProviderConnectRequest(t *testing.T) {
	req := buildProviderConnectRequest("svc-x", "9.9.9")
	if req.ServiceId != "svc-x" || req.Version != "9.9.9" {
		t.Fatalf("service/version = %s/%s, want svc-x/9.9.9", req.ServiceId, req.Version)
	}
	if len(req.Functions) != 2 {
		t.Fatalf("functions = %d, want 2 (prom.query / prom.query_range)", len(req.Functions))
	}
	if req.Functions[0].Id != "prom.query" || req.Functions[1].Id != "prom.query_range" {
		t.Fatalf("function ids = %s,%s", req.Functions[0].Id, req.Functions[1].Id)
	}
	if req.SdkLanguage != "go" || req.ProtocolVersion != "v1" {
		t.Fatalf("sdk/protocol = %s/%s", req.SdkLanguage, req.ProtocolVersion)
	}
	for _, fn := range req.Functions {
		if fn.InputSchema == "" || fn.OutputSchema == "" {
			t.Fatalf("function %s missing schemas", fn.Id)
		}
	}
	// pin：构造的纯类型化消息 Marshal 恒成功（C 类豁免的不变式，
	// 见 main.go 两处 Marshal 分支的豁免注释）
	if _, err := proto.Marshal(req); err != nil {
		t.Fatalf("Marshal(regReq) = %v, want always-success", err)
	}
	if _, err := proto.Marshal(&sdkv1.ProviderHeartbeatRequest{ServiceId: "s", SessionId: "x"}); err != nil {
		t.Fatalf("Marshal(hbReq) = %v, want always-success", err)
	}
}

func TestRunSuccessWithDefaultsAndHeartbeat(t *testing.T) {
	heartbeats := make(chan struct{}, 4)
	registered := make(chan struct{}, 1)
	addr, stop := fakeAgentHandler(t, func(msgID uint32, body []byte) ([]byte, error) {
		switch msgID {
		case protocol.MsgProviderConnectRequest:
			select {
			case registered <- struct{}{}:
			default:
			}
			return proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-prom"})
		case protocol.MsgProviderHeartbeatRequest:
			select {
			case heartbeats <- struct{}{}:
			default:
			}
			return proto.Marshal(&sdkv1.ProviderHeartbeatResponse{})
		}
		return nil, nil
	})
	defer stop()

	// AGENT_ADDR set; RPC_ADDR / SERVICE_ID / VERSION left empty to cover defaults.
	t.Setenv("PROM_URL", "http://127.0.0.1:1")
	t.Setenv("AGENT_ADDR", addr)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()

	select {
	case <-registered:
	case <-time.After(10 * time.Second):
		t.Fatal("run() did not register with fake agent")
	}

	// 30s heartbeat ticker in run() is too slow for tests; exercise
	// heartbeatLoop directly with a short interval.
	client, err := tcptr.NewClient(&tcptr.Config{Address: addr, Insecure: true,
		RecvTimeout: 5 * time.Second, SendTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer func() { _ = client.Close() }()

	hbCtx, hbCancel := context.WithCancel(context.Background())
	hbDone := make(chan error, 1)
	go func() { hbDone <- heartbeatLoop(hbCtx, client, "svc-prom", "sess-prom", 10*time.Millisecond) }()

	select {
	case <-heartbeats:
	case <-time.After(5 * time.Second):
		t.Fatal("heartbeatLoop did not send heartbeat")
	}
	hbCancel()
	select {
	case err := <-hbDone:
		if err != nil {
			t.Fatalf("heartbeatLoop err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("heartbeatLoop did not stop on cancel")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run err = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("run() did not return after context cancel")
	}
}

// fakeAgentHandler builds a plain TCP agent answering the provider handshake
// with a configurable responder.
func fakeAgentHandler(t *testing.T, respond func(msgID uint32, body []byte) ([]byte, error)) (addr string, stop func()) {
	t.Helper()
	srv, err := tcptr.NewServer(&tcptr.Config{Address: "127.0.0.1:0", Insecure: true},
		transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
			return respond(msgID, body)
		}))
	if err != nil {
		t.Fatalf("fake agent: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = srv.Serve(ctx)
		close(done)
	}()
	return srv.Addr(), func() {
		cancel()
		_ = srv.Close()
		<-done
	}
}
