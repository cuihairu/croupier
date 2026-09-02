package agent

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentlocal "github.com/cuihairu/croupier/internal/platform/agentlocal"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// upstream.go: reconnectLoop / metricsLoop / updateLoop / syncWithRetry
// ---------------------------------------------------------------------------

func TestUpstreamReconnectLoop_ConnectsAndCancelled(t *testing.T) {
	f := newFakeControlServer(t)
	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient(f.addr(), "agent-rc", store, &UpstreamMetadata{})
	connected := make(chan struct{}, 2)
	client.OnConnected(func() { connected <- struct{}{} })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.reconnectLoop(ctx, true)

	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("reconnectLoop did not connect")
	}
	require.Eventually(t, func() bool { return f.registerCount() >= 1 }, 2*time.Second, 50*time.Millisecond)
	client.Stop()

	// Cancelled context exits immediately, even against an unreachable server.
	cancelledCtx, cancel2 := context.WithCancel(context.Background())
	cancel2()
	client2 := NewUpstreamClient("127.0.0.1:1", "agent-rc2", store, &UpstreamMetadata{})
	done := make(chan struct{})
	go func() {
		client2.reconnectLoop(cancelledCtx, true)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reconnectLoop did not exit on cancelled context")
	}
}

func TestUpstreamMetricsLoop_ReportsImmediately(t *testing.T) {
	f := newFakeControlServer(t)
	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient(f.addr(), "agent-metrics", store, &UpstreamMetadata{})
	client.WithMetricsReporting(50 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))
	defer client.Stop()

	require.Eventually(t, func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.metricEvents >= 1
	}, 3*time.Second, 50*time.Millisecond)
}

