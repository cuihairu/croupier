package agent

// coverage_v9_test.go 补充 internal/app/agent 未覆盖分支：
// app.go（StartLocalServer 闭包/回调、Stop、超时与透传方法、扩展同步边界）、
// extension_runtime.go（decode 失败路径）、extension_driver_runtime.go（Sync 失败路径）、
// extension_sync_puller.go（ticker 与非法 URL）、control_client.go（mux 守卫与拨号失败）。

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	"github.com/cuihairu/croupier/internal/platform/tlsutil"
	"github.com/cuihairu/croupier/internal/telemetry"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// --- app.go: StartLocalServer ---

func TestAppStartLocalServer_OpenAPICallerPathsV9(t *testing.T) {
	app := NewWithConfigDir("127.0.0.1:1", "agent-v9-caller", t.TempDir())
	app.SetLocalAddr("127.0.0.1:0")
	require.NoError(t, app.StartLocalServer())
	defer app.Stop()
	require.NotNil(t, app.providerManager)

	// providerManager 已初始化：caller 透传到 ProviderManager.Call（未注册 provider 报错）。
	_, err := app.extensionDrivers.Invoke(context.Background(), "openapi-driver", "external.v9.echo", []byte("{}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider not found")

	// providerManager 被置空后：caller 返回 "provider manager not initialized"。
	app.providerManager = nil
	_, err = app.extensionDrivers.Invoke(context.Background(), "openapi-driver", "external.v9.echo", []byte("{}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider manager not initialized")
}

func TestAppStartLocalServer_OutboundTLSV9(t *testing.T) {
	app := NewWithConfigDir("127.0.0.1:1", "agent-v9-tls", t.TempDir())
	app.SetLocalAddr("127.0.0.1:0")
	app.WithOutboundTLSConfig(&tlsutil.ClientTLSConfig{ServerName: "upstream"})
	require.NoError(t, app.StartLocalServer())
	defer app.Stop()
	require.NotNil(t, app.localHandler)
}

// connectSDKProviderV9 用 MuxConn 模拟 SDK Provider 接入 App 的本地 TCP 网关。
func connectSDKProviderV9(t *testing.T, addr string) *tcptr.MuxConn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	provider := tcptr.NewMuxConn(conn, nil, transportcore.HandlerFunc(func(_ context.Context, msgID uint32, _ uint32, _ []byte) ([]byte, error) {
		return nil, nil
	}))
	go func() { _ = provider.Run(context.Background()) }()
	t.Cleanup(func() { _ = provider.Close() })
	return provider
}

func TestAppStartLocalServer_OnConnectDisconnectCallbacksV9(t *testing.T) {
	app := NewWithConfigDir("127.0.0.1:1", "agent-v9-cb", t.TempDir())
	app.SetLocalAddr("127.0.0.1:0")
	require.NoError(t, app.StartLocalServer())
	defer app.Stop()

	// 直接用 SDK Provider 接入触发 App 自身的 OnConnect/OnDisconnect 闭包：
	// 连接建立 → a.store.Register；断开 → a.store.RemoveProvider。
	provider := connectSDKProviderV9(t, app.GetLocalServerAddr())
	connectBody, err := proto.Marshal(&sdkv1.ProviderConnectRequest{
		ServiceId: "svc-v9",
		Version:   "1.0.0",
		Functions: []*sdkv1.ProviderFunctionDescriptor{{Id: "v9.fn", Version: "1.0.0"}},
	})
	require.NoError(t, err)
	_, _, err = provider.Call(context.Background(), protocol.MsgProviderConnectRequest, connectBody)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, ok := app.store.List()["v9.fn"]
		return ok
	}, 3*time.Second, 50*time.Millisecond)

	require.NoError(t, provider.Close())
	require.Eventually(t, func() bool {
		_, ok := app.store.List()["v9.fn"]
		return !ok
	}, 3*time.Second, 50*time.Millisecond)
}

// --- app.go: Stop / SetProviderCallTimeout / 透传配置 ---

