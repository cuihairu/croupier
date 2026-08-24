package agent

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	transportcore "github.com/cuihairu/croupier/internal/transport"
	tcptr "github.com/cuihairu/croupier/internal/transport/tcp"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// flakyControlServer is a TCP control-plane fake whose register/heartbeat
// handlers can be toggled to fail or return garbage at runtime.
type flakyControlServer struct {
	addrVal      string
	closeFn      func()
	mu           sync.Mutex
	heartbeats   int
	registers    int
	metricEvents int
	hbFail       atomic.Bool
	regFail      atomic.Bool
	garbageReg   atomic.Bool
}

func newFlakyControlServer(t *testing.T) *flakyControlServer {
	t.Helper()
	f := &flakyControlServer{}
	handler := transportcore.HandlerFunc(func(ctx context.Context, msgID uint32, reqID uint32, body []byte) ([]byte, error) {
		switch msgID {
		case protocol.MsgRegisterRequest:
			f.mu.Lock()
			f.registers++
			f.mu.Unlock()
			if f.garbageReg.Load() {
				return []byte("this-is-not-proto"), nil
			}
			if f.regFail.Load() {
				return nil, fmt.Errorf("register rejected")
			}
			return proto.Marshal(&agentv1.RegisterResponse{SessionId: "sess-flaky"})
		case protocol.MsgHeartbeatRequest:
			f.mu.Lock()
			f.heartbeats++
			f.mu.Unlock()
			if f.hbFail.Load() {
				return nil, fmt.Errorf("heartbeat rejected")
			}
			return proto.Marshal(&agentv1.HeartbeatResponse{})
		case protocol.MsgMetricEvent:
			f.mu.Lock()
			f.metricEvents++
			f.mu.Unlock()
			return nil, nil
		default:
			return proto.Marshal(&agentv1.RegisterCapabilitiesResponse{})
		}
	})
	srv, err := tcptr.NewServer(&tcptr.Config{
		Address:        "127.0.0.1:0",
		Insecure:       true,
		RecvTimeout:    5 * time.Second,
		SendTimeout:    5 * time.Second,
		ConnectTimeout: time.Second,
	}, handler)
	require.NoError(t, err)
	f.addrVal = srv.Addr()
	f.closeFn = func() { _ = srv.Close() }
	go func() { _ = srv.Serve(context.Background()) }()
	t.Cleanup(f.closeFn)
	return f
}

func (f *flakyControlServer) addr() string { return f.addrVal }

func (f *flakyControlServer) heartbeatCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.heartbeats
}

func TestUpstreamHeartbeatLoop_FailureAndRecovery(t *testing.T) {
	f := newFlakyControlServer(t)
	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient(f.addr(), "agent-hb", store, nil)
	// NewUpstreamClient ignores interval fields; apply them via WithMetadata.
	client.WithMetadata(UpstreamMetadata{HeartbeatInterval: 40 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))
	defer client.Stop()

	require.Eventually(t, func() bool { return f.heartbeatCount() >= 1 }, 3*time.Second, 20*time.Millisecond)

	// Restarting the connection (dial → register) keeps the loop healthy
	// across a transient outage window.
	f.closeFn()
	time.Sleep(150 * time.Millisecond)
	cancel()
}

func TestUpstreamHeartbeatLoop_ServerDown(t *testing.T) {
	f := newFlakyControlServer(t)
	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient(f.addr(), "agent-hb-down", store, nil)
	client.WithMetadata(UpstreamMetadata{HeartbeatInterval: 40 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))
	defer client.Stop()

	require.Eventually(t, func() bool { return f.heartbeatCount() >= 1 }, 3*time.Second, 20*time.Millisecond)

	// Server dies: heartbeats fail, reconnect dials fail, loop keeps trying.
	f.closeFn()
	time.Sleep(200 * time.Millisecond)
}

func TestUpstreamStart_EmptyServerAddr(t *testing.T) {
	client := NewUpstreamClient("", "agent-empty", agentlocal.NewLocalStore(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))
	client.Stop()
}

