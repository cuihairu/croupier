package croupier

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ValidateClientConfig
// ---------------------------------------------------------------------------

func validClientConfig2() *ClientConfig {
	config := DefaultClientConfig()
	config.GameID = "game-default"
	return config
}

func TestValidateClientConfig2_AllBranches(t *testing.T) {
	if err := ValidateClientConfig(nil); err == nil || err.Error() != "config cannot be nil" {
		t.Fatalf("nil config error = %v", err)
	}

	required := []struct {
		mutate  func(*ClientConfig)
		message string
	}{
		{func(c *ClientConfig) { c.ServiceID = "" }, "service_id is required"},
		{func(c *ClientConfig) { c.GameID = "" }, "game_id is required"},
		{func(c *ClientConfig) { c.AgentAddr = "" }, "agent_addr is required"},
		{func(c *ClientConfig) { c.Env = "" }, "env is required"},
		{func(c *ClientConfig) { c.TimeoutSeconds = 0 }, "timeout_seconds must be positive"},
		{func(c *ClientConfig) { c.TimeoutSeconds = 10; c.HeartbeatInterval = 5 }, "must be >= timeout_seconds"},
		{func(c *ClientConfig) { c.HeartbeatInterval = -1 }, "heartbeat_interval must be positive"},
		{func(c *ClientConfig) { c.Insecure = false; c.CAFile = "" }, "CA file is required"},
		{func(c *ClientConfig) { c.CertFile = "cert.pem" }, "key_file is required"},
		{func(c *ClientConfig) { c.KeyFile = "key.pem" }, "cert_file is required"},
		{func(c *ClientConfig) { c.LogLevel = "VERBOSE" }, "invalid log_level"},
	}
	for _, tc := range required {
		config := validClientConfig2()
		tc.mutate(config)
		err := ValidateClientConfig(config)
		if err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Fatalf("expected error containing %q, got %v", tc.message, err)
		}
	}

	if err := ValidateClientConfig(validClientConfig2()); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
}

func TestValidateClientConfig2_LogLevelCaseInsensitive(t *testing.T) {
	for _, level := range []string{"DEBUG", "info", "warn", "ERROR", "off"} {
		config := validClientConfig2()
		config.LogLevel = level
		if err := ValidateClientConfig(config); err != nil {
			t.Fatalf("log level %q should be accepted: %v", level, err)
		}
	}
}

