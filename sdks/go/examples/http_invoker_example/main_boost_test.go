package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/sdks/go/pkg/croupier"
)

// runScenario returns an error when the server rejects the invocation.
func TestRunScenarioBoost_ServerRejectsInvocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "forbidden", "message": "denied"})
	}))
	defer server.Close()

	cfg := &croupier.InvokerConfig{Address: server.URL, Insecure: true, TimeoutSeconds: 5}
	err := runScenario(cfg)
	if err == nil {
		t.Fatal("server rejection must surface as an error")
	}
	if !strings.Contains(err.Error(), "invoke") && !strings.Contains(err.Error(), "403") && !strings.Contains(err.Error(), "denied") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// runScenario surfaces connection failures when the server is closed.
func TestRunScenarioBoost_ConnectionRefused(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	address := server.URL
	server.Close() // nothing listens anymore

	cfg := &croupier.InvokerConfig{Address: address, Insecure: true, TimeoutSeconds: 2}
	if err := runScenario(cfg); err == nil {
		t.Fatal("connection failure must surface as an error")
	}
}

// runScenario succeeds end to end against a well-behaved mock server and
// round-trips scope headers.
func TestRunScenarioBoost_HeaderPropagation(t *testing.T) {
	var gotGameID, gotEnv, gotIdem string
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotGameID = r.Header.Get("X-Game-ID")
		gotEnv = r.Header.Get("X-Env")
		gotIdem = r.Header.Get("Idempotency-Key")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"result": map[string]interface{}{"status": "banned"},
		})
	}))
	defer server.Close()

	cfg := &croupier.InvokerConfig{Address: server.URL, Insecure: true, TimeoutSeconds: 5}
	if err := runScenario(cfg); err != nil {
		t.Fatalf("runScenario: %v", err)
	}

	if gotGameID != "example-game" || gotEnv != "development" {
		t.Fatalf("scope headers missing: game=%q env=%q", gotGameID, gotEnv)
	}
	if gotIdem == "" {
		t.Fatal("idempotency key must be sent")
	}
	params, ok := gotBody["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload must be nested under params: %v", gotBody)
	}
	if params["playerId"] != "player_12345" || params["reason"] != "违规操作" {
		t.Fatalf("payload not round-tripped: %v", params)
	}
	if params["duration"] != float64(3600) {
		t.Fatalf("duration not round-tripped: %v", params["duration"])
	}
}

// errServerUnreachable stays referenced so the sentinel stays part of the API.
func TestRunScenarioBoost_SentinelExists(t *testing.T) {
	if errServerUnreachable == nil || errServerUnreachable.Error() != "server unreachable" {
		t.Fatalf("unexpected sentinel: %v", errServerUnreachable)
	}
}
