package croupier

import (
	"strings"
	"testing"
	"time"
)

// validClientConfig returns a config that passes ValidateClientConfig.
func validClientConfig() *ClientConfig {
	return &ClientConfig{
		ServiceID:         "svc-1",
		GameID:            "game-1",
		AgentAddr:         "127.0.0.1:19091",
		Env:               "dev",
		TimeoutSeconds:    30,
		HeartbeatInterval: 60,
		Insecure:          true,
		LogLevel:          "INFO",
	}
}

func TestValidateClientConfig_NilConfig(t *testing.T) {
	t.Parallel()

	if err := ValidateClientConfig(nil); err == nil || !strings.Contains(err.Error(), "config cannot be nil") {
		t.Fatalf("ValidateClientConfig(nil) error = %v, want nil config rejection", err)
	}
}

func TestValidateClientConfig_NestedConfigErrors(t *testing.T) {
	t.Parallel()

	badReconnect := validClientConfig()
	badReconnect.Reconnect = &ReconnectConfig{InitialDelayMs: -1}
	if err := ValidateClientConfig(badReconnect); err == nil || !strings.Contains(err.Error(), "invalid reconnect config") {
		t.Fatalf("ValidateClientConfig() error = %v, want nested reconnect rejection", err)
	}

	badRetry := validClientConfig()
	badRetry.Retry = &RetryConfig{Enabled: true, MaxAttempts: 99}
	if err := ValidateClientConfig(badRetry); err == nil || !strings.Contains(err.Error(), "invalid retry config") {
		t.Fatalf("ValidateClientConfig() error = %v, want nested retry rejection", err)
	}

	if err := ValidateClientConfig(validClientConfig()); err != nil {
		t.Fatalf("ValidateClientConfig(valid) error = %v", err)
	}
}

func TestValidateReconnectConfig_NilAndNegativeMaxDelay(t *testing.T) {
	t.Parallel()

	if err := ValidateReconnectConfig(nil); err != nil {
		t.Fatalf("ValidateReconnectConfig(nil) error = %v, want nil", err)
	}
	if err := ValidateReconnectConfig(&ReconnectConfig{MaxDelayMs: -5}); err == nil ||
		!strings.Contains(err.Error(), "max_delay_ms cannot be negative") {
		t.Fatalf("ValidateReconnectConfig() error = %v, want negative max delay rejection", err)
	}
}

func TestValidateInvokerConfig_NilConfig(t *testing.T) {
	t.Parallel()

	if err := ValidateInvokerConfig(nil); err == nil || !strings.Contains(err.Error(), "config cannot be nil") {
		t.Fatalf("ValidateInvokerConfig(nil) error = %v, want nil config rejection", err)
	}
}

func TestValidateTaskEvent_NilAndMissingTaskID(t *testing.T) {
	t.Parallel()

	if err := ValidateTaskEvent(nil); err == nil || !strings.Contains(err.Error(), "event cannot be nil") {
		t.Fatalf("ValidateTaskEvent(nil) error = %v, want nil event rejection", err)
	}
	if err := ValidateTaskEvent(&TaskEvent{EventType: "started"}); err == nil ||
		!strings.Contains(err.Error(), "task_id is required") {
		t.Fatalf("ValidateTaskEvent() error = %v, want missing task ID rejection", err)
	}

	completed := &TaskEvent{TaskID: "task-1", EventType: "completed"}
	if err := ValidateTaskEvent(completed); err != nil {
		t.Fatalf("ValidateTaskEvent(completed) error = %v", err)
	}
	if !completed.Done {
		t.Fatal("completed event must set Done")
	}
}

func TestCalculateReconnectDelay_NilConfig(t *testing.T) {
	t.Parallel()

	if got := CalculateReconnectDelay(nil, 3); got != 0 {
		t.Fatalf("CalculateReconnectDelay(nil, 3) = %v, want 0", got)
	}
}

