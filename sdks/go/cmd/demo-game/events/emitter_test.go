package events

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignIsCanonicalAndVerifiable(t *testing.T) {
	body := []byte(`{"gameId":"demo-game"}`)
	sig := Sign("sekret", "1700000000", body)

	mac := hmac.New(sha256.New, []byte("sekret"))
	mac.Write([]byte("1700000000."))
	mac.Write(body)
	want := hex.EncodeToString(mac.Sum(nil))
	if sig != want {
		t.Fatalf("sign = %s, want %s", sig, want)
	}
	// Canonical string is timestamp + "." + body: a different timestamp or
	// body must not verify.
	if sig == Sign("sekret", "1700000001", body) {
		t.Fatal("timestamp not bound into signature")
	}
	if sig == Sign("sekret", "1700000000", []byte(`{"gameId":"other"}`)) {
		t.Fatal("body not bound into signature")
	}
	if sig == Sign("other", "1700000000", body) {
		t.Fatal("secret not bound into signature")
	}
}

func TestEmitterDisabledWithoutEndpoint(t *testing.T) {
	t.Setenv(EnvEndpoint, "")
	t.Setenv(EnvSecret, "whatever")
	e := NewEmitter("demo-game", "demo")
	if e.Enabled() {
		t.Fatal("emitter enabled without endpoint")
	}
	accepted, err := e.Emit(map[string]any{"k": "v"})
	if accepted != 0 || err != nil {
		t.Fatalf("disabled emit = %d, %v; want 0, nil", accepted, err)
	}
}

func TestEmitterPostsSignedIngestRequest(t *testing.T) {
	var gotSig, gotTS, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/analytics/ingest" {
			t.Errorf("path = %s", r.URL.Path)
		}
		raw := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(raw)
		gotBody = string(raw)
		gotSig = r.Header.Get(SignatureHeader)
		gotTS = r.Header.Get(TimestampHeader)
		_ = json.NewEncoder(w).Encode(map[string]int{"accepted": 3, "rejected": 0})
	}))
	defer server.Close()

	t.Setenv(EnvEndpoint, server.URL)
	t.Setenv(EnvSecret, "sekret")
	e := NewEmitter("demo-game", "demo")
	if !e.Enabled() {
		t.Fatal("emitter should be enabled with endpoint set")
	}
	accepted, err := e.Emit(map[string]any{"events": []int{1, 2, 3}})
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if accepted != 3 {
		t.Fatalf("accepted = %d, want 3", accepted)
	}
	if gotSig == "" || gotTS == "" {
		t.Fatal("signature/timestamp headers missing")
	}
	if gotSig != Sign("sekret", gotTS, []byte(gotBody)) {
		t.Fatal("signature does not verify over posted body")
	}
	var req IngestRequest
	if err := json.Unmarshal([]byte(gotBody), &req); err != nil {
		t.Fatalf("request not IngestRequest shape: %v", err)
	}
	if req.GameId != "demo-game" || req.Env != "demo" {
		t.Fatalf("request identity = %s/%s", req.GameId, req.Env)
	}
}

func TestEmitterUnsignedWithoutSecret(t *testing.T) {
	var gotSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSig = r.Header.Get(SignatureHeader)
		_ = json.NewEncoder(w).Encode(map[string]int{"accepted": 1})
	}))
	defer server.Close()

	t.Setenv(EnvEndpoint, server.URL)
	t.Setenv(EnvSecret, "")
	e := NewEmitter("demo-game", "demo")
	if _, err := e.Emit(map[string]any{}); err != nil {
		t.Fatalf("emit without secret: %v", err)
	}
	if gotSig != "" {
		t.Fatal("signature header set without secret")
	}
}

func TestEmitterSurfacesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	t.Setenv(EnvEndpoint, server.URL)
	t.Setenv(EnvSecret, "")
	e := NewEmitter("demo-game", "demo")
	if _, err := e.Emit(map[string]any{}); err == nil {
		t.Fatal("server error swallowed by emitter")
	}
}
