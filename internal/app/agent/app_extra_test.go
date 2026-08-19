package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- App 基础方法 ---

func TestApp_NilGuards(t *testing.T) {
	var a *App
	a.WithTelemetry(nil)
	a.SetLocalAddr("x")
	assert.Equal(t, "", a.GetLocalServerAddr())
	a.WithUpstreamMetadata(UpstreamMetadata{})
	a.WithUpstreamTLSConfig(nil)
	a.SetUpstreamTransportKind("tcp")
	a.OnConnected(func() {})
	a.OnDisconnected(func(error) {})
	a.SyncUpstream(context.Background())
	require.Error(t, a.SyncUpstream(context.Background()))
	require.Error(t, a.HeartbeatUpstream(context.Background()))
	a.WithVersion("v")
	assert.Nil(t, a.OpsServer())
	assert.Nil(t, a.MetricsCollector())
	assert.Nil(t, a.Tasks())
	assert.Zero(t, a.ActiveTaskCount())
	assert.Nil(t, a.ExtensionRuntime())
	assert.Nil(t, a.Store())
	a.WithOpsConfig(nil)
	_, err := a.ApplyExtensionSyncPayload(nil)
	require.Error(t, err)
	_, err = a.ApplyExtensionSyncPayloadJSON(nil)
	require.Error(t, err)
	_, err = a.ReconcileExtensions()
	require.Error(t, err)
	err = a.PullExtensionSyncOnce(context.Background())
	require.Error(t, err)
	assert.Nil(t, a.extensionRuntimeDynamicLabels())
	assert.False(t, a.hasExtensionFunction("f"))
	_, err = a.invokeExtensionFunction(context.Background(), "f", nil)
	require.Error(t, err)
}

func TestApp_Accessors(t *testing.T) {
	app := New("127.0.0.1:1", "agent-acc")

	require.Error(t, app.SyncUpstream(context.Background())) // 未连接
	require.Error(t, app.HeartbeatUpstream(context.Background()))

	app.WithVersion("9.9.9")
	app.WithOpsConfig(&OpsConfig{Enabled: true, MetricsEnabled: true, MetricsInterval: time.Second})
	app.WithOpsConfig(nil) // 覆盖 nil 分支
	app.WithOutboundTLSConfig(&tlsutil.ClientTLSConfig{ServerName: "s"})
	require.NotNil(t, app.Store())
	require.NotNil(t, app.Tasks())
	require.NotNil(t, app.ExtensionRuntime())
	require.NotNil(t, app.MetricsCollector())
	assert.Zero(t, app.ActiveTaskCount())

	app.Tasks().Set("task-1", "127.0.0.1:1")
	assert.Equal(t, 1, app.ActiveTaskCount())
	if _, ok := app.Tasks().Get("task-1"); !ok {
		t.Fatal("task not found after Set")
	}
	app.Tasks().Delete("task-1")
	assert.Zero(t, app.ActiveTaskCount())

	app.SetLocalAddr("127.0.0.1:0")
	assert.Equal(t, "", app.GetLocalServerAddr()) // 尚未启动 server

	app.WithTelemetry(nil)
	require.NotNil(t, app)
}

func TestApp_WithUpstreamMetadataAndCallbacks(t *testing.T) {
	app := New("127.0.0.1:1", "agent-meta")
	connected := false
	disconnected := false
	app.OnConnected(func() { connected = true })
	app.OnDisconnected(func(error) { disconnected = true })
	app.WithUpstreamMetadata(UpstreamMetadata{GameID: "g", Env: "e"})
	assert.Equal(t, "g", app.upstream.GameID())
	assert.Equal(t, "e", app.upstream.Env())
	assert.False(t, connected)
	assert.False(t, disconnected)
}

