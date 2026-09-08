package croupier

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkv1 "github.com/cuihairu/croupier/sdks/go/pkg/pb/croupier/sdk/v1"
)

// ---------------------------------------------------------------------------
// NoOpLogger
// ---------------------------------------------------------------------------

func TestBoostNoOpLogger_Debugf(t *testing.T) {
	l := &NoOpLogger{}
	l.Debugf("should not output %d", 1)
	if GetGlobalLogger() == nil {
		t.Fatal("global logger must never be nil")
	}
}

func TestBoostNoOpLogger_Infof(t *testing.T) {
	l := &NoOpLogger{}
	l.Infof("should not output %s", "x")
}

func TestBoostNoOpLogger_Warnf(t *testing.T) {
	l := &NoOpLogger{}
	l.Warnf("should not output")
}

func TestBoostNoOpLogger_Errorf(t *testing.T) {
	l := &NoOpLogger{}
	l.Errorf("should not output")
}

func TestBoostNoOpLogger_SatisfiesInterface(t *testing.T) {
	var _ Logger = &NoOpLogger{}
	var _ Logger = NewDefaultLogger(false, nil)
}

// ---------------------------------------------------------------------------
// DefaultLogger
// ---------------------------------------------------------------------------

func TestBoostDefaultLogger_DebugDisabled(t *testing.T) {
	var buf bytes.Buffer
	l := NewDefaultLogger(false, &buf)
	l.Debugf("hidden %d", 42)
	if buf.Len() != 0 {
		t.Fatalf("debug output should be suppressed, got %q", buf.String())
	}
}

func TestBoostDefaultLogger_DebugEnabled(t *testing.T) {
	var buf bytes.Buffer
	l := NewDefaultLogger(true, &buf)
	l.Debugf("visible %d", 42)
	if !strings.Contains(buf.String(), "[DEBUG] visible 42") {
		t.Fatalf("unexpected debug output %q", buf.String())
	}
}

func TestBoostDefaultLogger_Infof(t *testing.T) {
	var buf bytes.Buffer
	l := NewDefaultLogger(false, &buf)
	l.Infof("hello %s", "world")
	if !strings.Contains(buf.String(), "[INFO] hello world") {
		t.Fatalf("unexpected info output %q", buf.String())
	}
}

func TestBoostDefaultLogger_Warnf(t *testing.T) {
	var buf bytes.Buffer
	l := NewDefaultLogger(false, &buf)
	l.Warnf("careful %s", "!")
	if !strings.Contains(buf.String(), "[WARN] careful !") {
		t.Fatalf("unexpected warn output %q", buf.String())
	}
}

func TestBoostDefaultLogger_Errorf(t *testing.T) {
	var buf bytes.Buffer
	l := NewDefaultLogger(false, &buf)
	l.Errorf("boom %s", "!")
	if !strings.Contains(buf.String(), "[ERROR] boom !") {
		t.Fatalf("unexpected error output %q", buf.String())
	}
}

func TestBoostDefaultLogger_NilWriterDefaultsToStdout(t *testing.T) {
	l := NewDefaultLogger(false, nil)
	if l.out == nil {
		t.Fatal("nil writer should default to stdout")
	}
	l.Infof("written to stdout, safe")
}

// ---------------------------------------------------------------------------
// Global logger helpers
// ---------------------------------------------------------------------------

func TestBoostGlobalLogger_SetGetRoundTrip(t *testing.T) {
	original := GetGlobalLogger()
	defer SetGlobalLogger(original)

	var buf bytes.Buffer
	custom := NewDefaultLogger(true, &buf)
	SetGlobalLogger(custom)
	if GetGlobalLogger() != custom {
		t.Fatal("GetGlobalLogger should return the logger previously set")
	}
}

func TestBoostGlobalLogger_HelpersRouteToCustomLogger(t *testing.T) {
	original := GetGlobalLogger()
	defer SetGlobalLogger(original)

	var buf bytes.Buffer
	SetGlobalLogger(NewDefaultLogger(true, &buf))

	logDebugf("dbg %d", 1)
	logInfof("inf %d", 2)
	logWarnf("wrn %d", 3)
	logErrorf("err %d", 4)

	out := buf.String()
	for _, prefix := range []string{"[DEBUG] dbg 1", "[INFO] inf 2", "[WARN] wrn 3", "[ERROR] err 4"} {
		if !strings.Contains(out, prefix) {
			t.Fatalf("missing %q in %q", prefix, out)
		}
	}
}

