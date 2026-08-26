package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFeatureFlagsConfig_Enabled(t *testing.T) {
	// nil config: everything on (fail-open).
	var nilFlags FeatureFlagsConfig
	assert.True(t, nilFlags.Enabled(FlagDev))
	assert.True(t, nilFlags.Enabled("unknown-flag"))

	// Explicit false disables; unset stays on.
	flags := FeatureFlagsConfig{FlagDev: false, FlagAnalytics: true}
	assert.False(t, flags.Enabled(FlagDev))
	assert.True(t, flags.Enabled(FlagAnalytics))
	assert.True(t, flags.Enabled(FlagSupport), "unset flag must default to enabled")
	assert.True(t, flags.Enabled("typo-flag"), "unknown flag must fail open")
}

func TestUnmarshalConfig_FeatureFlags(t *testing.T) {
	raw := `
server:
  mode: prod
featureFlags:
  dev: false
  analytics: false
`
	var c Config
	require.NoError(t, yaml.Unmarshal([]byte(raw), &c))
	assert.False(t, c.FeatureFlags.Enabled(FlagDev))
	assert.False(t, c.FeatureFlags.Enabled(FlagAnalytics))
	assert.True(t, c.FeatureFlags.Enabled(FlagSupport), "unset domains default on")
}

func TestUnmarshalConfig_FeatureFlagsAbsent(t *testing.T) {
	raw := `
server:
  mode: prod
`
	var c Config
	require.NoError(t, yaml.Unmarshal([]byte(raw), &c))
	for _, flag := range []string{FlagDev, FlagSupport, FlagAnalytics, FlagOps, FlagExtensions} {
		assert.True(t, c.FeatureFlags.Enabled(flag), "%s must default on when featureFlags absent", flag)
	}
}