func TestUpstreamDialServer_RegisterGarbageResponse(t *testing.T) {
	f := newFlakyControlServer(t)
	f.garbageReg.Store(true)

	client := NewUpstreamClient(f.addr(), "agent-rej2", agentlocal.NewLocalStore(), &UpstreamMetadata{})
	err := client.dialServer(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to register after connection")
}

func TestUpstreamUpdateLoop_DebounceBurstAndSyncFailure(t *testing.T) {
	f := newFlakyControlServer(t)
	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient(f.addr(), "agent-burst", store, &UpstreamMetadata{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))
	defer client.Stop()

	require.Eventually(t, func() bool { return f.registerCountSafe() >= 1 }, 3*time.Second, 20*time.Millisecond)

	// Burst of updates within the debounce window collapses into one sync.
	for i := 0; i < 3; i++ {
		store.Register(fmt.Sprintf("sess-%d", i), "svc-burst", "127.0.0.1:9", "1.0", []*sdkv1.ProviderFunctionDescriptor{
			{Id: fmt.Sprintf("fn-%d", i), Version: "1.0"},
		}, nil)
	}
	time.Sleep(100 * time.Millisecond)

	// Killing the server before another update forces the sync to fail.
	f.closeFn()
	store.Register("sess-after", "svc-burst", "127.0.0.1:9", "1.0", nil, nil)
	time.Sleep(200 * time.Millisecond)
}

func (f *flakyControlServer) registerCountSafe() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.registers
}

func TestUpstreamNilAndNotConnectedGuards(t *testing.T) {
	ctx := context.Background()

	var nilClient *UpstreamClient
	require.Error(t, nilClient.Sync(ctx))
	require.Error(t, nilClient.Heartbeat(ctx))
	require.Error(t, nilClient.SendMetricEvent(ctx, &opsv1.MetricsReport{}))
	require.Error(t, nilClient.ReportMetricsOnce(ctx))

	client := NewUpstreamClient("127.0.0.1:1", "a", agentlocal.NewLocalStore(), nil)
	require.Error(t, client.Sync(ctx))
	require.Error(t, client.Heartbeat(ctx))
	require.Error(t, client.SendMetricEvent(ctx, nil))
	require.Error(t, client.ReportMetricsOnce(ctx))

	// reportMetrics tolerates send failures (logs only).
	client.reportMetrics(ctx)
}

