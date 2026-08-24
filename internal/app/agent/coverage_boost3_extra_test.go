package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustUnmarshalJSON(t *testing.T, raw string, v any) {
	t.Helper()
	require.NoError(t, json.Unmarshal([]byte(raw), v))
}

const badDriverPayloadJSON = `{
	"agentId": "agent-bad-driver",
	"installations": [
		{
			"installationId": 21,
			"extensionId": "ext-bad-driver",
			"enabled": true,
			"bindings": [
				{"bindingType": "function", "bindingKey": "bad.fn", "specJson": "{\"driver\":\"no-such-driver\"}"}
			]
		}
	]
}`

func TestAppApplyExtensionSyncPayload_DirectBadDriver(t *testing.T) {
	app := New("127.0.0.1:1", "agent-bad-driver")
	var payload extensionsync.AgentSyncPayload
	mustUnmarshalJSON(t, badDriverPayloadJSON, &payload)

	_, err := app.ApplyExtensionSyncPayload(&payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "driver not found")

	// With a last payload stored, Reconcile reloads fine but syncing the
	// (still unknown) driver fails again.
	_, err = app.ReconcileExtensions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "driver not found")
}

func TestAppSyncExtensions_WithProviderManagerInitialized(t *testing.T) {
	app := NewWithConfigDir("127.0.0.1:1", "agent-pm-sync", t.TempDir())
	app.SetLocalAddr("127.0.0.1:0")
	require.NoError(t, app.StartLocalServer())
	defer app.Stop()
	require.NotNil(t, app.providerManager)

	var payload extensionsync.AgentSyncPayload
	mustUnmarshalJSON(t, badDriverPayloadJSON, &payload)
	_, err := app.extensionRuntime.ApplyPayload(&payload)
	require.NoError(t, err)

	err = app.syncExtensionsFromRuntime(context.Background())
	require.Error(t, err)
}

func TestAppSyncExtensionFunctions_EmptyFunctionsRemoveProvider(t *testing.T) {
	app := New("127.0.0.1:1", "agent-empty-funcs")
	var payload extensionsync.AgentSyncPayload
	mustUnmarshalJSON(t, `{
		"agentId": "agent-empty-funcs",
		"installations": [{"installationId": 31, "extensionId": "ext-empty", "enabled": true, "bindings": []}]
	}`, &payload)
	_, err := app.extensionRuntime.ApplyPayload(&payload)
	require.NoError(t, err)

	// No bindings => no discovered functions => provider entry removed.
	app.store.Register("extension:31", "", "", "1.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "leftover.fn", Version: "1.0"},
	}, nil)
	app.syncExtensionFunctionsFromRuntime()
	assert.False(t, app.hasExtensionFunction("leftover.fn"))
}

func TestAppPullExtensionSyncOnce_SyncFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","payload":` + badDriverPayloadJSON + `}`))
	}))
	defer srv.Close()

	t.Setenv("CROUPIER_EXTENSION_SYNC_API", srv.URL)
	app := New("127.0.0.1:1", "agent-bad-driver")
	app.startExtensionSync(context.Background())
	require.NotNil(t, app.extensionPuller)

	err := app.PullExtensionSyncOnce(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "driver not found")
}

func TestAppExtensionRuntimeDynamicLabels_AfterError(t *testing.T) {
	app := New("127.0.0.1:1", "agent-labels")
	app.extensionRuntime.RecordError(errors.New("sync exploded"))

	labels := app.extensionRuntimeDynamicLabels()
	assert.Equal(t, "error", labels["extensions.runtime.status"])
	assert.Contains(t, labels["extensions.runtime.last_error"], "sync exploded")
	assert.Contains(t, labels, "extensions.runtime.driver_init")

	var nilApp *App
	assert.Nil(t, nilApp.extensionRuntimeDynamicLabels())
}

func TestBuildExtensionProviderEntries_Variants(t *testing.T) {
	snapshot := ExtensionRuntimeSnapshot{
		Installations: []RuntimeInstallation{
			{
				// Not an external-platform extension: skipped entirely.
				ExtensionID: "plain-ext",
				Bindings:    []RuntimeBinding{{BindingType: "provider", BindingKey: "p"}},
			},
			{
				ExtensionID: "demo.external-platform",
				Bindings: []RuntimeBinding{
					// Non-provider binding types are skipped.
					{BindingType: "function", BindingKey: "f"},
					// Empty provider name cannot be parsed.
					{BindingType: "provider"},
					// Existing "methods" in config wins over derived ones.
					{BindingType: "openapi", BindingKey: "shop", Spec: map[string]any{
						"methods":    []any{map[string]any{"name": "listItems"}},
						"operations": []any{"listItems", "getItem"},
					}},
					// No operations: entry still created without methods.
					{BindingType: "provider", BindingKey: "bare"},
				},
			},
		},
	}

	entries := buildExtensionProviderEntries(snapshot)
	require.Contains(t, entries, "shop")
	require.Contains(t, entries, "bare")

	shop := entries["shop"]
	assert.True(t, shop.Enabled)
	require.Contains(t, shop.Config, "methods")
	rawMethods, ok := shop.Config["methods"].([]any)
	require.True(t, ok)
	assert.Len(t, rawMethods, 1)

	_, hasMethods := entries["bare"].Config["methods"]
	assert.True(t, hasMethods) // default "invoke" operation derives a method
}