func TestBoostGlobalLogger_SetAndGetConcurrent(t *testing.T) {
	original := GetGlobalLogger()
	defer SetGlobalLogger(original)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			SetGlobalLogger(NewDefaultLogger(false, nil))
			_ = GetGlobalLogger()
		}
	}()
	for i := 0; i < 50; i++ {
		GetGlobalLogger().Infof("concurrent %d", i)
	}
	<-done
}

// ---------------------------------------------------------------------------
// firstNonEmpty / firstNonEmptySlice edge branches
// ---------------------------------------------------------------------------

func TestBoostFirstNonEmpty_AllBranches(t *testing.T) {
	cases := []struct {
		values []string
		want   string
	}{
		{[]string{}, ""},
		{[]string{""}, ""},
		{[]string{"", ""}, ""},
		{[]string{"a"}, "a"},
		{[]string{"", "b"}, "b"},
		{[]string{"", "", "c"}, "c"},
		{[]string{"a", "b", "c"}, "a"},
	}
	for _, tc := range cases {
		if got := firstNonEmpty(tc.values...); got != tc.want {
			t.Fatalf("firstNonEmpty(%v) = %q, want %q", tc.values, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// nextBackoffDelay floor/cap invariants
// ---------------------------------------------------------------------------

func TestBoostNextBackoffDelay_AlwaysWithinBounds(t *testing.T) {
	c := &client{}
	rc := &ReconnectConfig{
		BackoffMultiplier: 2.0,
		JitterFactor:      0.3,
	}
	const max = 10 * time.Second
	current := time.Millisecond
	for i := 0; i < 20; i++ {
		next := c.nextBackoffDelay(current, max, rc)
		if next < time.Millisecond {
			t.Fatalf("delay %v below 1ms floor", next)
		}
		if next > max+time.Duration(0.3*float64(max)) {
			t.Fatalf("delay %v exceeds max+jitter bound", next)
		}
		current = next
	}
}

// ---------------------------------------------------------------------------
// gzipBytes — normal + error paths
// ---------------------------------------------------------------------------

func TestGzipBytes_Normal(t *testing.T) {
	data := []byte("hello world")
	compressed, err := gzipBytes(data)
	if err != nil {
		t.Fatalf("gzipBytes: %v", err)
	}
	if len(compressed) == 0 {
		t.Fatal("gzipBytes returned empty result")
	}
	// verify it's valid gzip
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll: %v", err)
	}
	if string(decompressed) != string(data) {
		t.Fatalf("decompressed data mismatch: got %q, want %q", string(decompressed), string(data))
	}
}

func TestGzipBytes_EmptyData(t *testing.T) {
	compressed, err := gzipBytes([]byte{})
	if err != nil {
		t.Fatalf("gzipBytes empty: %v", err)
	}
	if len(compressed) == 0 {
		t.Fatal("gzipBytes empty returned empty result")
	}
}

// ---------------------------------------------------------------------------
// jsonString — error path (json.Marshal failure is rare for strings)
// ---------------------------------------------------------------------------

func TestJsonString_Normal(t *testing.T) {
	result := jsonString("hello")
	if result != `"hello"` {
		t.Fatalf("jsonString: got %q, want %q", result, `"hello"`)
	}
}

func TestJsonString_Empty(t *testing.T) {
	result := jsonString("")
	if result != `""` {
		t.Fatalf("jsonString empty: got %q, want %q", result, `""`)
	}
}

func TestJsonString_SpecialChars(t *testing.T) {
	result := jsonString(`line1\nline2`)
	if !strings.Contains(result, `line1`) {
		t.Fatalf("jsonString special: got %q", result)
	}
}

// ---------------------------------------------------------------------------
// atomicWriteFile — error paths
// ---------------------------------------------------------------------------

func TestAtomicWriteFile_Normal(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "test.txt")
	data := []byte("hello world")
	if err := atomicWriteFile(target, data); err != nil {
		t.Fatalf("atomicWriteFile: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("content mismatch: got %q, want %q", string(got), string(data))
	}
}

func TestAtomicWriteFile_InvalidDir(t *testing.T) {
	err := atomicWriteFile("/nonexistent/dir/file.txt", []byte("data"))
	if err == nil {
		t.Fatal("expected error for invalid directory")
	}
}

func TestAtomicWriteFile_EmptyData(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "empty.txt")
	if err := atomicWriteFile(target, []byte{}); err != nil {
		t.Fatalf("atomicWriteFile empty: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("os.ReadFile: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty file, got %d bytes", len(got))
	}
}

// ---------------------------------------------------------------------------
// sdkVersion — both branches
// ---------------------------------------------------------------------------

func TestSdkVersion(t *testing.T) {
	v := sdkVersion()
	if v == "" {
		t.Fatal("sdkVersion should not return empty string")
	}
}

// ---------------------------------------------------------------------------
// getGoroutineID — normal + error path
// ---------------------------------------------------------------------------

func TestGetGoroutineID_Boost(t *testing.T) {
	id := getGoroutineID()
	if id <= 0 {
		t.Fatalf("getGoroutineID returned invalid id: %d", id)
	}
}

func TestGetGoroutineID_Concurrent_Boost(t *testing.T) {
	done := make(chan int64, 10)
	for i := 0; i < 10; i++ {
		go func() {
			done <- getGoroutineID()
		}()
	}
	seen := make(map[int64]bool)
	for i := 0; i < 10; i++ {
		id := <-done
		seen[id] = true
	}
	// each goroutine should have a unique ID (or at least some unique ones)
	if len(seen) < 2 {
		t.Fatalf("expected at least 2 unique goroutine IDs, got %d", len(seen))
	}
}

// ---------------------------------------------------------------------------
// orDefault — all branches
// ---------------------------------------------------------------------------

func TestOrDefault(t *testing.T) {
	cases := []struct {
		value, fallback, want string
	}{
		{"a", "b", "a"},
		{"", "b", "b"},
		{"  ", "b", "b"},
		{"x", "", "x"},
		{"", "", ""},
	}
	for _, tc := range cases {
		if got := orDefault(tc.value, tc.fallback); got != tc.want {
			t.Fatalf("orDefault(%q, %q) = %q, want %q", tc.value, tc.fallback, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// buildManifest — various function descriptor combinations
// ---------------------------------------------------------------------------

func TestBuildManifest_Variants(t *testing.T) {
	m := &TCPManager{
		config: ClientConfig{
			ProviderLang: "go",
			ProviderSDK:  "croupier-go-sdk",
		},
		functions: []*sdkv1.ProviderFunctionDescriptor{
			{Id: "fn1", Version: "1.0.0", Resource: "player", Operation: "ban",
				Risk: "high", Permission: "player.ban", Description: "Ban player",
				InputSchema: `{"type":"object"}`, OutputSchema: `{"type":"object"}`},
			{Id: "fn2"},
			{Id: "", Version: "1.0.0"}, // blank ID should be skipped
			nil,                        // nil should be skipped
			{Id: "fn3"},                // no version - uses default
		},
	}
	manifest := m.buildManifest("svc", "2.0.0", m.functions)
	if manifest == nil {
		t.Fatal("buildManifest returned nil")
	}
	provider, ok := manifest["provider"].(map[string]interface{})
	if !ok {
		t.Fatal("manifest missing provider")
	}
	if provider["id"] != "svc" {
		t.Fatalf("provider.id = %v, want svc", provider["id"])
	}
	functions, ok := manifest["functions"].([]map[string]interface{})
	if !ok {
		t.Fatal("manifest missing functions")
	}
	// fn1 + fn2 + fn3 (blank ID and nil skipped, fn3 with whitespace version kept)
	if len(functions) != 3 {
		t.Fatalf("expected 3 functions, got %d", len(functions))
	}
	if functions[0]["id"] != "fn1" {
		t.Fatalf("functions[0].id = %v", functions[0]["id"])
	}
	if functions[0]["resource"] != "player" {
		t.Fatalf("functions[0].resource = %v", functions[0]["resource"])
	}
}

func TestBuildManifest_EmptyServiceID(t *testing.T) {
	m := &TCPManager{config: ClientConfig{}}
	manifest := m.buildManifest("", "", nil)
	provider := manifest["provider"].(map[string]interface{})
	if provider["id"] != "go-service" {
		t.Fatalf("expected default provider id, got %v", provider["id"])
	}
	if provider["version"] != "1.0.0" {
		t.Fatalf("expected default version, got %v", provider["version"])
	}
	if provider["lang"] != "go" {
		t.Fatalf("expected default lang, got %v", provider["lang"])
	}
	if provider["sdk"] != "croupier-go-sdk" {
		t.Fatalf("expected default sdk, got %v", provider["sdk"])
	}
}

// ---------------------------------------------------------------------------
// validateInboundPayload — various branches
// ---------------------------------------------------------------------------

func TestValidateInboundPayload_Disabled(t *testing.T) {
	m := &TCPManager{config: ClientConfig{ValidateInputPayloads: false}}
	if err := m.validateInboundPayload("fn", []byte("bad")); err != nil {
		t.Fatalf("expected nil when validation disabled, got %v", err)
	}
}

func TestValidateInboundPayload_NoSchema(t *testing.T) {
	m := &TCPManager{
		config:    ClientConfig{ValidateInputPayloads: true},
		functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "fn1"}},
	}
	if err := m.validateInboundPayload("fn1", []byte(`{"key":"val"}`)); err != nil {
		t.Fatalf("expected nil when no schema, got %v", err)
	}
}

func TestValidateInboundPayload_InvalidSchema(t *testing.T) {
	m := &TCPManager{
		config:    ClientConfig{ValidateInputPayloads: true},
		functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "fn1", InputSchema: "not-json"}},
	}
	if err := m.validateInboundPayload("fn1", []byte(`{"key":"val"}`)); err != nil {
		t.Fatalf("expected nil for invalid schema, got %v", err)
	}
}

func TestValidateInboundPayload_InvalidPayload(t *testing.T) {
	m := &TCPManager{
		config:    ClientConfig{ValidateInputPayloads: true},
		functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "fn1", InputSchema: `{"type":"object"}`}},
	}
	err := m.validateInboundPayload("fn1", []byte("not-json"))
	if err == nil {
		t.Fatal("expected error for invalid payload")
	}
}

func TestValidateInboundPayload_SchemaValidationFails(t *testing.T) {
	schema := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	m := &TCPManager{
		config:    ClientConfig{ValidateInputPayloads: true},
		functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "fn1", InputSchema: schema}},
	}
	err := m.validateInboundPayload("fn1", []byte(`{"age":123}`))
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// ---------------------------------------------------------------------------
// Reconnect — error paths
// ---------------------------------------------------------------------------

