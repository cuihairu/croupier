package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	reg "github.com/cuihairu/croupier/internal/platform/registry"
)

func TestBuildOpsAgentSnapshot_NilSession(t *testing.T) {
	result := BuildOpsAgentSnapshot(nil)
	assert.Nil(t, result)
}

func TestBuildOpsAgentSnapshot_EmptyAgentID(t *testing.T) {
	sess := &reg.AgentSession{
		AgentID: "",
	}
	result := BuildOpsAgentSnapshot(sess)
	assert.Nil(t, result)
}

func TestBuildOpsAgentSnapshot_WhitespaceAgentID(t *testing.T) {
	sess := &reg.AgentSession{
		AgentID: "   ",
	}
	result := BuildOpsAgentSnapshot(sess)
	assert.Nil(t, result)
}

func TestBuildOpsAgentSnapshot_WithExpireAt(t *testing.T) {
	sess := &reg.AgentSession{
		AgentID:   "agent-1",
		GameID:    "game1",
		Env:       "dev",
		Addr:      "192.168.1.100:9090",
		Version:   "1.0.0",
		Region:    "us-east",
		Zone:      "zone-a",
		Labels:    map[string]string{"type": "worker"},
		ExpireAt:  time.Now().Add(5 * time.Minute),
		Functions: map[string]reg.FunctionMeta{"func1": {Enabled: true}, "func2": {Enabled: false}},
		Providers: []reg.ProviderSession{
			{ProviderID: "provider1", GameID: "game1", Env: "dev", FunctionIDs: []string{"func1"}},
		},
	}

	result := BuildOpsAgentSnapshot(sess)
	assert.NotNil(t, result)
	assert.Equal(t, "agent-1", result["id"])
	assert.Equal(t, "agent-1", result["agent_id"])
	assert.Equal(t, "game1", result["game_id"])
	assert.Equal(t, "dev", result["env"])
	assert.Equal(t, "worker", result["type"])
	assert.Equal(t, "192.168.1.100:9090", result["addr"])
	assert.Equal(t, "192.168.1.100:9090", result["rpc_addr"])
	assert.Equal(t, "192.168.1.100", result["ip"])
	assert.Equal(t, "1.0.0", result["version"])
	assert.Equal(t, "us-east", result["region"])
	assert.Equal(t, "zone-a", result["zone"])
	assert.Equal(t, true, result["healthy"])
	assert.Equal(t, 1, result["functions"])
	assert.Equal(t, 1, result["providers_count"])
}

func TestBuildOpsAgentSnapshot_ExpiredSession(t *testing.T) {
	sess := &reg.AgentSession{
		AgentID:  "agent-1",
		ExpireAt: time.Now().Add(-1 * time.Minute),
	}

	result := BuildOpsAgentSnapshot(sess)
	assert.NotNil(t, result)
	assert.Equal(t, false, result["healthy"])
	assert.Equal(t, 0, result["expires_in_sec"])
}

func TestBuildOpsAgentSnapshot_ZeroExpireAt(t *testing.T) {
	sess := &reg.AgentSession{
		AgentID:  "agent-1",
		ExpireAt: time.Time{},
	}

	result := BuildOpsAgentSnapshot(sess)
	assert.NotNil(t, result)
	assert.Equal(t, false, result["healthy"])
	assert.Equal(t, 0, result["expires_in_sec"])
}

func TestBuildOpsAgentSnapshot_NilLabels(t *testing.T) {
	sess := &reg.AgentSession{
		AgentID:  "agent-1",
		Labels:   nil,
		ExpireAt: time.Now().Add(5 * time.Minute),
	}

	result := BuildOpsAgentSnapshot(sess)
	assert.NotNil(t, result)
	assert.Equal(t, "agent", result["type"]) // default type
}

func TestBuildOpsAgentSnapshot_NilFunctions(t *testing.T) {
	sess := &reg.AgentSession{
		AgentID:   "agent-1",
		Functions: nil,
		ExpireAt:  time.Now().Add(5 * time.Minute),
	}

	result := BuildOpsAgentSnapshot(sess)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result["functions"])
}

func TestBuildOpsAgentSnapshot_NilProviders(t *testing.T) {
	sess := &reg.AgentSession{
		AgentID:   "agent-1",
		Providers: nil,
		ExpireAt:  time.Now().Add(5 * time.Minute),
	}

	result := BuildOpsAgentSnapshot(sess)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result["providers_count"])
}

func TestBuildProviders_EmptyProviders(t *testing.T) {
	result := buildProviders(nil)
	assert.Nil(t, result)

	result = buildProviders([]reg.ProviderSession{})
	assert.Nil(t, result)
}