// ---------------------------------------------------------------------------
// provider.go
// ---------------------------------------------------------------------------

func TestProviderManagerSyncExtensionProviders_KeepReplaceAndConflict(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte(`
providers:
  static1:
    enabled: true
    type: openapi
`), 0o600))

	store := agentlocal.NewLocalStore()
	m := NewProviderManager(store, dir, nil)
	require.NoError(t, m.Load(context.Background()))

	ctx := context.Background()
	entry := ProviderEntry{Enabled: true, Type: "openapi", Config: map[string]interface{}{}}

	// Two managed providers.
	require.NoError(t, m.SyncExtensionProviders(ctx, map[string]ProviderEntry{"a": entry, "b": entry}))
	// Keeping only "a" removes "b" and re-initializes "a".
	require.NoError(t, m.SyncExtensionProviders(ctx, map[string]ProviderEntry{"a": entry}))
	assert.True(t, m.IsPlatformFunction("a.x"))
	assert.False(t, m.IsPlatformFunction("b.x"))

	// Conflicting with a static provider without override: skipped.
	require.NoError(t, m.SyncExtensionProviders(ctx, map[string]ProviderEntry{"static1": entry}))
	require.NoError(t, m.Close())
}

func TestProviderManagerLoad_InitFailureContinues(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte(`
providers:
  broken:
    enabled: true
    type: totally-unknown
  off:
    enabled: false
    type: openapi
  good:
    enabled: true
    type: openapi
`), 0o600))

	m := NewProviderManager(agentlocal.NewLocalStore(), dir, nil)
	require.NoError(t, m.Load(context.Background()))
	assert.True(t, m.IsPlatformFunction("good.anything"))
	assert.False(t, m.IsPlatformFunction("broken.anything"))
	require.NoError(t, m.Close())
}

func TestProviderManagerStartHeartbeat_ZeroInterval(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte(`
providers:
  hb0:
    enabled: true
    type: openapi
`), 0o600))

	t.Setenv("CROUPIER_AGENTLOCAL_PROVIDER_HEARTBEAT", "0s")
	m := NewProviderManager(agentlocal.NewLocalStore(), dir, nil)
	require.NoError(t, m.Load(context.Background()))
	require.NoError(t, m.Close())
}

func TestProviderManagerWrapper_WithRealManager(t *testing.T) {
	store := agentlocal.NewLocalStore()
	m := NewProviderManager(store, t.TempDir(), nil)
	require.NoError(t, m.SyncExtensionProviders(context.Background(), map[string]ProviderEntry{
		"real": {Enabled: true, Type: "openapi", Config: map[string]interface{}{}},
	}))

	w := &providerManagerWrapper{pm: m}
	assert.True(t, w.IsPlatformFunction("real.method"))

	_, err := w.Call(context.Background(), "real.method", []byte("{}"))
	// The OpenAPI provider without a spec cannot serve the call, but the
	// wrapper must delegate into it.
	_ = err
	require.NoError(t, m.Close())
}

// ---------------------------------------------------------------------------
// upstream.go leftovers
// ---------------------------------------------------------------------------

func TestUpstreamSyncWithRetry_CancelDuringBackoff(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "a", agentlocal.NewLocalStore(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	err := client.syncWithRetry(ctx, 3)
	require.ErrorIs(t, err, context.Canceled)
}

func TestUpstreamSendMetricEvent_NilReportWhileConnected(t *testing.T) {
	f := newFlakyControlServer(t)
	client := NewUpstreamClient(f.addr(), "agent-nr", agentlocal.NewLocalStore(), nil)
	ctx := context.Background()
	require.NoError(t, client.dialServer(ctx))
	defer client.Stop()

	require.Error(t, client.SendMetricEvent(ctx, nil))
}

func TestUpstreamMetricsLoop_DefaultInterval(t *testing.T) {
	f := newFlakyControlServer(t)
	client := NewUpstreamClient(f.addr(), "agent-mi0", agentlocal.NewLocalStore(), nil)
	client.WithMetricsReporting(0) // falls back to 30s inside metricsLoop

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))
	defer client.Stop()

	// The immediate first report must still arrive.
	require.Eventually(t, func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.metricEvents >= 1
	}, 3*time.Second, 50*time.Millisecond)
}

func TestBuildProviders_AddrVersionBackfill(t *testing.T) {
	now := time.Now()
	localData := map[string][]agentlocal.Instance{
		"fn.backfill": {
			{ProviderID: "svc-c", LastSeen: now},                                  // no addr/version
			{ProviderID: "svc-c", Addr: "10.9.9.9:1", Version: "", LastSeen: now}, // addr backfills
		},
	}
	versions := map[string]map[string]string{"fn.backfill": {"i": "7.7"}}

	procs := buildProviders(localData, versions)
	require.Len(t, procs, 1)
	assert.Equal(t, "10.9.9.9:1", procs[0].Addr)
	assert.Equal(t, "7.7", procs[0].Version)
}