func TestApp_ApplyExtensionSyncPayload(t *testing.T) {
	app := New("127.0.0.1:1", "agent-ext")

	payload := &extensionsync.AgentSyncPayload{
		AgentID: "agent-ext",
		Installations: []extensionsync.AgentInstallationPayload{
			{
				InstallationID:  1,
				InstallationKey: "key-1",
				ExtensionID:     "ext-1",
				ReleaseVersion:  "1.0.0",
				Enabled:         true,
				Bindings: []extensionsync.AgentBindingPayload{
					{BindingType: "function", BindingKey: "my.func"},
				},
			},
		},
	}
	result, err := app.ApplyExtensionSyncPayload(payload)
	require.NoError(t, err)
	require.NotNil(t, result)

	// 函数被发现并注册路由
	assert.True(t, app.hasExtensionFunction("my.func"))
	assert.False(t, app.hasExtensionFunction(""))
	assert.False(t, app.hasExtensionFunction("missing.func"))

	// JSON 入口
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	_, err = app.ApplyExtensionSyncPayloadJSON(raw)
	require.NoError(t, err)

	// 无效 JSON
	_, err = app.ApplyExtensionSyncPayloadJSON([]byte("{bad"))
	require.Error(t, err)

	// ReconcileExtensions
	res, err := app.ReconcileExtensions()
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestApp_PullExtensionSyncOnceNotInitialized(t *testing.T) {
	app := New("127.0.0.1:1", "agent-pull")
	err := app.PullExtensionSyncOnce(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestApp_ExtensionRuntimeDynamicLabels(t *testing.T) {
	app := New("127.0.0.1:1", "agent-labels")
	labels := app.extensionRuntimeDynamicLabels()
	require.NotNil(t, labels)
	assert.Equal(t, "0", labels["extensions.runtime.installations"])
	assert.Contains(t, labels, "extensions.runtime.driver_init")

	app.extensionRuntime.RecordError(errors.New("boom"))
	labels = app.extensionRuntimeDynamicLabels()
	assert.Contains(t, labels["extensions.runtime.last_error"], "boom")
}

func TestApp_InvokeExtensionFunctionErrors(t *testing.T) {
	app := New("127.0.0.1:1", "agent-inv")

	_, err := app.invokeExtensionFunction(context.Background(), "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "function id is required")

	_, err = app.invokeExtensionFunction(context.Background(), "no.such", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestApp_RunSmoke(t *testing.T) {
	// Run 连接一个假控制面：本地 server 启动、上游注册成功、Run 返回 nil。
	f := newFakeMuxControlServer(t)
	dir := t.TempDir()
	app := NewWithConfigDir(f.addr(), "agent-run", dir)
	app.SetLocalAddr("127.0.0.1:0")
	app.WithUpstreamMetadata(UpstreamMetadata{GameID: "g", Env: "e", HeartbeatInterval: 1 * time.Hour})

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, app.Run(ctx))
	defer func() {
		cancel()
		// 等注册完成后上游保持连接态（无并发写 c.client），可安全 Stop。
		// 注意：UpstreamClient.client 字段无锁读写是产品级数据竞争，单独上报。
		time.Sleep(100 * time.Millisecond)
		app.Stop()
	}()
	require.Eventually(t, func() bool { return f.registerCount() >= 1 }, 3*time.Second, 50*time.Millisecond)

	addr := app.GetLocalServerAddr()
	require.NotEmpty(t, addr)
	assert.True(t, strings.HasPrefix(addr, "127.0.0.1:"))

	require.NotNil(t, app.OpsServer())
	require.NotNil(t, app.providerManager)
}

func TestApp_StartExtensionSync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/agents/agent-sync/extensions", r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    0,
			"message": "ok",
			"payload": map[string]any{
				"agentId": "agent-sync",
				"installations": []map[string]any{
					{
						"installationId": 7,
						"extensionId":    "ext-1",
						"enabled":        true,
						"bindings": []map[string]any{
							{"bindingType": "function", "bindingKey": "sync.fn"},
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	t.Setenv("CROUPIER_EXTENSION_SYNC_API", srv.URL)
	t.Setenv("CROUPIER_EXTENSION_SYNC_INTERVAL", "200ms")
	t.Setenv("CROUPIER_AGENTLOCAL_PRUNE_INTERVAL", "100ms")
	t.Setenv("CROUPIER_AGENTLOCAL_MAX_AGE", "1ms")
	t.Setenv("CROUPIER_AGENTLOCAL_TASKRESULT_MAX_AGE", "1ms")

	app := New("127.0.0.1:1", "agent-sync")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.startMaintenance(ctx)
	app.startExtensionSync(ctx)
	require.NotNil(t, app.extensionPuller)

	// Start 的后台循环会把 payload 应用到 runtime
	require.Eventually(t, func() bool {
		return len(app.extensionRuntime.Snapshot().Installations) > 0
	}, 3*time.Second, 50*time.Millisecond)

	// PullExtensionSyncOnce 拉取并把 runtime 状态同步为函数路由
	require.NoError(t, app.PullExtensionSyncOnce(ctx))
	assert.True(t, app.hasExtensionFunction("sync.fn"))

	// 非法 interval 覆盖分支
	t.Setenv("CROUPIER_AGENTLOCAL_PRUNE_INTERVAL", "not-a-duration")
	app2 := New("127.0.0.1:1", "agent-sync2")
	app2.startMaintenance(ctx) // 使用默认值，不应 panic
}

func TestApp_StartMaintenanceDisabled(t *testing.T) {
	t.Setenv("CROUPIER_AGENTLOCAL_PRUNE_INTERVAL", "-1s")
	app := New("127.0.0.1:1", "agent-m")
	app.startMaintenance(context.Background()) // pruneInterval<=0 直接返回

	var nilApp *App
	nilApp.startMaintenance(context.Background())
	nilApp.startExtensionSync(context.Background())
}

func TestParseDurationEnvExtra(t *testing.T) {
	t.Setenv("T_DUR2", "1h")
	assert.Equal(t, time.Hour, parseDurationEnv("T_DUR2", 5*time.Second))

	t.Setenv("T_DUR2", "bogus")
	assert.Equal(t, 5*time.Second, parseDurationEnv("T_DUR2", 5*time.Second))
}

// --- ExtensionSyncPuller ---

func TestExtensionSyncPuller_PullOnceVariants(t *testing.T) {
	runtime := NewExtensionRuntime()

	t.Run("nil puller", func(t *testing.T) {
		var p *ExtensionSyncPuller
		require.Error(t, p.PullOnce(context.Background()))
	})

	t.Run("http error status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()
		p := NewExtensionSyncPuller(srv.URL, "agent-1", time.Second, runtime)
		err := p.PullOnce(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status=500")
	})

	t.Run("bad json", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("not json"))
		}))
		defer srv.Close()
		p := NewExtensionSyncPuller(srv.URL, "agent-1", time.Second, runtime)
		require.Error(t, p.PullOnce(context.Background()))
	})

	t.Run("null payload", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "payload": nil})
		}))
		defer srv.Close()
		p := NewExtensionSyncPuller(srv.URL, "agent-1", time.Second, runtime)
		require.NoError(t, p.PullOnce(context.Background()))
	})

	t.Run("invalid payload shape", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "payload": map[string]any{"installations": "not-a-list"}})
		}))
		defer srv.Close()
		p := NewExtensionSyncPuller(srv.URL, "agent-1", time.Second, runtime)
		require.Error(t, p.PullOnce(context.Background()))
	})

	t.Run("connection error", func(t *testing.T) {
		p := NewExtensionSyncPuller("http://127.0.0.1:1", "agent-1", time.Second, runtime)
		require.Error(t, p.PullOnce(context.Background()))
	})
}