func TestUpstreamUpdateLoop_SyncsOnStoreChange(t *testing.T) {
	f := newFakeControlServer(t)
	store := agentlocal.NewLocalStore()

	client := NewUpstreamClient(f.addr(), "agent-upd", store, &UpstreamMetadata{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, client.Start(ctx))
	defer client.Stop()

	require.Eventually(t, func() bool { return f.registerCount() >= 1 }, 3*time.Second, 50*time.Millisecond)
	base := f.registerCount()

	// Store changes trigger the debounced update loop which re-syncs.
	store.Register("sess-upd", "svc-upd", "127.0.0.1:9", "1.0", []*sdkv1.ProviderFunctionDescriptor{
		{Id: "fn-upd", Version: "1.0", Resource: "player", Operation: "kick"},
	}, nil)

	require.Eventually(t, func() bool { return f.registerCount() > base }, 3*time.Second, 50*time.Millisecond)
}

func TestUpstreamSyncWithRetry_FailureAndCancel(t *testing.T) {
	store := agentlocal.NewLocalStore()
	client := NewUpstreamClient("127.0.0.1:1", "agent-retry", store, &UpstreamMetadata{})

	err := client.syncWithRetry(context.Background(), 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	err = client.syncWithRetry(cancelledCtx, 3)
	require.ErrorIs(t, err, context.Canceled)

	// attempts <= 0 is normalized to 1.
	require.Error(t, client.syncWithRetry(context.Background(), 0))
}

func TestUpstreamSendTaskEvent_NotConnectedAndMarshalError(t *testing.T) {
	f := newFakeControlServer(t)
	store := agentlocal.NewLocalStore()
	ctx := context.Background()

	client := NewUpstreamClient(f.addr(), "agent-ste", store, &UpstreamMetadata{})
	require.NoError(t, client.dialServer(ctx))

	// Invalid UTF-8 in a proto3 string forces a marshal error.
	err := client.SendTaskEvent(ctx, &sdkv1.TaskEvent{TaskId: "\xff\xfe"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal task event")

	client.Stop()
	// Client object still set but connection closed.
	err = client.SendTaskEvent(ctx, &sdkv1.TaskEvent{TaskId: "t"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestUpstreamWithMetadata_Timeouts(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "a", agentlocal.NewLocalStore(), nil)

	client.WithMetadata(UpstreamMetadata{
		DialTimeout:       time.Second,
		RequestTimeout:    2 * time.Second,
		HeartbeatInterval: 3 * time.Second,
		Labels:            map[string]string{"k": "v"},
	})
	assert.Equal(t, time.Second, client.dialTimeout)
	assert.Equal(t, 2*time.Second, client.requestTimeout)
	assert.Equal(t, 3*time.Second, client.heartbeatInterval)
	assert.Equal(t, map[string]string{"k": "v"}, client.labels)

	// Zero values keep the previous configuration.
	client.WithMetadata(UpstreamMetadata{})
	assert.Equal(t, time.Second, client.dialTimeout)
	assert.Equal(t, 3*time.Second, client.heartbeatInterval)
}

func TestUpstreamComposeLabels_DynamicVariants(t *testing.T) {
	client := NewUpstreamClient("127.0.0.1:1", "a", agentlocal.NewLocalStore(), &UpstreamMetadata{
		Labels: map[string]string{"base": "1"},
	})

	client.SetDynamicLabelsProvider(func() map[string]string { return map[string]string{} })
	assert.Equal(t, map[string]string{"base": "1"}, client.composeLabels())

	client.SetDynamicLabelsProvider(func() map[string]string {
		return map[string]string{"dyn": "2", "  ": "skipped"}
	})
	assert.Equal(t, map[string]string{"base": "1", "dyn": "2"}, client.composeLabels())
}

func TestBuildProviders_InstanceVariants(t *testing.T) {
	now := time.Now()
	localData := map[string][]agentlocal.Instance{
		// Empty function IDs are skipped.
		"": {{ProviderID: "svc-x", Addr: "127.0.0.1:1", LastSeen: now}},
		// Instances without provider ID are skipped.
		"fn.orphan": {{ProviderID: "", Addr: "127.0.0.1:2", LastSeen: now}},
		// Same provider reuses one AgentProcess and dedupes function IDs;
		// empty addr/version fall back to sibling instances.
		"fn.shared": {
			{ProviderID: "svc-a", Addr: "10.0.0.1:1000", Version: "1.0", LastSeen: now},
			{ProviderID: "svc-a", LastSeen: now.Add(time.Second)},
		},
		// Provider without version falls back to the function version snapshot.
		"fn.via-fn": {
			{
				ProviderID: "svc-b",
				Addr:       "10.0.0.2:2000",
				LastSeen:   now,
				Metadata:   map[string]string{"sdkLanguage": "go", "sdkVersion": "1.2", "sdkName": "croupier-go", "gameId": "g1", "env": "prod"},
			},
		},
	}
	versions := map[string]map[string]string{"fn.via-fn": {"inst1": "9.9"}}

	procs := buildProviders(localData, versions)
	require.Len(t, procs, 2)

	byService := map[string]*agentv1.AgentProcess{}
	for _, p := range procs {
		byService[p.ServiceId] = p
	}
	require.Contains(t, byService, "svc-a")
	require.Contains(t, byService, "svc-b")

	a := byService["svc-a"]
	assert.Equal(t, "10.0.0.1:1000", a.Addr)
	assert.Equal(t, "1.0", a.Version)
	assert.Equal(t, []string{"fn.shared"}, a.FunctionIds)
	assert.Equal(t, now.Add(time.Second).Unix(), a.LastSeenUnix)

	b := byService["svc-b"]
	assert.Equal(t, "go", b.SdkLanguage)
	assert.Equal(t, "1.2", b.SdkVersion)
	assert.Equal(t, "croupier-go", b.SdkName)
	assert.Equal(t, "g1", b.GameId)
	assert.Equal(t, "prod", b.Env)
	assert.Equal(t, "9.9", b.Version)
}

// ---------------------------------------------------------------------------
// app.go: Run failure paths / maintenance loop / nil guards
// ---------------------------------------------------------------------------

func TestAppRun_InvalidLocalAddr(t *testing.T) {
	app := New("127.0.0.1:1", "agent-bad-addr")
	app.SetLocalAddr("invalid host::notaport")
	err := app.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start local server")
}

func TestAppRun_ProviderLoadFailure(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte(":[unclosed"), 0o600))

	app := NewWithConfigDir("127.0.0.1:1", "agent-bad-yaml", dir)
	app.SetLocalAddr("127.0.0.1:0")
	err := app.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse provider config")
}

func TestAppRun_LocalServerAddrConflict(t *testing.T) {
	// Occupy a port first, then let Run bind the same one.
	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer blocker.Close()

	app := NewWithConfigDir("127.0.0.1:1", "agent-conflict", t.TempDir())
	app.SetLocalAddr(blocker.Addr().String())
	err = app.Run(context.Background())
	require.Error(t, err)
}

func TestAppStartMaintenance_TickerFires(t *testing.T) {
	t.Setenv("CROUPIER_AGENTLOCAL_PRUNE_INTERVAL", "1ms")
	t.Setenv("CROUPIER_AGENTLOCAL_MAX_AGE", "1ms")
	t.Setenv("CROUPIER_AGENTLOCAL_TASKRESULT_MAX_AGE", "1ms")

	app := New("127.0.0.1:1", "agent-prune")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.startMaintenance(ctx)

	// Give the fast ticker a chance to prune; no assertion on effect, just
	// make sure the loop body executes without deadlocking.
	time.Sleep(20 * time.Millisecond)
}

func TestAppNilReceiverGuards(t *testing.T) {
	var app *App
	app.WithUpstreamMetadata(UpstreamMetadata{GameID: "g"})
	app.WithUpstreamTLSConfig(nil)
	app.SetUpstreamTransportKind("tcp")
	app.WithOutboundTLSConfig(nil)
	app.Stop()
	assert.Nil(t, app.Store())
	require.Error(t, app.SyncUpstream(context.Background()))
	require.Error(t, app.HeartbeatUpstream(context.Background()))
	assert.Nil(t, app.Tasks())
	assert.Zero(t, app.ActiveTaskCount())
}

func TestAppWithUpstreamMetadata_AfterLocalServerStarted(t *testing.T) {
	app := NewWithConfigDir("127.0.0.1:1", "agent-meta", t.TempDir())
	app.SetLocalAddr("127.0.0.1:0")
	require.NoError(t, app.StartLocalServer())
	defer app.Stop()

	app.WithUpstreamMetadata(UpstreamMetadata{GameID: "game-meta", Env: "dev"})
	require.NotNil(t, app.localHandler)
}

func TestAppApplyExtensionSyncPayload_NilPayload(t *testing.T) {
	app := New("127.0.0.1:1", "agent-ext")
	_, err := app.ApplyExtensionSyncPayload(nil)
	require.Error(t, err)

	_, err = app.ApplyExtensionSyncPayloadJSON([]byte("{invalid"))
	require.Error(t, err)
}

func TestAppInvokeExtensionFunction_ExternalCallerWithoutProviderManager(t *testing.T) {
	app := New("127.0.0.1:1", "agent-ext-call")
	app.extensionMu.Lock()
	app.extensionRoutes["external.pay.submit"] = extensionFunctionRoute{Driver: "workflow-driver"}
	app.extensionMu.Unlock()

	_, err := app.invokeExtensionFunction(context.Background(), "external.pay.submit", []byte("raw"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider manager not initialized")
}

func TestProviderManagerWrapper_WithoutManagerFallsBack(t *testing.T) {
	w := &providerManagerWrapper{app: New("127.0.0.1:1", "agent-wrapper")}
	assert.False(t, w.IsPlatformFunction("plain.fn"))
	_, err := w.Call(context.Background(), "plain.fn", nil)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// provider.go: SyncExtensionProviders edges / Load read failure / heartbeat
// ---------------------------------------------------------------------------

func TestProviderManagerLoad_UnreadableConfig(t *testing.T) {
	dir := t.TempDir()
	// A directory named providers.yaml makes os.ReadFile fail.
	require.NoError(t, os.Mkdir(filepath.Join(dir, "providers.yaml"), 0o755))

	m := NewProviderManager(agentlocal.NewLocalStore(), dir, nil)
	err := m.Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read provider config")
}

func TestProviderManagerSyncExtensionProviders_RemoveAndInvalid(t *testing.T) {
	store := agentlocal.NewLocalStore()
	m := NewProviderManager(store, t.TempDir(), nil)
	ctx := context.Background()

	entry := ProviderEntry{Enabled: true, Type: "openapi", Config: map[string]interface{}{}}
	require.NoError(t, m.SyncExtensionProviders(ctx, map[string]ProviderEntry{
		"pay": entry,
		"  ":  entry, // blank name skipped
		"bad": {Enabled: true, Type: "unsupported-type"},
		"off": {Enabled: false, Type: "openapi"},
	}))

	// Sync again without "pay": the managed provider must be removed.
	require.NoError(t, m.SyncExtensionProviders(ctx, map[string]ProviderEntry{}))
	assert.False(t, m.IsPlatformFunction("pay.do"))

	// Re-add a valid one to prove the add path still works.
	require.NoError(t, m.SyncExtensionProviders(ctx, map[string]ProviderEntry{"pay2": entry}))
	assert.True(t, m.IsPlatformFunction("pay2.anything"))
	require.NoError(t, m.Close())
}

func TestProviderManagerStartHeartbeat_FastInterval(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "providers.yaml"), []byte(`
providers:
  hb:
    enabled: true
    type: openapi
`), 0o600))

	t.Setenv("CROUPIER_AGENTLOCAL_PROVIDER_HEARTBEAT", "1ms")
	m := NewProviderManager(agentlocal.NewLocalStore(), dir, nil)
	require.NoError(t, m.Load(context.Background()))
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, m.Close())
}

// ---------------------------------------------------------------------------
// ops_server.go
// ---------------------------------------------------------------------------

func TestNewOpsServer_NilConfigUsesDefaults(t *testing.T) {
	s := NewOpsServer(nil, "a", "v", nil)
	require.NotNil(t, s)
	assert.NotNil(t, s.config)
	assert.False(t, s.config.Enabled)
}

func TestOpsServerDisabledGuards(t *testing.T) {
	s := NewOpsServer(nil, "a", "v", nil)
	ctx := context.Background()

	_, err := s.StopProcess(ctx, &opsv1.StopProcessRequest{ProcessName: "x"})
	require.Error(t, err)
	_, err = s.StartProcess(ctx, &opsv1.StartProcessRequest{ProcessName: "x"})
	require.Error(t, err)
}

func TestOpsServerRestartProcess_StartFailure(t *testing.T) {
	cfg := DefaultOpsConfig()
	cfg.Enabled = true
	cfg.AllowRestart = true
	s := NewOpsServer(cfg, "a", "v", nil)

	// Register a running process, then sabotage its command so the restart's
	// start attempt fails.
	p := &managedProcess{
		name:   "victim",
		config: ManagedProcessConfig{Command: "sleep", Args: []string{"30"}},
		state:  opsv1.ProcessState_PROCESS_STATE_RUNNING,
		stopCh: make(chan struct{}),
	}
	s.mu.Lock()
	s.processes["victim"] = p
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.processes, "victim")
		s.mu.Unlock()
	}()

	require.NoError(t, s.startProcess(p))
	p.config.Command = "/nonexistent/command-xyz"

	_, err := s.RestartProcess(context.Background(), &opsv1.RestartProcessRequest{ProcessName: "victim"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to restart process")
}

func TestOpsServerStartProcess_ConfigEdges(t *testing.T) {
	dir := t.TempDir()
	cfg := DefaultOpsConfig()
	cfg.Enabled = true
	cfg.AllowRestart = true
	cfg.ManagedProcesses["envproc"] = ManagedProcessConfig{
		Command:    "sleep",
		Args:       []string{"30"},
		WorkingDir: dir,
		Env:        map[string]string{"TEST_VAR": "1"},
	}
	s := NewOpsServer(cfg, "a", "v", nil)
	// NOTE: s.Close()/s.Stop() crash due to a lock-mismatch bug in production
	// code (Lock paired with RUnlock); clean up the process manually instead.
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if p, ok := s.processes["envproc"]; ok {
			p.mu.Lock()
			s.stopProcess(p)
			p.mu.Unlock()
		}
	}()

	resp, err := s.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "envproc"})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Greater(t, resp.Pid, int32(0))

	// Starting twice reports already-running.
	_, err = s.StartProcess(context.Background(), &opsv1.StartProcessRequest{ProcessName: "envproc"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestOpsServerMonitorProcess_NilCmd(t *testing.T) {
	s := NewOpsServer(nil, "a", "v", nil)
	s.monitorProcess(&managedProcess{name: "empty"}) // returns without panic
}

func TestOpsServerExecuteCommand_Variants(t *testing.T) {
	cfg := DefaultOpsConfig()
	cfg.Enabled = true
	cfg.AllowExec = true
	cfg.ExecTimeout = 10 * time.Second
	s := NewOpsServer(cfg, "a", "v", nil)
	ctx := context.Background()

	// Missing binary yields -1 (not an ExitError).
	resp, err := s.ExecuteCommand(ctx, &opsv1.ExecuteCommandRequest{Command: "definitely-not-a-command-xyz"})
	require.NoError(t, err)
	assert.Equal(t, int32(-1), resp.ExitCode)

	// WorkingDir + Env + per-request timeout override.
	dir := t.TempDir()
	resp, err = s.ExecuteCommand(ctx, &opsv1.ExecuteCommandRequest{
		Command:        "pwd",
		WorkingDir:     dir,
		Env:            map[string]string{"OPS_TEST_MARKER": "1"},
		TimeoutSeconds: 5,
	})
	require.NoError(t, err)
	assert.Zero(t, resp.ExitCode)
	assert.Equal(t, dir, strings.TrimSpace(resp.StdOut))

	// Timeout above the hard cap is clamped without failing fast commands.
	resp, err = s.ExecuteCommand(ctx, &opsv1.ExecuteCommandRequest{
		Command:        "true",
		TimeoutSeconds: 400,
	})
	require.NoError(t, err)
	assert.Zero(t, resp.ExitCode)
}

func TestOpsServerJSON_InvalidRequests(t *testing.T) {
	s := NewOpsServer(nil, "a", "v", nil)
	ctx := context.Background()

	_, err := s.ListServicesJSON(ctx, []byte("{invalid"))
	require.Error(t, err)

	_, err = s.GetServiceStatusJSON(ctx, []byte("{invalid"))
	require.Error(t, err)
}

func TestOpsServerServiceMethods_EnvironmentDependent(t *testing.T) {
	s := NewOpsServer(nil, "a", "v", nil)
	ctx := context.Background()

	// Whether systemd exists is environment-specific; both outcomes are fine,
	// but one of the branches must run without panicking.
	if resp, err := s.ListServices(ctx, &ListServicesRequest{NamePattern: "no-such-unit"}); err == nil {
		require.NotNil(t, resp)
	} else {
		assert.Nil(t, resp)
	}

	if resp, err := s.ListServicesJSON(ctx, []byte(`{"namePattern":"no-such-unit"}`)); err == nil {
		require.NotNil(t, resp)
	}

	if resp, err := s.GetServiceStatus(ctx, &GetServiceStatusRequest{Name: "no-such-unit-xyz"}); err == nil {
		require.NotNil(t, resp)
	} else {
		assert.Nil(t, resp)
	}

	if _, err := s.ListCronJobs(ctx); err != nil {
		// Allowed to fail when no crontab exists.
		_ = err
	}
}

// ---------------------------------------------------------------------------
// systemd_parse.go pure helpers
// ---------------------------------------------------------------------------

func TestMapStartTypeVariants(t *testing.T) {
	assert.Equal(t, "auto", mapStartType("enabled"))
	assert.Equal(t, "manual", mapStartType("disabled"))
	assert.Equal(t, "unknown", mapStartType("static"))
}

func TestMapActiveStateVariants(t *testing.T) {
	assert.Equal(t, "running", mapActiveState("active (running)"))
	assert.Equal(t, "running", mapActiveState("active"))
	assert.Equal(t, "stopped", mapActiveState("inactive (dead)"))
	assert.Equal(t, "stopped", mapActiveState("inactive"))
	assert.Equal(t, "stopped", mapActiveState("failed"))
	assert.Equal(t, "unknown", mapActiveState("activating"))
}

func TestMapSubStateVariants(t *testing.T) {
	assert.Equal(t, "running", mapSubState("running"))
	assert.Equal(t, "stopped", mapSubState("dead"))
	assert.Equal(t, "stopped", mapSubState("failed"))
	assert.Equal(t, "stopped", mapSubState("exited"))
	assert.Equal(t, "stopped", mapSubState("stop-sigterm"))
	assert.Equal(t, "", mapSubState("auto-restart"))
}

func TestAfterColonVariants(t *testing.T) {
	assert.Equal(t, "value", afterColon("Key:   value"))
	assert.Equal(t, "value", afterColon("Key:value"))
	assert.Equal(t, "", afterColon("no colon here"))
	assert.Equal(t, "", afterColon(""))
}
