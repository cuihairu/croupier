package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLayered_FeatureEnabled_Composition(t *testing.T) {
	resetForTest()
	store := newStore(t)

	t.Run("L2 disabled wins over L3", func(t *testing.T) {
		resetForTest()
		l := InitLayered(context.Background(), &ConfigInput{
			FeatureFlags: map[string]bool{"analytics": false},
		}, newStore(t))
		// L3 试图开启（不允许）：合成仍为 false。
		l.l3 = map[string]json.RawMessage{KeyFeatureAnalytics: json.RawMessage(`true`)}
		l.l3Loaded = true
		assert.False(t, l.FeatureEnabled("analytics"))
	})

	t.Run("L2 enabled, L3 disables", func(t *testing.T) {
		resetForTest()
		l := InitLayered(context.Background(), &ConfigInput{}, newStore(t))
		assert.True(t, l.FeatureEnabled("ops"))
		l.l3 = map[string]json.RawMessage{KeyFeatureOps: json.RawMessage(`false`)}
		l.l3Loaded = true
		assert.False(t, l.FeatureEnabled("ops"))
	})

	t.Run("unset everywhere fail-open", func(t *testing.T) {
		l := InitLayered(context.Background(), &ConfigInput{}, store)
		assert.True(t, l.FeatureEnabled("dev"))
		assert.True(t, l.FeatureEnabled("unknown-flag"))
	})

	t.Run("L2 disabled only", func(t *testing.T) {
		resetForTest()
		l := InitLayered(context.Background(), &ConfigInput{
			FeatureFlags: map[string]bool{"support": false},
		}, newStore(t))
		assert.False(t, l.FeatureEnabled("support"))
	})
}

func TestLayered_FeatureRoundTripThroughStore(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	require.NoError(t, store.Set(context.Background(), KeyFeatureDev, json.RawMessage(`false`), "admin"))
	l.Reload(context.Background(), store)
	assert.False(t, l.FeatureEnabled("dev"))

	require.NoError(t, store.Clear(context.Background(), KeyFeatureDev))
	l.Reload(context.Background(), store)
	assert.True(t, l.FeatureEnabled("dev"))
}

func TestLayered_GetBool(t *testing.T) {
	resetForTest()
	l := InitLayered(context.Background(), &ConfigInput{}, newStore(t))
	assert.True(t, l.GetBool(KeyFeatureExtensions, true))
	assert.False(t, l.GetBool(KeyFeatureExtensions, false))
	// 非法 key 返回默认值。
	assert.True(t, l.GetBool("site.name", true))
}

func TestLayered_ObsSnapshot(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{
		ObsAlertmanagerURL: "http://from-env:9093",
	}, store)

	snap := l.ObsSnapshot()
	assert.Equal(t, "http://from-env:9093", snap.AlertmanagerURL)
	assert.Equal(t, "config", snap.Sources[KeyObsAlertmanagerURL])
	assert.Empty(t, snap.GrafanaExploreURL)
	assert.Equal(t, "default", snap.Sources[KeyObsGrafanaExploreURL])

	require.NoError(t, store.Set(context.Background(), KeyObsGrafanaExploreURL, json.RawMessage(`"http://grafana:3000"`), "admin"))
	l.Reload(context.Background(), store)
	snap = l.ObsSnapshot()
	assert.Equal(t, "http://grafana:3000", snap.GrafanaExploreURL)
	assert.Equal(t, "database", snap.Sources[KeyObsGrafanaExploreURL])
	assert.Equal(t, "config", snap.Sources[KeyObsAlertmanagerURL])
}

func TestIsValidKey_WhitelistExpansion(t *testing.T) {
	for _, key := range []string{
		KeyFeatureDev, KeyFeatureSupport, KeyFeatureAnalytics, KeyFeatureOps, KeyFeatureExtensions,
		KeyObsAlertmanagerURL, KeyObsGrafanaExploreURL, KeyObsJaegerURL,
	} {
		assert.True(t, IsValidKey(key), key)
	}
	// bootstrap 类禁止入白名单。
	for _, key := range []string{"database.dataSource", "auth.jwtSecret", "server.port"} {
		assert.False(t, IsValidKey(key), key)
	}
	assert.True(t, IsBoolKey(KeyFeatureDev))
	assert.False(t, IsBoolKey(KeySiteName))
	assert.False(t, IsBoolKey(KeyObsJaegerURL))
}