func TestExtensionSyncPuller_ConstructAndStartGuards(t *testing.T) {
	p := NewExtensionSyncPuller(" http://x/ ", "  a1  ", 0, NewExtensionRuntime())
	assert.Equal(t, "http://x", p.baseURL)
	assert.Equal(t, "a1", p.agentID)
	assert.Equal(t, 30*time.Second, p.interval)

	var nilPuller *ExtensionSyncPuller
	nilPuller.Start(context.Background()) // 不应 panic

	p2 := NewExtensionSyncPuller("", "", time.Second, nil)
	p2.Start(context.Background()) // baseURL 为空直接返回
}

func TestExtensionDriverRuntime_Extra(t *testing.T) {
	rt := NewExtensionDriverRuntime()

	// nil 守卫
	var nilRt *ExtensionDriverRuntime
	_, err := nilRt.Invoke(context.Background(), "d", "f", nil)
	require.Error(t, err)
	assert.Equal(t, ExtensionDriverSyncResult{}, nilRt.LastResult())
	nilRt.SetOpenAPICaller(nil)
	nilRt.RegisterDriver(nil)

	// 名称错误
	_, err = rt.Invoke(context.Background(), " ", "f", nil)
	require.Error(t, err)
	_, err = rt.Invoke(context.Background(), "no-such", "f", nil)
	require.Error(t, err)

	// noop 驱动生命周期（注册为运行时驱动供后续 Sync/Invoke 使用）
	noop := NewNoopExtensionDriver("custom")
	rt.RegisterDriver(noop)
	require.NoError(t, noop.Init(context.Background(), RuntimeInstallation{}))
	require.NoError(t, noop.Reload(context.Background(), RuntimeInstallation{}))
	require.NoError(t, noop.Stop(context.Background(), 1))
	out, err := noop.Invoke(context.Background(), "f", []byte("payload"))
	require.NoError(t, err)
	assert.Equal(t, []byte("payload"), out)
	require.NotNil(t, noop)

	// openapi 驱动
	od := NewOpenAPIExtensionDriver()
	require.NoError(t, od.Init(context.Background(), RuntimeInstallation{}))
	require.NoError(t, od.Reload(context.Background(), RuntimeInstallation{}))
	require.NoError(t, od.Stop(context.Background(), 1))
	od.SetCaller(func(ctx context.Context, provider, method string, request []byte) ([]byte, error) {
		return []byte(`{"ok":true}`), nil
	})
	out, err = od.Invoke(context.Background(), "external.demo.ping", []byte("{}"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(out))

	_, err = od.Invoke(context.Background(), "not-external", []byte("{}"))
	require.Error(t, err)

	var nilOpenapi *openapiExtensionDriver
	_, err = nilOpenapi.Invoke(context.Background(), "external.a.b", nil)
	require.Error(t, err)
	nilOpenapi.SetCaller(nil)

	// LastResult 记录
	snap := ExtensionRuntimeSnapshot{Installations: []RuntimeInstallation{
		{InstallationID: 1, Bindings: []RuntimeBinding{
			{BindingType: "driver", Spec: map[string]any{"driver": "custom"}},
		}},
	}}
	_, err = rt.Sync(context.Background(), snap)
	require.NoError(t, err)
	stats := rt.LastResult()
	assert.Equal(t, 1, stats.Initialized)

	out, err = rt.Invoke(context.Background(), "custom", "f", []byte("p"))
	require.NoError(t, err)
	assert.Equal(t, []byte("p"), out)
}

// --- UpstreamClient 辅助 ---

func TestUpstreamClient_SetTransportKindInvalid(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "a", agentlocal.NewLocalStore(), nil)
	client.SetTransportKind("tcp")
	client.SetTransportKind("")
	assert.Panics(t, func() { client.SetTransportKind("grpc") })
}