func TestReconnect_AlreadyConnected(t *testing.T) {
	config := ClientConfig{AgentAddr: "localhost:19090", Insecure: true}
	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)
	tcpMgr.connected = true

	err := tcpMgr.Reconnect(context.Background())
	if err != nil {
		t.Fatalf("expected nil when already connected, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// handleDisconnect — idempotent
// ---------------------------------------------------------------------------

func TestHandleDisconnect_Idempotent(t *testing.T) {
	config := ClientConfig{AgentAddr: "localhost:19090", Insecure: true}
	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)

	called := 0
	tcpMgr.onDisconnect = func() { called++ }
	tcpMgr.connected = false // already disconnected

	tcpMgr.handleDisconnect()
	if called != 0 {
		t.Fatalf("onDisconnect should not be called when already disconnected, got %d", called)
	}
}

func TestHandleDisconnect_WithCallback(t *testing.T) {
	config := ClientConfig{AgentAddr: "localhost:19090", Insecure: true}
	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)

	called := 0
	tcpMgr.onDisconnect = func() { called++ }
	tcpMgr.connected = true

	tcpMgr.handleDisconnect()
	if called != 1 {
		t.Fatalf("onDisconnect should be called once, got %d", called)
	}
	if tcpMgr.connected {
		t.Fatal("should be disconnected")
	}
}