func TestBuildProviders_WithEmptyProviderID(t *testing.T) {
	providers := []reg.ProviderSession{
		{ProviderID: "", GameID: "game1"},
		{ProviderID: "valid", GameID: "game1", FunctionIDs: []string{"func1"}},
	}

	result := buildProviders(providers)
	assert.Len(t, result, 1) // empty provider ID should be skipped
	assert.Equal(t, "valid", result[0]["provider_id"])
}

func TestBuildProviders_WhitespaceProviderID(t *testing.T) {
	providers := []reg.ProviderSession{
		{ProviderID: "   ", GameID: "game1"},
	}

	result := buildProviders(providers)
	assert.Len(t, result, 0) // whitespace provider ID should be skipped
}

func TestBuildProviders_NilFunctionIDs(t *testing.T) {
	providers := []reg.ProviderSession{
		{ProviderID: "provider1", FunctionIDs: nil},
	}

	result := buildProviders(providers)
	assert.Len(t, result, 1)
	assert.Equal(t, 0, result[0]["functions"])
}

func TestCountEnabledFunctions_NilMap(t *testing.T) {
	result := CountEnabledFunctions(nil)
	assert.Equal(t, 0, result)
}

func TestCountEnabledFunctions_MixedEnabled(t *testing.T) {
	functions := map[string]reg.FunctionMeta{
		"func1": {Enabled: true},
		"func2": {Enabled: false},
		"func3": {Enabled: true},
	}

	result := CountEnabledFunctions(functions)
	assert.Equal(t, 2, result)
}

func TestFirstNonEmpty_AllEmpty(t *testing.T) {
	result := firstNonEmpty("", "", "")
	assert.Equal(t, "", result)
}

func TestFirstNonEmpty_FirstNonEmpty(t *testing.T) {
	result := firstNonEmpty("first", "second", "third")
	assert.Equal(t, "first", result)
}

func TestFirstNonEmpty_MiddleNonEmpty(t *testing.T) {
	result := firstNonEmpty("", "second", "third")
	assert.Equal(t, "second", result)
}

func TestFirstNonEmpty_LastNonEmpty(t *testing.T) {
	result := firstNonEmpty("", "", "third")
	assert.Equal(t, "third", result)
}

func TestFirstNonEmpty_WhitespaceOnly(t *testing.T) {
	result := firstNonEmpty("  ", "  ", "  ")
	assert.Equal(t, "", result)
}

func TestGuessAgentIP_EmptyAddr(t *testing.T) {
	result := guessAgentIP("")
	assert.Equal(t, "", result)
}

func TestGuessAgentIP_WhitespaceAddr(t *testing.T) {
	result := guessAgentIP("   ")
	assert.Equal(t, "", result)
}

func TestGuessAgentIP_WithPort(t *testing.T) {
	result := guessAgentIP("192.168.1.100:9090")
	assert.Equal(t, "192.168.1.100", result)
}

func TestGuessAgentIP_WithGRPCScheme(t *testing.T) {
	result := guessAgentIP("grpc:///192.168.1.100:9090")
	assert.Equal(t, "192.168.1.100", result)
}

func TestGuessAgentIP_NoPort(t *testing.T) {
	result := guessAgentIP("192.168.1.100")
	assert.Equal(t, "192.168.1.100", result)
}

func TestGuessAgentIP_WithColonButNoPort(t *testing.T) {
	result := guessAgentIP("192.168.1.100:")
	assert.Equal(t, "192.168.1.100", result)
}

func TestGuessAgentLastSeen_ZeroTime(t *testing.T) {
	result := guessAgentLastSeen(time.Time{})
	assert.Equal(t, "", result)
}

func TestGuessAgentLastSeen_ValidTime(t *testing.T) {
	expireAt := time.Now().Add(5 * time.Minute)
	result := guessAgentLastSeen(expireAt)
	assert.NotEmpty(t, result)
}

func TestInjectMetrics_NilLabels(t *testing.T) {
	snapshot := map[string]interface{}{}
	injectMetrics(snapshot, nil)
	// Should not panic and snapshot should be unchanged
	assert.Empty(t, snapshot)
}

func TestInjectMetrics_WithAllMetrics(t *testing.T) {
	snapshot := map[string]interface{}{}
	labels := map[string]string{
		"stats.qps_1m":          "100.5",
		"stats.error_rate":      "0.01",
		"stats.avg_latency_ms":  "50.2",
		"stats.qps_limit":       "200",
		"stats.active_conns":    "10",
		"stats.total_requests":  "1000",
		"stats.failed_requests": "5",
	}

	injectMetrics(snapshot, labels)
	assert.Equal(t, 100.5, snapshot["qps_1m"])
	assert.Equal(t, 0.01, snapshot["error_rate"])
	assert.Equal(t, 50.2, snapshot["avg_latency_ms"])
	assert.Equal(t, 200.0, snapshot["qps_limit"])
	assert.Equal(t, int64(10), snapshot["active_conns"])
	assert.Equal(t, int64(1000), snapshot["total_requests"])
	assert.Equal(t, int64(5), snapshot["failed_requests"])
}

