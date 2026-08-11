package agent

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStartLocalServerWiresLocalHandlerToUpstream(t *testing.T) {
	app := NewWithConfigDir("", "agent-1", t.TempDir())
	app.SetLocalAddr("127.0.0.1:0")
	t.Cleanup(app.Stop)

	if err := app.StartLocalServer(); err != nil {
		t.Fatalf("StartLocalServer() error = %v", err)
	}

	if app.localHandler == nil {
		t.Fatal("expected local handler to be initialized")
	}
	if app.upstream == nil {
		t.Fatal("expected upstream client to be initialized")
	}
	if app.upstream.localHandler != app.localHandler {
		t.Fatal("expected upstream to use the local handler for bidirectional session requests")
	}
	if app.GetLocalServerAddr() == "" {
		t.Fatal("expected local server address")
	}
}

func TestParseDurationEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		def      time.Duration
		expected time.Duration
	}{
		{
			name:     "empty env",
			key:      "TEST_EMPTY_DURATION",
			value:    "",
			def:      5 * time.Second,
			expected: 5 * time.Second,
		},
		{
			name:     "valid duration",
			key:      "TEST_VALID_DURATION",
			value:    "10s",
			def:      5 * time.Second,
			expected: 10 * time.Second,
		},
		{
			name:     "invalid duration",
			key:      "TEST_INVALID_DURATION",
			value:    "invalid",
			def:      5 * time.Second,
			expected: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				t.Setenv(tt.key, tt.value)
			} else {
				os.Unsetenv(tt.key)
			}
			result := parseDurationEnv(tt.key, tt.def)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetConfigDir(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "empty env",
			value:    "",
			expected: "./configs",
		},
		{
			name:     "with value",
			value:    "/tmp/test",
			expected: "/tmp/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				t.Setenv("CROUPIER_CONFIG_DIR", tt.value)
			} else {
				os.Unsetenv("CROUPIER_CONFIG_DIR")
			}
			result := getConfigDir()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWithTelemetry(t *testing.T) {
	app := &App{}
	app.WithTelemetry(nil)
	assert.NotNil(t, app)
}

func TestWithUpstreamTLSConfig(t *testing.T) {
	app := &App{}
	app.WithUpstreamTLSConfig(nil)
	assert.NotNil(t, app)
}

func TestSetUpstreamTransportKind(t *testing.T) {
	app := &App{}
	app.SetUpstreamTransportKind("tcp")
	assert.NotNil(t, app)
}

func TestWithOutboundTLSConfig(t *testing.T) {
	app := &App{}
	app.WithOutboundTLSConfig(nil)
	assert.NotNil(t, app)
}

func TestWithOpsConfig(t *testing.T) {
	app := &App{}
	app.WithOpsConfig(nil)
	assert.NotNil(t, app)
}
