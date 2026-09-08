package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

func invokeReq(functionID string, payload string, metadata map[string]string) *sdkv1.InvokeRequest {
	req := &sdkv1.InvokeRequest{FunctionId: functionID, Payload: []byte(payload)}
	if metadata != nil {
		req.Metadata = metadata
	}
	return req
}

func TestInvokeAlertmanagerListAlerts(t *testing.T) {
	var gotPath, gotQuery, gotTrace, gotGame, gotEnv string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotTrace = r.Header.Get("X-Trace-Id")
		gotGame = r.Header.Get("X-Game-Id")
		gotEnv = r.Header.Get("X-Env")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"alerts":[]}`)
	}))
	defer upstream.Close()

	s := &server{}
	resp, err := s.Invoke(context.Background(), invokeReq("alertmanager.list_alerts",
		`{"base_url":"`+upstream.URL+`/","silenced":false,"inhibited":true,"active":false}`,
		map[string]string{"traceId": "t-1", "gameId": "demo", "env": "prod"}))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if gotPath != "/api/v2/alerts" {
		t.Fatalf("path = %q", gotPath)
	}
	for _, want := range []string{"silenced=false", "inhibited=true", "active=false"} {
		if !strings.Contains(gotQuery, want) {
			t.Fatalf("query %q missing %q", gotQuery, want)
		}
	}
	if gotTrace != "t-1" || gotGame != "demo" || gotEnv != "prod" {
		t.Fatalf("headers trace=%q game=%q env=%q", gotTrace, gotGame, gotEnv)
	}
	var out map[string]any
	if err := json.Unmarshal(resp.GetPayload(), &out); err != nil {
		t.Fatalf("payload not json: %v", err)
	}
}

func TestInvokeAlertmanagerOptionalFiltersOmitted(t *testing.T) {
	var gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `{}`)
	}))
	defer upstream.Close()

	s := &server{}
	if _, err := s.Invoke(context.Background(), invokeReq("alertmanager.list_alerts", `{"base_url":"`+upstream.URL+`"}`, nil)); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if gotQuery != "" {
		t.Fatalf("query = %q, want empty", gotQuery)
	}
}

func TestInvokeAlertmanagerErrors(t *testing.T) {
	s := &server{}
	if _, err := s.Invoke(context.Background(), invokeReq("alertmanager.list_alerts", `{bad`, nil)); err == nil || !strings.Contains(err.Error(), "bad payload") {
		t.Fatalf("bad payload err = %v", err)
	}
	if _, err := s.Invoke(context.Background(), invokeReq("alertmanager.list_alerts", `{}`, nil)); err == nil || !strings.Contains(err.Error(), "base_url required") {
		t.Fatalf("missing base_url err = %v", err)
	}
}

func TestInvokeAlertmanagerUpstreamError(t *testing.T) {
	// Non-2xx with valid JSON body passes the body through.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, `{"error":"timeout"}`)
	}))
	s := &server{}
	resp, err := s.Invoke(context.Background(), invokeReq("alertmanager.list_alerts", `{"base_url":"`+upstream.URL+`"}`, nil))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if string(resp.GetPayload()) != `{"error":"timeout"}` {
		t.Fatalf("payload = %q", resp.GetPayload())
	}
	upstream.Close()

	// Non-2xx with invalid JSON wraps status and body.
	upstream2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer upstream2.Close()
	resp, err = s.Invoke(context.Background(), invokeReq("alertmanager.list_alerts", `{"base_url":"`+upstream2.URL+`"}`, nil))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	var wrapped map[string]any
	if err := json.Unmarshal(resp.GetPayload(), &wrapped); err != nil {
		t.Fatalf("wrapped payload: %v", err)
	}
	if wrapped["body"] != "boom" {
		t.Fatalf("wrapped = %v", wrapped)
	}
}

func TestInvokeGrafanaSearchDashboards(t *testing.T) {
	var gotPath, gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `[{"id":"dash-1"}]`)
	}))
	defer upstream.Close()

	s := &server{}
	resp, err := s.Invoke(context.Background(), invokeReq("grafana.search_dashboards", `{"base_url":"`+upstream.URL+`","query":"cpu"}`, nil))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if gotPath != "/api/search" {
		t.Fatalf("path = %q", gotPath)
	}
	if !strings.Contains(gotQuery, "query=cpu") || !strings.Contains(gotQuery, "type=dash-db") {
		t.Fatalf("query = %q", gotQuery)
	}

	// Custom type is forwarded.
	if _, err := s.Invoke(context.Background(), invokeReq("grafana.search_dashboards", `{"base_url":"`+upstream.URL+`","type":"dash-folder"}`, nil)); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if strings.Contains(gotQuery, "query=") {
		t.Fatalf("query should omit empty query, got %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "type=dash-folder") {
		t.Fatalf("query = %q", gotQuery)
	}
	if !strings.Contains(string(resp.GetPayload()), "dash-1") {
		t.Fatalf("payload = %q", resp.GetPayload())
	}

	if _, err := s.Invoke(context.Background(), invokeReq("grafana.search_dashboards", `{bad`, nil)); err == nil || !strings.Contains(err.Error(), "bad payload") {
		t.Fatalf("bad payload err = %v", err)
	}
	if _, err := s.Invoke(context.Background(), invokeReq("grafana.search_dashboards", `{}`, nil)); err == nil || !strings.Contains(err.Error(), "base_url required") {
		t.Fatalf("missing base_url err = %v", err)
	}
}

func TestInvokeGeneric(t *testing.T) {
	var gotMethod, gotBody, gotHeader, gotTrace string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotHeader = r.Header.Get("X-Custom")
		gotTrace = r.Header.Get("X-Trace-Id")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer upstream.Close()

	s := &server{}
	resp, err := s.Invoke(context.Background(), invokeReq("http.generic_invoke",
		`{"method":"POST","url":"`+upstream.URL+`","headers":{"X-Custom":"c1"},"body":"raw-body"}`,
		map[string]string{"traceId": "t-9"}))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if gotMethod != http.MethodPost || gotBody != "raw-body" || gotHeader != "c1" || gotTrace != "t-9" {
		t.Fatalf("method=%q body=%q header=%q trace=%q", gotMethod, gotBody, gotHeader, gotTrace)
	}
	if !strings.Contains(string(resp.GetPayload()), `"ok":true`) {
		t.Fatalf("payload = %q", resp.GetPayload())
	}

	// Method defaults to GET and non-JSON responses are wrapped.
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		_, _ = io.WriteString(w, "plain-text")
	}))
	defer plain.Close()
	resp, err = s.Invoke(context.Background(), invokeReq("http.generic_invoke", `{"url":"`+plain.URL+`"}`, nil))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	var wrapped map[string]any
	if err := json.Unmarshal(resp.GetPayload(), &wrapped); err != nil {
		t.Fatalf("wrapped: %v", err)
	}
	if wrapped["body"] != "plain-text" {
		t.Fatalf("wrapped = %v", wrapped)
	}

	// Error paths.
	if _, err := s.Invoke(context.Background(), invokeReq("http.generic_invoke", `{bad`, nil)); err == nil || !strings.Contains(err.Error(), "bad payload") {
		t.Fatalf("bad payload err = %v", err)
	}
	if _, err := s.Invoke(context.Background(), invokeReq("unknown.function", `{bad`, nil)); err == nil || !strings.Contains(err.Error(), "bad payload") {
		t.Fatalf("unknown function falls to generic path, err = %v", err)
	}
	if _, err := s.Invoke(context.Background(), invokeReq("http.generic_invoke", `{"url":"http://[::1]:99999/x"}`, nil)); err == nil {
		t.Fatal("invalid url should error")
	}
	if _, err := s.Invoke(context.Background(), invokeReq("http.generic_invoke", `{"url":"http://127.0.0.1:1/x"}`, nil)); err == nil {
		t.Fatal("unreachable upstream should error")
	}
}

// fakeAgent spins up a plain TCP server that answers the provider-session
// connect handshake so adapter main() can run end-to-end.
func fakeAgent(t *testing.T) (addr string, registered <-chan struct{}, stop func()) {
	t.Helper()
	registeredCh := make(chan struct{}, 1)
	srv, err := tcptr.NewServer(&tcptr.Config{Address: "127.0.0.1:0", Insecure: true},
		transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
			if msgID == protocol.MsgProviderConnectRequest {
				resp, _ := proto.Marshal(&sdkv1.ProviderConnectResponse{SessionId: "sess-test"})
				select {
				case registeredCh <- struct{}{}:
				default:
				}
				return resp, nil
			}
			return nil, nil
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
	return srv.Addr(), registeredCh, func() {
		cancel()
		_ = srv.Close()
		<-done
	}
}

func TestMainRegistersWithFakeAgent(t *testing.T) {
	addr, registered, stop := fakeAgent(t)
	defer stop()

	t.Setenv("AGENT_ADDR", addr)
	t.Setenv("SERVICE_ID", "http-adapter-test")
	t.Setenv("VERSION", "9.9.9")

	go main()

	select {
	case <-registered:
	case <-time.After(10 * time.Second):
		t.Fatal("adapter main() did not register with fake agent")
	}
}