func TestAppStop_WithTelemetryV9(t *testing.T) {
	svc, err := telemetry.NewGameTelemetryService(telemetry.TelemetryConfig{
		ServiceName: "agent-v9",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)

	app := New("127.0.0.1:1", "agent-v9-stop")
	app.WithTelemetry(svc)
	app.Stop() // telemetry 非 nil → Shutdown 路径
}

func TestAppSetProviderCallTimeoutV9(t *testing.T) {
	var nilApp *App
	nilApp.SetProviderCallTimeout(time.Second) // nil / 未初始化 guard

	app := NewWithConfigDir("127.0.0.1:1", "agent-v9-timeout", t.TempDir())
	app.SetLocalAddr("127.0.0.1:0")
	require.NoError(t, app.StartLocalServer())
	defer app.Stop()
	app.SetProviderCallTimeout(3 * time.Second) // 透传 LocalHandler
}

func TestAppWithUpstreamTLSAndTransportKindV9(t *testing.T) {
	app := New("127.0.0.1:1", "agent-v9-pass")
	app.WithUpstreamTLSConfig(&tlsutil.ClientTLSConfig{ServerName: "s"})
	app.SetUpstreamTransportKind("  TCP ")
	assert.Equal(t, "tcp", app.upstream.transportKind)
}

// --- app.go: 扩展同步边界 ---

func TestAppSyncExtensionsFromRuntime_NilGuardsV9(t *testing.T) {
	var nilApp *App
	assert.NoError(t, nilApp.syncExtensionsFromRuntime(context.Background()))

	app := New("127.0.0.1:1", "agent-v9-nostore")
	app.store = nil
	app.syncExtensionFunctionsFromRuntime() // store 为 nil 直接返回
}

func TestAppExtensionRuntimeDynamicLabels_WithInstallationsV9(t *testing.T) {
	app := New("127.0.0.1:1", "agent-v9-labels")
	_, err := app.extensionRuntime.ApplyPayload(&extensionsync.AgentSyncPayload{
		AgentID: "agent-v9-labels",
		Installations: []extensionsync.AgentInstallationPayload{
			{
				InstallationID: 1,
				ExtensionID:    "ext-v9",
				Enabled:        true,
				Bindings: []extensionsync.AgentBindingPayload{
					{BindingType: "function", BindingKey: "v9.f1"},
					{BindingType: "function", BindingKey: "v9.f2"},
				},
			},
			{
				InstallationID: 2,
				ExtensionID:    "ext-v9-off",
				Enabled:        false,
			},
		},
	})
	require.NoError(t, err)

	labels := app.extensionRuntimeDynamicLabels()
	require.NotNil(t, labels)
	assert.Equal(t, "2", labels["extensions.runtime.installations"])
	assert.Equal(t, "1", labels["extensions.runtime.enabled"])
	assert.Equal(t, "2", labels["extensions.runtime.bindings"])
}

func TestAppInvokeExtensionFunction_ProviderManagerCallV9(t *testing.T) {
	app := NewWithConfigDir("127.0.0.1:1", "agent-v9-invoke", t.TempDir())
	app.SetLocalAddr("127.0.0.1:0")
	require.NoError(t, app.StartLocalServer())
	defer app.Stop()

	app.extensionMu.Lock()
	app.extensionRoutes["external.v9.missing"] = extensionFunctionRoute{Driver: "workflow-driver"}
	app.extensionMu.Unlock()

	// providerManager 已初始化，Call 因 provider 未注册而失败。
	_, err := app.invokeExtensionFunction(context.Background(), "external.v9.missing", []byte("{}"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider not found")
}

// --- app.go: discoverExtensionFunctions / resolveFunctionDriver 边界 ---

func TestDiscoverExtensionFunctions_EdgesV9(t *testing.T) {
	item := RuntimeInstallation{
		ExtensionID:    "ext-v9",
		ReleaseVersion: "1.0.0",
		Bindings: []RuntimeBinding{
			// key 不匹配目标函数的 binding（resolveFunctionDriver 中被跳过）。
			{BindingType: "function", BindingKey: "other.fn"},
			// 带显式 driver 的目标 binding。
			{BindingType: "function", BindingKey: "v9.fn", Spec: map[string]any{"driver": "custom-v9"}},
			// 空 id：push 守卫直接返回。
			{BindingType: "function", BindingKey: "   "},
			// capability 无 operations：单函数降级。
			{BindingType: "capability", BindingKey: "cap-v9", Spec: map[string]any{}},
		},
	}
	funcs := discoverExtensionFunctions(item)
	ids := map[string]bool{}
	for _, fn := range funcs {
		ids[fn.GetId()] = true
	}
	assert.True(t, ids["cap-v9"], "capability without operations should degrade to single function")
	assert.True(t, ids["v9.fn"])
	assert.False(t, ids[""])

	// 遍历顺序中先遇到 key 不匹配的 binding（continue 分支），最终命中目标。
	assert.Equal(t, "custom-v9", resolveFunctionDriver(item, "v9.fn"))
}

// --- extension_runtime.go ---

func TestExtensionRuntimeApplyPayload_NilReceiverV9(t *testing.T) {
	var rt *ExtensionRuntime
	_, err := rt.ApplyPayload(nil)
	require.Error(t, err)
}

func TestExtensionRuntimeDecodeFailuresV9(t *testing.T) {
	rt := NewExtensionRuntime()

	res, err := rt.ApplyPayload(&extensionsync.AgentSyncPayload{
		Installations: []extensionsync.AgentInstallationPayload{
			{InstallationID: 1, ExtensionID: "bad-config", ConfigJSON: "{not-json"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Failed)

	res, err = rt.ApplyPayload(&extensionsync.AgentSyncPayload{
		Installations: []extensionsync.AgentInstallationPayload{
			{InstallationID: 2, ExtensionID: "bad-secrets", SecretRefsJSON: "[not-map"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Failed)

	res, err = rt.ApplyPayload(&extensionsync.AgentSyncPayload{
		Installations: []extensionsync.AgentInstallationPayload{
			{
				InstallationID: 3,
				ExtensionID:    "bad-binding",
				Bindings:       []extensionsync.AgentBindingPayload{{BindingType: "function", SpecJSON: "{oops"}},
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Failed)

	// 有失败项时快照进入 degraded 状态。
	snap := rt.Snapshot()
	assert.Equal(t, "degraded", snap.LastApplyStatus)
	assert.NotEmpty(t, snap.LastError)
	assert.NotZero(t, snap.LastErrorAt)
}

// --- extension_driver_runtime.go ---

// flakyDriverV9 可注入 Init/Reload/Stop 失败。
type flakyDriverV9 struct {
	name        string
	failInit    bool
	failReload  bool
	failStop    bool
	initCount   int
	reloadCount int
	stopCount   int
}

func (d *flakyDriverV9) Name() string { return d.name }

func (d *flakyDriverV9) Init(ctx context.Context, installation RuntimeInstallation) error {
	d.initCount++
	if d.failInit {
		return errFakeV9("init failed")
	}
	return nil
}

func (d *flakyDriverV9) Reload(ctx context.Context, installation RuntimeInstallation) error {
	d.reloadCount++
	if d.failReload {
		return errFakeV9("reload failed")
	}
	return nil
}

func (d *flakyDriverV9) Stop(ctx context.Context, installationID uint) error {
	d.stopCount++
	if d.failStop {
		return errFakeV9("stop failed")
	}
	return nil
}

func (d *flakyDriverV9) Invoke(ctx context.Context, functionID string, payload []byte) ([]byte, error) {
	return payload, nil
}

type errFakeV9 string

func (e errFakeV9) Error() string { return string(e) }

func TestExtensionDriverRuntimeSync_FailurePathsV9(t *testing.T) {
	rt := NewExtensionDriverRuntime()
	bad := &flakyDriverV9{name: "flaky-v9"}
	initFail := &flakyDriverV9{name: "init-fail-v9", failInit: true}
	rt.RegisterDriver(bad)
	rt.RegisterDriver(initFail)

	// 覆盖 Init 失败：安装 2 的驱动 Init 直接报错。
	initSnap := ExtensionRuntimeSnapshot{Installations: []RuntimeInstallation{
		{InstallationID: 2, Bindings: []RuntimeBinding{
			{BindingType: "driver", Spec: map[string]any{"driver": "init-fail-v9"}},
		}},
	}}
	res, err := rt.Sync(context.Background(), initSnap)
	require.Error(t, err)
	assert.Equal(t, 1, res.Failed)
	assert.Equal(t, 0, res.Initialized)

	snap := ExtensionRuntimeSnapshot{Installations: []RuntimeInstallation{
		{InstallationID: 1, Bindings: []RuntimeBinding{
			{BindingType: "driver", Spec: map[string]any{"driver": "flaky-v9"}},
		}},
	}}

	// 初始化成功，建立 assignment。
	_, err = rt.Sync(context.Background(), snap)
	require.NoError(t, err)

	// 已初始化的驱动走 Reload，注入失败（失败后驱动会被卸载）。
	bad.failReload = true
	_, err = rt.Sync(context.Background(), snap)
	require.Error(t, err)

	// 重新初始化，随后清空快照触发 Stop，注入失败。
	bad.failReload = false
	_, err = rt.Sync(context.Background(), snap)
	require.NoError(t, err)

	bad.failStop = true
	res, err = rt.Sync(context.Background(), ExtensionRuntimeSnapshot{})
	require.Error(t, err)
	assert.Equal(t, 1, res.Failed)
	assert.Equal(t, 0, res.Stopped)
}

func TestResolveDriverNames_EdgesV9(t *testing.T) {
	// 空 TargetRef（"driver:" 前缀后为空）被 push 守卫忽略。
	names := resolveDriverNames(RuntimeInstallation{Bindings: []RuntimeBinding{
		{BindingType: "driver", TargetRef: "driver:"},
	}})
	assert.Equal(t, []string{"workflow-driver"}, names)

	// 无任何 binding 时回退 workflow-driver。
	assert.Equal(t, []string{"workflow-driver"}, resolveDriverNames(RuntimeInstallation{}))
}

// --- extension_sync_puller.go ---

func TestExtensionSyncPuller_StartTickerFiresV9(t *testing.T) {
	pulls := 0
	done := make(chan struct{})
	srv := newSyncAPIV9(t, func() {
		pulls++
		if pulls >= 3 {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	})
	rt := NewExtensionRuntime()
	p := NewExtensionSyncPuller(srv.URL, "agent-v9-tick", 50*time.Millisecond, rt)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ticker-driven pulls did not fire")
	}
	cancel()
}

func TestExtensionSyncPuller_PullOnce_InvalidURLV9(t *testing.T) {
	p := NewExtensionSyncPuller("://no-scheme-v9", "agent-v9-url", time.Second, NewExtensionRuntime())
	err := p.PullOnce(context.Background())
	require.Error(t, err)
}

// --- control_client.go ---

func TestNewMuxControlClient_DialFailureV9(t *testing.T) {
	_, err := newMuxControlClient("127.0.0.1:1", nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dial upstream")
}

func TestMuxControlClientClose_NilMuxV9(t *testing.T) {
	cancelled := false
	c := &muxControlClient{cancel: func() { cancelled = true }}
	require.NoError(t, c.Close())
	assert.True(t, cancelled)
}

func TestMuxControlClientCall_NilGuardV9(t *testing.T) {
	var c *muxControlClient
	err := c.call(context.Background(), 1, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

func TestMuxControlClientCall_AfterCloseV9(t *testing.T) {
	f := newFakeMuxControlServer(t)
	mc, err := newMuxControlClient(f.addr(), nil, nil)
	require.NoError(t, err)
	require.NoError(t, mc.Close())

	// 连接关闭后 Call 失败。
	_, err = mc.Heartbeat(context.Background(), &agentv1.HeartbeatRequest{AgentId: "a"})
	require.Error(t, err)
}

// --- extension_external_bridge.go ---

func TestInvokeExternalPlatformFunction_NilCallerV9(t *testing.T) {
	out, handled, err := invokeExternalPlatformFunction(context.Background(), "external.v9.fn", []byte("{}"), nil)
	require.True(t, handled)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "caller is not configured")
	assert.Nil(t, out)
}

// --- 杂项：LocalStore 语义（配合上面的 provider 会话用例） ---

func TestLocalStoreRegisterEmptyFuncsNoopV9(t *testing.T) {
	store := agentlocal.NewLocalStore()
	store.Register("p1", "s1", "", "1.0", nil, nil)
	assert.Empty(t, store.List())
}