// ---------------------------------------------------------------------------
// drainAndRecover — paths
// ---------------------------------------------------------------------------

func TestDrainAndRecover_NoReconnect(t *testing.T) {
	config := ClientConfig{AgentAddr: "localhost:19090", Insecure: true}
	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)
	tcpMgr.inflightCalls.Store(0)

	// No reconnect config — should just disconnect
	tcpMgr.drainAndRecover()
	if tcpMgr.draining.Load() {
		t.Fatal("draining should be reset after drainAndRecover")
	}
}

func TestDrainAndRecover_WithReconnect(t *testing.T) {
	config := ClientConfig{
		AgentAddr: "localhost:19090",
		Insecure:  true,
		Reconnect: &ReconnectConfig{Enabled: true},
		GameID:    "game",
		Env:       "prod",
	}
	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)
	tcpMgr.inflightCalls.Store(0)
	tcpMgr.serviceID = "svc"
	tcpMgr.serviceVersion = "1.0"

	// This will fail to reconnect (no server), but drainAndRecover should complete
	tcpMgr.drainAndRecover()
	if tcpMgr.draining.Load() {
		t.Fatal("draining should be reset")
	}
}

// ---------------------------------------------------------------------------
// SetOnDisconnect — basic
// ---------------------------------------------------------------------------

func TestSetOnDisconnect(t *testing.T) {
	config := ClientConfig{AgentAddr: "localhost:19090", Insecure: true}
	m, _ := NewTCPManager(config, nil)
	tcpMgr := m.(*TCPManager)

	called := false
	tcpMgr.SetOnDisconnect(func() { called = true })
	tcpMgr.connected = true // simulate connected state
	tcpMgr.handleDisconnect()
	if !called {
		t.Fatal("onDisconnect callback should have been called")
	}
}