func TestValidateClientConfig2_ReconnectAndRetryPropagation(t *testing.T) {
	config := validClientConfig2()
	config.Reconnect.BackoffMultiplier = 0.5
	err := ValidateClientConfig(config)
	if err == nil || !strings.Contains(err.Error(), "reconnect config") {
		t.Fatalf("expected nested reconnect error, got %v", err)
	}

	config = validClientConfig2()
	config.Retry.MaxAttempts = 99
	err = ValidateClientConfig(config)
	if err == nil || !strings.Contains(err.Error(), "retry config") {
		t.Fatalf("expected nested retry error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ValidateFunctionDescriptor / ProviderFunctionDescriptor / TaskEvent
// ---------------------------------------------------------------------------

func TestValidateFunctionDescriptor2_Semver(t *testing.T) {
	for _, version := range []string{"1.2.3", "v1.2.3", "1.2.3-beta.1", "0.0.1+build.5"} {
		if err := ValidateFunctionDescriptor(&FunctionDescriptor{ID: "fn", Version: version}); err != nil {
			t.Fatalf("semver %q rejected: %v", version, err)
		}
	}
	for _, version := range []string{"1.2", "abc", "1.2.3.4"} {
		if err := ValidateFunctionDescriptor(&FunctionDescriptor{ID: "fn", Version: version}); err == nil {
			t.Fatalf("version %q should be rejected", version)
		}
	}
}

func TestValidateProviderFunctionDescriptor2_JSONSchemas(t *testing.T) {
	base := func() *ProviderFunctionDescriptor {
		return &ProviderFunctionDescriptor{ID: "fn", Version: "1.0.0"}
	}

	if err := ValidateProviderFunctionDescriptor(base()); err != nil {
		t.Fatalf("minimal descriptor rejected: %v", err)
	}

	badInput := base()
	badInput.InputSchema = "{not-json"
	if err := ValidateProviderFunctionDescriptor(badInput); err == nil {
		t.Fatal("invalid input schema should be rejected")
	}

	badOutput := base()
	badOutput.OutputSchema = "[unclosed"
	if err := ValidateProviderFunctionDescriptor(badOutput); err == nil {
		t.Fatal("invalid output schema should be rejected")
	}

	good := base()
	good.InputSchema = `{"type":"object"}`
	good.OutputSchema = `{"type":"string"}`
	if err := ValidateProviderFunctionDescriptor(good); err != nil {
		t.Fatalf("valid schemas rejected: %v", err)
	}

	noID := base()
	noID.ID = ""
	if err := ValidateProviderFunctionDescriptor(noID); err == nil {
		t.Fatal("missing ID should be rejected")
	}
}

func TestValidateTaskEvent2_SetsDoneForTerminalTypes(t *testing.T) {
	if err := ValidateTaskEvent(nil); err == nil {
		t.Fatal("nil event should be rejected")
	}

	for _, eventType := range []string{"started", "progress", "completed", "error"} {
		event := &TaskEvent{TaskID: "t1", EventType: eventType}
		if err := ValidateTaskEvent(event); err != nil {
			t.Fatalf("event type %q rejected: %v", eventType, err)
		}
		wantDone := eventType == "completed" || eventType == "error"
		if event.Done != wantDone {
			t.Fatalf("event %q Done = %v, want %v", eventType, event.Done, wantDone)
		}
	}

	invalid := &TaskEvent{TaskID: "t1", EventType: "mystery"}
	if err := ValidateTaskEvent(invalid); err == nil {
		t.Fatal("unknown event type should be rejected")
	}

	noTask := &TaskEvent{EventType: "started"}
	if err := ValidateTaskEvent(noTask); err == nil {
		t.Fatal("missing task ID should be rejected")
	}
}

// ---------------------------------------------------------------------------
// IsValidJSONSchema / isValidSemVer
// ---------------------------------------------------------------------------

func TestIsValidJSONSchema2(t *testing.T) {
	for _, schema := range []string{`{"type":"object"}`, `{"$schema":"http://json-schema.org/draft-07/schema#"}`, `{"properties":{}}`} {
		if !IsValidJSONSchema(schema) {
			t.Fatalf("schema %q should be valid", schema)
		}
	}
	for _, schema := range []string{"", "{", "not json", `{"a":}`, `[]`, `"string"`, `null`, `42`, `{}`} {
		if IsValidJSONSchema(schema) {
			t.Fatalf("schema %q should be invalid", schema)
		}
	}
}

// ---------------------------------------------------------------------------
// RetryConfig helpers
// ---------------------------------------------------------------------------

func TestRetryConfigIsRetryable2(t *testing.T) {
	var nilRetry *RetryConfig
	if nilRetry.IsRetryable(14) {
		t.Fatal("nil retry config must never retry")
	}

	disabled := &RetryConfig{Enabled: false, RetryableStatusCodes: []int32{14}}
	if disabled.IsRetryable(14) {
		t.Fatal("disabled retry config must never retry")
	}

	enabled := DefaultRetryConfig()
	for _, code := range []int32{14, 13, 2, 10, 4} {
		if !enabled.IsRetryable(code) {
			t.Fatalf("default retryable status %d should retry", code)
		}
	}
	for _, code := range []int32{200, 404, 403} {
		if enabled.IsRetryable(code) {
			t.Fatalf("status %d should not retry", code)
		}
	}
}

func TestCalculateRetryDelay2_GrowsAndCaps(t *testing.T) {
	config := DefaultRetryConfig()
	previous := time.Duration(0)
	for attempt := 0; attempt < 6; attempt++ {
		delay := CalculateRetryDelay(config, attempt)
		base := time.Duration(config.InitialDelayMs) * time.Millisecond
		if delay < base/2 {
			t.Fatalf("attempt %d delay %v below half of base", attempt, delay)
		}
		max := time.Duration(float64(config.MaxDelayMs)*(1+config.JitterFactor)) * time.Millisecond
		if delay > max {
			t.Fatalf("attempt %d delay %v exceeds cap %v", attempt, delay, max)
		}
		if attempt > 0 && delay < previous/4 {
			t.Fatalf("delay dropped unexpectedly at attempt %d", attempt)
		}
		previous = delay
	}
}

// ---------------------------------------------------------------------------
// MergeWithDefaults / ApplyEnvOverrides
// ---------------------------------------------------------------------------

func TestMergeWithDefaults2_FillsOnlyMissingFields(t *testing.T) {
	partial := &ClientConfig{
		ServiceID: "custom-service",
		GameID:    "game-x",
		AgentAddr: "agent:19091",
		Env:       "production",
	}
	merged := MergeWithDefaults(partial)

	if merged.ServiceID != "custom-service" || merged.GameID != "game-x" {
		t.Fatalf("explicit values must be preserved: %+v", merged)
	}
	if merged.TimeoutSeconds != 30 || merged.HeartbeatInterval != 60 {
		t.Fatalf("defaults must be filled: %+v", merged)
	}
	if !merged.Insecure {
		t.Fatal("default should be insecure for development")
	}
	if merged.Reconnect == nil || merged.Retry == nil {
		t.Fatal("resiliency defaults must be filled")
	}
}

func TestMergeWithDefaults2_NilUsesAllDefaults(t *testing.T) {
	merged := MergeWithDefaults(nil)
	if merged.ServiceID == "" || merged.AgentAddr == "" {
		t.Fatalf("nil partial should produce full defaults: %+v", merged)
	}
}

func TestApplyEnvOverrides2(t *testing.T) {
	config := DefaultClientConfig()
	env := map[string]string{
		"CROUPIER_SERVICE_ID": "env-service",
		"CROUPIER_GAME_ID":    "env-game",
		"CROUPIER_AGENT_ADDR": "env-agent:1",
		"CROUPIER_ENV":        "staging",
		"CROUPIER_TIMEOUT":    "17",
		"CROUPIER_HEARTBEAT":  "180",
		"CROUPIER_LOG_LEVEL":  "debug",
	}
	overridden := ApplyEnvOverrides(config, env)

	if overridden.ServiceID != "env-service" || overridden.GameID != "env-game" {
		t.Fatalf("string overrides not applied: %+v", overridden)
	}
	if overridden.TimeoutSeconds != 17 || overridden.HeartbeatInterval != 180 {
		t.Fatalf("int overrides not applied: %+v", overridden)
	}
	if overridden.LogLevel != "DEBUG" {
		t.Fatalf("log level override not upper-cased: %q", overridden.LogLevel)
	}
}

func TestApplyEnvOverrides2_IgnoresUnknownAndMalformed(t *testing.T) {
	config := DefaultClientConfig()
	originalTimeout := config.TimeoutSeconds
	overridden := ApplyEnvOverrides(config, map[string]string{
		"CROUPIER_UNKNOWN_KEY": "value",
		"CROUPIER_TIMEOUT":     "not-a-number",
		"CROUPIER_HEARTBEAT":   "-5",
	})
	if overridden.TimeoutSeconds != originalTimeout {
		t.Fatalf("malformed timeout must be ignored: %d", overridden.TimeoutSeconds)
	}
	if overridden.HeartbeatInterval != config.HeartbeatInterval {
		t.Fatalf("non-positive heartbeat must be ignored: %d", overridden.HeartbeatInterval)
	}
}

// ---------------------------------------------------------------------------
// Default configs
// ---------------------------------------------------------------------------

func TestDefaultClientConfig2_Values(t *testing.T) {
	config := DefaultClientConfig()
	if !strings.HasPrefix(config.ServiceID, "go-sdk-") {
		t.Fatalf("service ID prefix = %q", config.ServiceID)
	}
	if config.ProviderLang != "go" || config.ProviderSDK != "croupier-go-sdk" {
		t.Fatalf("provider identity = %q/%q", config.ProviderLang, config.ProviderSDK)
	}
	if config.MaxFileSize != 10*1024*1024 {
		t.Fatalf("max file size = %d", config.MaxFileSize)
	}

	// Two calls generate distinct service IDs.
	if DefaultClientConfig().ServiceID == config.ServiceID {
		t.Fatal("service IDs should be unique per call")
	}
}

func TestDefaultReconnectConfig2_Values(t *testing.T) {
	reconnect := DefaultReconnectConfig()
	if !reconnect.Enabled || reconnect.MaxAttempts != 0 {
		t.Fatalf("unexpected reconnect defaults: %+v", reconnect)
	}
	if reconnect.InitialDelayMs != 1000 || reconnect.MaxDelayMs != 30000 {
		t.Fatalf("unexpected reconnect delays: %+v", reconnect)
	}
}

// ---------------------------------------------------------------------------
// HTTP invoker schema handling
// ---------------------------------------------------------------------------

func TestHTTPInvoker2_SetSchemaRejectsEmptyFunctionID(t *testing.T) {
	invoker := NewHTTPInvoker(&InvokerConfig{Address: "http://127.0.0.1:1"})
	defer invoker.Close()

	if err := invoker.SetSchema("", map[string]interface{}{}); err == nil {
		t.Fatal("empty function ID must be rejected")
	}
	if err := invoker.SetSchema("fn", map[string]interface{}{"type": "object"}); err != nil {
		t.Fatalf("valid schema rejected: %v", err)
	}
}

func TestValidateFunctionID2(t *testing.T) {
	for _, id := range []string{"", "   "} {
		if err := validateFunctionID(id); err == nil {
			t.Fatalf("function ID %q should be rejected", id)
		}
	}
	if err := validateFunctionID("player.ban"); err != nil {
		t.Fatalf("valid ID rejected: %v", err)
	}
}
