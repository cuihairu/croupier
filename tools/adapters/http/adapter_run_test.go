package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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

// --- Invoke error and metadata branches ---

func TestInvokeAlertmanagerBadBaseURLFailsParse(t *testing.T) {
	s := &server{}
	if _, err := s.Invoke(context.Background(), invokeReq("alertmanager.list_alerts", `{"base_url":"http://[::1"}`, nil)); err == nil {
		t.Fatal("bad base_url should fail url.Parse")
	}
}

func TestInvokeAlertmanagerUnreachableUpstream(t *testing.T) {
	s := &server{}
	if _, err := s.Invoke(context.Background(), invokeReq("alertmanager.list_alerts", `{"base_url":"http://127.0.0.1:1"}`, nil)); err == nil {
		t.Fatal("unreachable alertmanager upstream should error")
	}
}

func TestInvokeGrafanaBadBaseURLFailsParse(t *testing.T) {
	s := &server{}
	if _, err := s.Invoke(context.Background(), invokeReq("grafana.search_dashboards", `{"base_url":"http://[::1"}`, nil)); err == nil {
		t.Fatal("bad base_url should fail url.Parse")
	}
}

func TestInvokeGrafanaUnreachableUpstream(t *testing.T) {
	s := &server{}
	if _, err := s.Invoke(context.Background(), invokeReq("grafana.search_dashboards", `{"base_url":"http://127.0.0.1:1"}`, nil)); err == nil {
		t.Fatal("unreachable grafana upstream should error")
	}
}

func TestInvokeGrafanaMetadataHeaders(t *testing.T) {
	var gotTrace, gotGame, gotEnv string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTrace = r.Header.Get("X-Trace-Id")
		gotGame = r.Header.Get("X-Game-Id")
		gotEnv = r.Header.Get("X-Env")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer upstream.Close()

	s := &server{}
	if _, err := s.Invoke(context.Background(), invokeReq("grafana.search_dashboards",
		`{"base_url":"`+upstream.URL+`"}`,
		map[string]string{"traceId": "gt-1", "gameId": "ggame", "env": "genv"})); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if gotTrace != "gt-1" || gotGame != "ggame" || gotEnv != "genv" {
		t.Fatalf("headers trace=%q game=%q env=%q", gotTrace, gotGame, gotEnv)
	}
}

func TestInvokeGrafanaUpstreamError(t *testing.T) {
	// Non-2xx with valid JSON body passes through.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, `{"message":"down"}`)
	}))
	defer upstream.Close()
	s := &server{}
	resp, err := s.Invoke(context.Background(), invokeReq("grafana.search_dashboards", `{"base_url":"`+upstream.URL+`"}`, nil))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if string(resp.GetPayload()) != `{"message":"down"}` {
		t.Fatalf("payload = %q", resp.GetPayload())
	}

	// Non-2xx with non-JSON body wraps status and body.
	upstream2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "not found body")
	}))
	defer upstream2.Close()
	resp, err = s.Invoke(context.Background(), invokeReq("grafana.search_dashboards", `{"base_url":"`+upstream2.URL+`"}`, nil))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !strings.Contains(string(resp.GetPayload()), "not found body") {
		t.Fatalf("wrapped payload = %q", resp.GetPayload())
	}
}

func TestInvokeGenericBadMethodFailsNewRequest(t *testing.T) {
	s := &server{}
	if _, err := s.Invoke(context.Background(), invokeReq("http.generic_invoke",
		`{"method":"BAD METHOD","url":"http://127.0.0.1:1"}`, nil)); err == nil {
		t.Fatal("invalid method should fail http.NewRequestWithContext")
	}
}

func TestInvokeGenericMetadataGameEnvHeaders(t *testing.T) {
	var gotGame, gotEnv string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotGame = r.Header.Get("X-Game-Id")
		gotEnv = r.Header.Get("X-Env")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer upstream.Close()

	s := &server{}
	if _, err := s.Invoke(context.Background(), invokeReq("http.generic_invoke",
		`{"url":"`+upstream.URL+`"}`,
		map[string]string{"gameId": "meta-game", "env": "meta-env", "traceId": ""})); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if gotGame != "meta-game" || gotEnv != "meta-env" {
		t.Fatalf("headers game=%q env=%q", gotGame, gotEnv)
	}
}

// --- run()/keepAlive() lifecycle coverage ---

func TestRunUnreachableAgentReturnsClientError(t *testing.T) {
	t.Setenv("AGENT_ADDR", "127.0.0.1:1")
	if err := run(context.Background()); err == nil || !strings.Contains(err.Error(), "Failed to create TCP client") {
		t.Fatalf("run err = %v, want TCP client failure", err)
	}
}

func TestRunAgentAddrDefaultIsUsed(t *testing.T) {
	// Empty AGENT_ADDR takes the documented default (127.0.0.1:19090).
	// The default endpoint may or may not have a live agent in dev/CI, so
	// accept either a client failure or a cancelled run — both prove the
	// default branch was exercised.
	t.Setenv("AGENT_ADDR", "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := run(ctx)
	if err != nil && !strings.Contains(err.Error(), "Failed to create TCP client") {
		t.Fatalf("run err = %v, want client failure or nil", err)
	}
}

func TestMainFatalsWhenRunFails(t *testing.T) {
	if os.Getenv("HTTP_ADAPTER_CHILD") == "1" {
		main() // should log.Fatalf and exit(1) before returning
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run", "TestMainFatalsWhenRunFails$")
	cmd.Env = append(os.Environ(), "HTTP_ADAPTER_CHILD=1", "AGENT_ADDR=127.0.0.1:1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("child exit err = %v, want exit status 1", err)
	}
	if !strings.Contains(stderr.String(), "Failed to create TCP client") {
		t.Fatalf("child stderr = %q, want TCP client failure", stderr.String())
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

func TestRunRegisterCallFailure(t *testing.T) {
	// A server that accepts the TCP connection but closes it before answering
	// the provider-connect call makes tcpClient.Call fail inside run().
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	t.Setenv("AGENT_ADDR", ln.Addr().String())
	if err := run(context.Background()); err == nil || !strings.Contains(err.Error(), "Failed to register with agent") {
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

	t.Setenv("AGENT_ADDR", addr)
	if err := run(context.Background()); err == nil || !strings.Contains(err.Error(), "Failed to parse ProviderConnectResponse") {
		t.Fatalf("run err = %v, want unmarshal failure", err)
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
			return proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-run"})
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
	t.Setenv("AGENT_ADDR", addr)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()

	select {
	case <-registered:
	case <-time.After(10 * time.Second):
		t.Fatal("run() did not register with fake agent")
	}

	// 30s heartbeat ticker in run() is too slow for tests; exercise keepAlive
	// directly with a short interval.
	client, err := tcptr.NewClient(&tcptr.Config{Address: addr, Insecure: true,
		RecvTimeout: 5 * time.Second, SendTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	hbCtx, hbCancel := context.WithCancel(context.Background())
	hbDone := make(chan error, 1)
	go func() { hbDone <- keepAlive(hbCtx, client, "svc-hb", "sess-run", 10*time.Millisecond) }()

	select {
	case <-heartbeats:
	case <-time.After(5 * time.Second):
		t.Fatal("keepAlive did not send heartbeat")
	}
	hbCancel()
	select {
	case err := <-hbDone:
		if err != nil {
			t.Fatalf("keepAlive err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("keepAlive did not stop on cancel")
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
