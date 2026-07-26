package croupier

import (
	"context"
	"testing"
)

func TestGenerateUUID(t *testing.T) {
	uuid := generateUUID()
	if len(uuid) != 36 {
		t.Errorf("expected UUID length 36, got %d: %q", len(uuid), uuid)
	}

	// Verify format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	if uuid[8] != '-' || uuid[13] != '-' || uuid[18] != '-' || uuid[23] != '-' {
		t.Errorf("invalid UUID format: %q", uuid)
	}

	// Verify uniqueness
	uuid2 := generateUUID()
	if uuid == uuid2 {
		t.Errorf("expected unique UUIDs, got same: %q", uuid)
	}
}

func TestTypes_DefaultClientConfig(t *testing.T) {
	cfg := DefaultClientConfig()

	if cfg.AgentAddr != "localhost:19090" {
		t.Errorf("expected AgentAddr=localhost:19090, got %s", cfg.AgentAddr)
	}
	if cfg.Env != "development" {
		t.Errorf("expected Env=development, got %s", cfg.Env)
	}
	if cfg.ServiceVersion != "1.0.0" {
		t.Errorf("expected ServiceVersion=1.0.0, got %s", cfg.ServiceVersion)
	}
	if cfg.TimeoutSeconds != 30 {
		t.Errorf("expected TimeoutSeconds=30, got %d", cfg.TimeoutSeconds)
	}
	if cfg.HeartbeatInterval != 60 {
		t.Errorf("expected HeartbeatInterval=60, got %d", cfg.HeartbeatInterval)
	}
	if !cfg.Insecure {
		t.Error("expected Insecure=true")
	}
	if cfg.ProviderLang != "go" {
		t.Errorf("expected ProviderLang=go, got %s", cfg.ProviderLang)
	}
	if cfg.ProviderSDK != "croupier-go-sdk" {
		t.Errorf("expected ProviderSDK=croupier-go-sdk, got %s", cfg.ProviderSDK)
	}
	if cfg.LogLevel != "INFO" {
		t.Errorf("expected LogLevel=INFO, got %s", cfg.LogLevel)
	}
	if cfg.MaxFileSize != 10*1024*1024 {
		t.Errorf("expected MaxFileSize=10MB, got %d", cfg.MaxFileSize)
	}
	if cfg.Headers == nil {
		t.Error("expected Headers to be initialized")
	}
	if cfg.Reconnect == nil {
		t.Error("expected Reconnect config")
	}
	if cfg.Retry == nil {
		t.Error("expected Retry config")
	}
	if cfg.ServiceID == "" {
		t.Error("expected ServiceID to be set")
	}
}

func TestHandlerWithDescriptor_Handle(t *testing.T) {
	called := false
	handler := &HandlerWithDescriptor{
		Handler: func(ctx context.Context, payload []byte) ([]byte, error) {
			called = true
			return []byte("result"), nil
		},
		Descriptor: &LocalFunctionDescriptor{
			ID:      "test.func",
			Version: "1.0.0",
		},
	}

	result, err := handler.Handle(context.Background(), []byte("input"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler was not called")
	}
	if string(result) != "result" {
		t.Errorf("expected 'result', got %q", string(result))
	}
}

func TestHandlerWithDescriptor_GetDescriptor(t *testing.T) {
	desc := &LocalFunctionDescriptor{
		ID:      "test.func",
		Version: "1.0.0",
	}
	handler := &HandlerWithDescriptor{
		Handler:    func(ctx context.Context, payload []byte) ([]byte, error) { return nil, nil },
		Descriptor: desc,
	}

	got := handler.GetDescriptor()
	if got != desc {
		t.Error("GetDescriptor should return the same descriptor")
	}
}

func TestTypes_FunctionDescriptor_Fields(t *testing.T) {
	fd := FunctionDescriptor{
		ID:        "player.ban",
		Version:   "1.0.0",
		Resource:  "player",
		Risk:      "high",
		Operation: "ban",
		Enabled:   true,
	}

	if fd.ID != "player.ban" {
		t.Errorf("expected ID=player.ban, got %s", fd.ID)
	}
	if fd.Risk != "high" {
		t.Errorf("expected Risk=high, got %s", fd.Risk)
	}
}

func TestLocalFunctionDescriptor_Fields(t *testing.T) {
	lfd := LocalFunctionDescriptor{
		ID:           "player.ban",
		Version:      "1.0.0",
		Tags:         []string{"player", "admin"},
		Summary:      "Ban a player",
		Description:  "Ban a player from the game",
		OperationID:  "banPlayer",
		Deprecated:   false,
		InputSchema:  `{"type":"object"}`,
		OutputSchema: `{"type":"object"}`,
		Resource:     "player",
		Risk:         "danger",
		Operation:    "ban",
	}

	if lfd.ID != "player.ban" {
		t.Errorf("expected ID=player.ban, got %s", lfd.ID)
	}
	if len(lfd.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(lfd.Tags))
	}
	if lfd.Risk != "danger" {
		t.Errorf("expected Risk=danger, got %s", lfd.Risk)
	}
}

func TestTaskEvent_Fields(t *testing.T) {
	event := TaskEvent{
		EventType: "progress",
		TaskID:    "task-123",
		Payload:   `{"pct":50}`,
		Error:     "",
		Done:      false,
	}

	if event.EventType != "progress" {
		t.Errorf("expected EventType=progress, got %s", event.EventType)
	}
	if event.TaskID != "task-123" {
		t.Errorf("expected TaskID=task-123, got %s", event.TaskID)
	}
	if event.Done {
		t.Error("expected Done=false")
	}
}

func TestInvokerConfig_Fields(t *testing.T) {
	cfg := InvokerConfig{
		Address:        "localhost:19090",
		TimeoutSeconds: 60,
		Insecure:       true,
		CAFile:         "/path/to/ca",
		CertFile:       "/path/to/cert",
		KeyFile:        "/path/to/key",
		Reconnect:      DefaultReconnectConfig(),
		Retry:          DefaultRetryConfig(),
	}

	if cfg.Address != "localhost:19090" {
		t.Errorf("expected Address=localhost:19090, got %s", cfg.Address)
	}
	if cfg.CAFile != "/path/to/ca" {
		t.Errorf("expected CAFile=/path/to/ca, got %s", cfg.CAFile)
	}
}