func TestInjectMetrics_InvalidValues(t *testing.T) {
	snapshot := map[string]interface{}{}
	labels := map[string]string{
		"stats.qps_1m":       "not_a_number",
		"stats.active_conns": "invalid",
	}

	injectMetrics(snapshot, labels)
	assert.NotContains(t, snapshot, "qps_1m")
	assert.NotContains(t, snapshot, "active_conns")
}

func TestEnsureMetricDefaults_AllMissing(t *testing.T) {
	snapshot := map[string]interface{}{}
	ensureMetricDefaults(snapshot)

	assert.Equal(t, 0.0, snapshot["qps_1m"])
	assert.Equal(t, 0.0, snapshot["error_rate"])
	assert.Equal(t, 0.0, snapshot["avg_latency_ms"])
	assert.Equal(t, 0.0, snapshot["qps_limit"])
	assert.Equal(t, int64(0), snapshot["active_conns"])
	assert.Equal(t, int64(0), snapshot["total_requests"])
	assert.Equal(t, int64(0), snapshot["failed_requests"])
}

func TestEnsureMetricDefaults_SomeExisting(t *testing.T) {
	snapshot := map[string]interface{}{
		"qps_1m":       100.0,
		"active_conns": int64(5),
	}
	ensureMetricDefaults(snapshot)

	assert.Equal(t, 100.0, snapshot["qps_1m"])          // preserved
	assert.Equal(t, int64(5), snapshot["active_conns"]) // preserved
	assert.Equal(t, 0.0, snapshot["error_rate"])        // added default
	assert.Equal(t, 0.0, snapshot["avg_latency_ms"])    // added default
}

func TestParseFloatLabel_NilLabels(t *testing.T) {
	result, ok := parseFloatLabel(nil, "key")
	assert.False(t, ok)
	assert.Equal(t, 0.0, result)
}

func TestParseFloatLabel_MissingKey(t *testing.T) {
	labels := map[string]string{"other": "100"}
	result, ok := parseFloatLabel(labels, "key")
	assert.False(t, ok)
	assert.Equal(t, 0.0, result)
}

func TestParseFloatLabel_InvalidValue(t *testing.T) {
	labels := map[string]string{"key": "not_a_number"}
	result, ok := parseFloatLabel(labels, "key")
	assert.False(t, ok)
	assert.Equal(t, 0.0, result)
}

func TestParseFloatLabel_ValidValue(t *testing.T) {
	labels := map[string]string{"key": "100.5"}
	result, ok := parseFloatLabel(labels, "key")
	assert.True(t, ok)
	assert.Equal(t, 100.5, result)
}

func TestParseFloatLabel_WithWhitespace(t *testing.T) {
	labels := map[string]string{"key": "  100.5  "}
	result, ok := parseFloatLabel(labels, "key")
	assert.True(t, ok)
	assert.Equal(t, 100.5, result)
}

func TestParseFloatLabel_FallbackKey(t *testing.T) {
	labels := map[string]string{"fallback": "100.5"}
	result, ok := parseFloatLabel(labels, "primary", "fallback")
	assert.True(t, ok)
	assert.Equal(t, 100.5, result)
}

func TestParseIntLabel_NilLabels(t *testing.T) {
	result, ok := parseIntLabel(nil, "key")
	assert.False(t, ok)
	assert.Equal(t, int64(0), result)
}

func TestParseIntLabel_MissingKey(t *testing.T) {
	labels := map[string]string{"other": "100"}
	result, ok := parseIntLabel(labels, "key")
	assert.False(t, ok)
	assert.Equal(t, int64(0), result)
}

func TestParseIntLabel_InvalidValue(t *testing.T) {
	labels := map[string]string{"key": "not_a_number"}
	result, ok := parseIntLabel(labels, "key")
	assert.False(t, ok)
	assert.Equal(t, int64(0), result)
}

func TestParseIntLabel_ValidValue(t *testing.T) {
	labels := map[string]string{"key": "100"}
	result, ok := parseIntLabel(labels, "key")
	assert.True(t, ok)
	assert.Equal(t, int64(100), result)
}

func TestParseIntLabel_WithWhitespace(t *testing.T) {
	labels := map[string]string{"key": "  100  "}
	result, ok := parseIntLabel(labels, "key")
	assert.True(t, ok)
	assert.Equal(t, int64(100), result)
}

func TestParseIntLabel_FallbackKey(t *testing.T) {
	labels := map[string]string{"fallback": "100"}
	result, ok := parseIntLabel(labels, "primary", "fallback")
	assert.True(t, ok)
	assert.Equal(t, int64(100), result)
}
