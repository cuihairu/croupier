package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

func TestRunScenario_InvokeFlow(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}
	var gotGameID, gotEnv, gotIdem string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		switch {
		case r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/auth/login":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "t"})
		case r.Method == http.MethodPost && len(r.URL.Path) > 0:
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			gotGameID = r.Header.Get("X-Game-ID")
			gotEnv = r.Header.Get("X-Env")
			gotIdem = r.Header.Get("Idempotency-Key")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"taskId": "",
				"result": map[string]interface{}{"banned": true},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &croupier.InvokerConfig{
		Address:        server.URL + "/api/v1",
		TimeoutSeconds: 5,
		Insecure:       true,
	}
	if err := runScenario(cfg); err != nil {
		t.Fatalf("runScenario: %v", err)
	}
	if gotPath == "" {
		t.Fatal("no invoke request hit the server")
	}
	if gotGameID != "example-game" || gotEnv != "development" {
		t.Fatalf("scope headers missing: game=%q env=%q", gotGameID, gotEnv)
	}
	if gotIdem == "" {
		t.Fatal("idempotency key not propagated")
	}
	if gotBody["params"] == nil {
		t.Fatalf("unexpected invoke body: %v", gotBody)
	}
}

func TestRunScenario_ServerDown(t *testing.T) {
	// 指向一个已关闭的端口：连接失败必须报错而不是挂死。
	cfg := &croupier.InvokerConfig{
		Address:        "http://127.0.0.1:1/api/v1",
		TimeoutSeconds: 2,
		Insecure:       true,
	}
	if err := runScenario(cfg); err == nil {
		t.Fatal("expected error when server is unreachable")
	}
}