func TestUpstreamClient_SetTLSConfig(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "a", agentlocal.NewLocalStore(), nil)
	client.SetTLSConfig(&tlsutil.ClientTLSConfig{ServerName: "s"})
	client.SetTLSConfig(nil)
}

func TestUpstreamClient_SendTaskEventNilClient(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "a", agentlocal.NewLocalStore(), nil)
	require.Error(t, client.SendTaskEvent(context.Background(), nil))
	require.Error(t, client.ReportTaskEvent(context.Background(), nil))
	require.Error(t, client.SendMetricEvent(context.Background(), nil))
	var nilClient *UpstreamClient
	require.Error(t, nilClient.SendTaskEvent(context.Background(), nil))
	require.Error(t, nilClient.SendMetricEvent(context.Background(), nil))
}

func TestUpstreamClient_WithMetricsReporting(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "a", agentlocal.NewLocalStore(), nil)
	client.WithMetricsReporting(time.Second)
	var nilClient *UpstreamClient
	nilClient.WithMetricsReporting(time.Second)

	client.metricsEnabled = true
	require.Error(t, client.SendMetricEvent(context.Background(), nil)) // 未连接
}

func TestApp_WithTelemetryService(t *testing.T) {
	app := New("127.0.0.1:1", "a")
	app.WithTelemetry(&telemetry.GameTelemetryService{})
	require.NotNil(t, app)
}