func TestMergeWithDefaults_FullOverride(t *testing.T) {
	t.Parallel()

	partial := &ClientConfig{
		ServiceID:          "svc-override",
		GameID:             "game-override",
		AgentAddr:          "10.0.0.1:19091",
		AgentIPCAddr:       "unix:///tmp/croupier.sock",
		Env:                "production",
		ServiceVersion:     "2.0.0",
		TimeoutSeconds:     11,
		HeartbeatInterval:  22,
		Insecure:           true,
		CAFile:             "/tmp/ca.pem",
		CertFile:           "/tmp/cert.pem",
		KeyFile:            "/tmp/key.pem",
		ServerName:         "agent.internal",
		InsecureSkipVerify: true,
		AuthToken:          "token-override",
		LogLevel:           "DEBUG",
		DisableLogging:     true,
		DebugLogging:       true,
		Headers:            map[string]string{"X-Custom": "yes"},
		Reconnect:          &ReconnectConfig{Enabled: false, MaxAttempts: 4},
		Retry:              &RetryConfig{Enabled: false, MaxAttempts: 6},
		EnableFileTransfer: true,
		MaxFileSize:        2048,
	}

	merged := MergeWithDefaults(partial)
	checks := map[string]struct {
		got, want interface{}
	}{
		"ServiceID":          {merged.ServiceID, "svc-override"},
		"GameID":             {merged.GameID, "game-override"},
		"AgentAddr":          {merged.AgentAddr, "10.0.0.1:19091"},
		"AgentIPCAddr":       {merged.AgentIPCAddr, "unix:///tmp/croupier.sock"},
		"Env":                {merged.Env, "production"},
		"ServiceVersion":     {merged.ServiceVersion, "2.0.0"},
		"TimeoutSeconds":     {merged.TimeoutSeconds, 11},
		"HeartbeatInterval":  {merged.HeartbeatInterval, 22},
		"Insecure":           {merged.Insecure, true},
		"CAFile":             {merged.CAFile, "/tmp/ca.pem"},
		"CertFile":           {merged.CertFile, "/tmp/cert.pem"},
		"KeyFile":            {merged.KeyFile, "/tmp/key.pem"},
		"ServerName":         {merged.ServerName, "agent.internal"},
		"InsecureSkipVerify": {merged.InsecureSkipVerify, true},
		"AuthToken":          {merged.AuthToken, "token-override"},
		"LogLevel":           {merged.LogLevel, "DEBUG"},
		"DisableLogging":     {merged.DisableLogging, true},
		"DebugLogging":       {merged.DebugLogging, true},
		"MaxFileSize":        {merged.MaxFileSize, 2048},
	}
	for name, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %v, want %v", name, check.got, check.want)
		}
	}
	if merged.Reconnect != partial.Reconnect || merged.Retry != partial.Retry {
		t.Error("nested configs were not replaced by the partial values")
	}
	if merged.Headers["X-Custom"] != "yes" {
		t.Errorf("custom header not merged: %v", merged.Headers)
	}
	if !merged.EnableFileTransfer {
		t.Error("EnableFileTransfer not merged")
	}

	if got := MergeWithDefaults(nil); got.AgentAddr != "localhost:19091" || got.TimeoutSeconds != 30 {
		t.Fatalf("MergeWithDefaults(nil) = %+v, want defaults", got)
	}
}

func TestApplyEnvOverrides_NilConfig(t *testing.T) {
	t.Parallel()

	merged := ApplyEnvOverrides(nil, map[string]string{"CROUPIER_AGENT_ADDR": "127.0.0.1:19999"})
	if merged.AgentAddr != "127.0.0.1:19999" {
		t.Fatalf("AgentAddr = %q, want env override", merged.AgentAddr)
	}
}

func TestApplyEnvOverrides_ReconnectEnablesAndMaxAttempts(t *testing.T) {
	t.Parallel()

	enabled := ApplyEnvOverrides(&ClientConfig{}, map[string]string{"CROUPIER_RECONNECT": "true"})
	if enabled.Reconnect == nil || !enabled.Reconnect.Enabled {
		t.Fatalf("CROUPIER_RECONNECT=true did not enable reconnection: %+v", enabled.Reconnect)
	}

	capped := ApplyEnvOverrides(&ClientConfig{}, map[string]string{"CROUPIER_RECONNECT_MAX": "5"})
	if capped.Reconnect == nil || capped.Reconnect.MaxAttempts != 5 {
		t.Fatalf("CROUPIER_RECONNECT_MAX=5 not applied: %+v", capped.Reconnect)
	}
}

func TestClientConfigClone_NilReceiver(t *testing.T) {
	t.Parallel()

	var config *ClientConfig
	if clone := config.Clone(); clone != nil {
		t.Fatalf("Clone() on nil receiver = %+v, want nil", clone)
	}
	if copy := config.DeepCopy(); copy != nil {
		t.Fatalf("DeepCopy() on nil receiver = %+v, want nil", copy)
	}
}

func TestRetryConfigIsRetryable_NilAndDisabled(t *testing.T) {
	t.Parallel()

	var retry *RetryConfig
	if retry.IsRetryable(500) {
		t.Fatal("nil RetryConfig must not be retryable")
	}
	if got := (&RetryConfig{Enabled: false, RetryableStatusCodes: []int32{500}}).IsRetryable(500); got {
		t.Fatal("disabled RetryConfig must not be retryable")
	}
	if got := (&RetryConfig{Enabled: true, RetryableStatusCodes: []int32{500}}).IsRetryable(500); !got {
		t.Fatal("listed status code must be retryable")
	}
}

func TestInvokeOptionsWithDefaults_NilOptions(t *testing.T) {
	t.Parallel()

	defaulted := (*InvokeOptions)(nil).WithDefaults()
	if defaulted == nil {
		t.Fatal("WithDefaults(nil) = nil, want fresh options")
	}
	if defaulted.Timeout != 30*time.Second {
		t.Fatalf("Timeout = %v, want 30s default", defaulted.Timeout)
	}
}