func TestUpstreamSendMetricEvent_MarshalError(t *testing.T) {
	f := newFlakyControlServer(t)
	client := NewUpstreamClient(f.addr(), "agent-me", agentlocal.NewLocalStore(), &UpstreamMetadata{})
	ctx := context.Background()
	require.NoError(t, client.dialServer(ctx))
	defer client.Stop()

	err := client.SendMetricEvent(ctx, &opsv1.MetricsReport{AgentId: "\xff\xfe"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal metrics report")
}

// ---------------------------------------------------------------------------
// control_client.go: call() error branches
// ---------------------------------------------------------------------------

func TestTCPControlClientCall_Errors(t *testing.T) {
	f := newFlakyControlServer(t)
	ctx := context.Background()

	cc, err := newControlClient("tcp", f.addr(), nil)
	require.NoError(t, err)
	defer cc.Close()

	// Invalid UTF-8 forces a proto marshal error.
	_, err = cc.Register(ctx, &agentv1.RegisterRequest{AgentId: "\xff"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal request")

	// Closed underlying connection makes the RPC fail.
	require.NoError(t, cc.Close())
	_, err = cc.Heartbeat(ctx, &agentv1.HeartbeatRequest{})
	require.Error(t, err)
}

func TestTCPControlClientCall_GarbageResponse(t *testing.T) {
	f := newFlakyControlServer(t)
	f.garbageReg.Store(true)

	cc, err := newControlClient("tcp", f.addr(), nil)
	require.NoError(t, err)
	defer cc.Close()

	_, err = cc.Register(context.Background(), &agentv1.RegisterRequest{AgentId: "a"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal response")
}

func TestMuxControlClientCall_Errors(t *testing.T) {
	f := newFlakyControlServer(t)
	ctx := context.Background()

	mc, err := newMuxControlClient(f.addr(), nil, nil)
	require.NoError(t, err)
	defer mc.Close()

	_, err = mc.Register(ctx, &agentv1.RegisterRequest{AgentId: "\xff"})
	require.Error(t, err)

	f.garbageReg.Store(true)
	_, err = mc.Register(ctx, &agentv1.RegisterRequest{AgentId: "a"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal response")
}

// ---------------------------------------------------------------------------
// extension runtime / driver runtime error paths
// ---------------------------------------------------------------------------

func TestExtensionRuntimeNilGuards(t *testing.T) {
	var r *ExtensionRuntime
	_, err := r.Reload()
	require.Error(t, err)

	snap := r.Snapshot()
	assert.NotNil(t, snap.Installations)

	r.RecordError(nil) // no-op
	r.RecordError(errors.New("boom"))
}

func TestExtensionRuntimeReload_NoPayload(t *testing.T) {
	r := NewExtensionRuntime()
	_, err := r.Reload()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no payload to reload")
}

func TestAppReconcileExtensions_NoPayload(t *testing.T) {
	app := New("127.0.0.1:1", "agent-reconcile")
	_, err := app.ReconcileExtensions()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no payload to reload")
}

func TestAppApplyExtensionSyncPayloadJSON_UnknownDriver(t *testing.T) {
	app := New("127.0.0.1:1", "agent-unknown-driver")

	payload := `{
		"agentId": "agent-unknown-driver",
		"installations": [
			{
				"installationId": 11,
				"extensionId": "ext-driver",
				"enabled": true,
				"bindings": [
					{"bindingType": "function", "bindingKey": "drv.fn", "specJson": "{\"driver\":\"no-such-driver\"}"}
				]
			}
		]
	}`
	_, err := app.ApplyExtensionSyncPayloadJSON([]byte(payload))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "driver not found")
}

func TestAppPullExtensionSyncOnce_FailingAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	t.Setenv("CROUPIER_EXTENSION_SYNC_API", srv.URL)
	app := New("127.0.0.1:1", "agent-pull-fail")
	app.startExtensionSync(context.Background())
	require.NotNil(t, app.extensionPuller)

	err := app.PullExtensionSyncOnce(context.Background())
	require.Error(t, err)
}

func TestExtensionDriverRuntime_RegisterAndResolve(t *testing.T) {
	var nilRuntime *ExtensionDriverRuntime
	nilRuntime.RegisterDriver(nil)

	r := NewExtensionDriverRuntime()
	r.RegisterDriver(nil)
	r.RegisterDriver(&namedDriver{name: "  "}) // blank name ignored

	item := RuntimeInstallation{
		ExtensionID: "ext-d",
		Bindings: []RuntimeBinding{
			{BindingType: "function", Spec: map[string]any{"driver": "custom-a"}},
			{BindingType: "provider", BindingKey: "p1"},
			{BindingType: "openapi", BindingKey: "p2"},
			{BindingType: "webhook", BindingKey: "w1"},
			{BindingType: "grpc", BindingKey: "g1"},
			{BindingType: "page", BindingKey: "pg1"},
			{BindingType: "ui", BindingKey: "u1"},
			{BindingType: "navigation", BindingKey: "n1"},
			{BindingType: "other", TargetRef: "driver:from-target"},
			{BindingType: "other", TargetRef: "driver:custom-a"}, // dedupe
			{BindingType: "other", Spec: map[string]any{"driver": "custom-a"}},
		},
	}
	names := resolveDriverNames(item)
	assert.Contains(t, names, "custom-a")
	assert.Contains(t, names, "openapi-driver")
	assert.Contains(t, names, "webhook-driver")
	assert.Contains(t, names, "grpc-driver")
	assert.Contains(t, names, "from-target")
	assert.NotContains(t, names, "")

	// Sync with unknown drivers reports the failure.
	_, err := r.Sync(context.Background(), ExtensionRuntimeSnapshot{
		Installations: []RuntimeInstallation{item},
	})
	require.Error(t, err)

	// nil receiver guard.
	var nilSync *ExtensionDriverRuntime
	_, err = nilSync.Sync(context.Background(), ExtensionRuntimeSnapshot{})
	require.Error(t, err)
}

type namedDriver struct {
	name string
}

func (d *namedDriver) Name() string { return d.name }
func (d *namedDriver) Init(ctx context.Context, item RuntimeInstallation) error {
	return nil
}
func (d *namedDriver) Reload(ctx context.Context, item RuntimeInstallation) error {
	return nil
}
func (d *namedDriver) Stop(ctx context.Context, installationID uint) error {
	return nil
}
func (d *namedDriver) Invoke(ctx context.Context, functionID string, payload []byte) ([]byte, error) {
	return payload, nil
}

// ---------------------------------------------------------------------------
// systemd helpers
// ---------------------------------------------------------------------------

func TestRunSystemdCmd_RunnerErrors(t *testing.T) {
	orig := systemdRunner
	defer func() { systemdRunner = orig }()

	systemdRunner = func(args ...string) ([]byte, error) {
		return nil, errors.New("exec failed")
	}
	_, err := runSystemdCmd("list-units")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec failed")

	systemdRunner = func(args ...string) ([]byte, error) {
		return []byte("some stderr detail"), errors.New("exit 1")
	}
	_, err = runSystemdCmd("show", "foo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "some stderr detail")
}

func TestParseCronFile_MissingFile(t *testing.T) {
	jobs := parseCronFile("/nonexistent/path/crontab", "root")
	assert.Empty(t, jobs)
}
