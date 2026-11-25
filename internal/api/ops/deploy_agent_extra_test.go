package ops

// Extra coverage for deploy_agent.go: NewDeployAgent's default download command and
// DownloadCmd switching, all StagePackage branches (object store not wired / signed
// URL failure / download execution failure / non-zero exit code / the wget and curl
// command shapes and success paths), ApplyUpdate's three branches (error passthrough /
// failure message / success returning the fingerprint), and Healthy's no-recent-metrics
// branch.
//
// Reuses coverage_v3_test.go's fakeResolver/fakeCaller (respFor returns canned
// responses per msgID, lastBody captures the request body); object storage uses the
// local stubObjectStore.

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	agentops "github.com/cuihairu/croupier/internal/logic/ops"
	"github.com/cuihairu/croupier/internal/model"
	objstore "github.com/cuihairu/croupier/internal/platform/objstore"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// stubObjectStore is an objstore.Store stand-in whose signed URL can be made to fail
// (the remaining methods are unused).
type stubObjectStore struct {
	url string
	err error
}

func (s *stubObjectStore) Put(context.Context, string, objstore.ReadSeeker, int64, string) error {
	return errors.New("unused")
}
func (s *stubObjectStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("unused")
}
func (s *stubObjectStore) SignedURL(context.Context, string, string, time.Duration) (string, error) {
	return s.url, s.err
}
func (s *stubObjectStore) Delete(context.Context, string) error {
	return errors.New("unused")
}
func (s *stubObjectStore) List(context.Context, string, string, string, int) (objstore.ListResult, error) {
	return objstore.ListResult{}, errors.New("unused")
}
func (s *stubObjectStore) CreatePrefix(context.Context, string) error {
	return errors.New("unused")
}
func (s *stubObjectStore) RenamePrefix(context.Context, string, string) error {
	return errors.New("unused")
}

// deployAgentFixture wires up a DeployAgent with scope ready and resolvable sessions.
func deployAgentFixture(t *testing.T) (*DeployAgent, *fakeCaller) {
	t.Helper()
	store := registry.NewStore()
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{AgentID: "a1", GameID: "demo", Env: "prod"}))
	svcCtx := &svc.ServiceContext{RegistryStore: store, ObjectStore: &stubObjectStore{url: "https://obj/p1.tgz"}}
	a := NewDeployAgent(svcCtx)
	caller := &fakeCaller{respFor: map[uint32]proto.Message{}}
	setupOpsResolver(t, caller)
	return a, caller
}

// setupOpsResolver wires the global ops client's session resolver and registers cleanup.
func setupOpsResolver(t *testing.T, caller *fakeCaller) {
	t.Helper()
	client := agentops.GetAgentOpsClient()
	client.SetSessionResolver(&fakeResolver{conns: map[string]transport.SessionCaller{"a1": caller}})
	t.Cleanup(func() { client.SetSessionResolver(nil) })
}

// opsGetClient returns the global ops client (tests mutate its session resolver directly).
func opsGetClient() *agentops.AgentOpsClient { return agentops.GetAgentOpsClient() }

func TestNewDeployAgentDefaults(t *testing.T) {
	a := NewDeployAgent(&svc.ServiceContext{})
	require.Equal(t, "curl", a.downloadCmd)
	a.DownloadCmd("wget")
	require.Equal(t, "wget", a.downloadCmd)
}

func TestStagePackageObjectStoreNotConfigured(t *testing.T) {
	a := NewDeployAgent(&svc.ServiceContext{})
	_, err := a.StagePackage(context.Background(), "a1", &model.DeployPackage{PackageID: "p1", Version: "v1"})
	require.ErrorContains(t, err, "object store not configured")
}

func TestStagePackageSignedURLError(t *testing.T) {
	a, _ := deployAgentFixture(t)
	a.ctx.ObjectStore = &stubObjectStore{err: errors.New("denied")}
	_, err := a.StagePackage(context.Background(), "a1", &model.DeployPackage{PackageID: "p1", Version: "v1", ObjectKey: "k"})
	require.ErrorContains(t, err, "signed url: denied")
}

func TestStagePackageDownloadError(t *testing.T) {
	a, _ := deployAgentFixture(t)
	// resolver not wired → GetClient fails → "download: ops client unavailable"
	client := opsGetClient()
	client.SetSessionResolver(nil)
	_, err := a.StagePackage(context.Background(), "a1", &model.DeployPackage{PackageID: "p1", Version: "v1", ObjectKey: "k"})
	require.ErrorContains(t, err, "download:")
}

func TestStagePackageExitCodeNonZero(t *testing.T) {
	a, caller := deployAgentFixture(t)
	caller.respFor[uint32(protocol.MsgExecuteCommandRequest)] = &opsv1.ExecuteCommandResponse{ExitCode: 3, StdOut: "boom"}
	_, err := a.StagePackage(context.Background(), "a1", &model.DeployPackage{PackageID: "p1", Version: "v1", ObjectKey: "k"})
	require.ErrorContains(t, err, "download exit 3: boom")
}

func TestStagePackageSuccessCurlAndWget(t *testing.T) {
	for _, tc := range []struct {
		cmd     string
		wantCmd string
		wantArg string
	}{
		{cmd: "", wantCmd: "curl", wantArg: "-fsSL"},
		{cmd: "wget", wantCmd: "wget", wantArg: "-O"},
	} {
		a, caller := deployAgentFixture(t)
		if tc.cmd != "" {
			a.DownloadCmd(tc.cmd)
		}
		caller.respFor[uint32(protocol.MsgExecuteCommandRequest)] = &opsv1.ExecuteCommandResponse{ExitCode: 0}
		staged, err := a.StagePackage(context.Background(), "a1", &model.DeployPackage{PackageID: "p1", Version: "v1", ObjectKey: "k"})
		require.NoError(t, err)
		require.Equal(t, "/tmp/croupier-deploy-p1-v1", staged)
		// assert the shape of the command actually dispatched to the agent
		var req opsv1.ExecuteCommandRequest
		require.NoError(t, proto.Unmarshal(caller.lastBody[uint32(protocol.MsgExecuteCommandRequest)], &req))
		require.Equal(t, tc.wantCmd, req.Command)
		require.Contains(t, req.Args, tc.wantArg)
		require.Contains(t, req.Args, "https://obj/p1.tgz")
	}
}

func TestApplyUpdateBranches(t *testing.T) {
	// error passthrough (resolver not wired)
	a, _ := deployAgentFixture(t)
	opsGetClient().SetSessionResolver(nil)
	_, err := a.ApplyUpdate(context.Background(), "a1", "game", "/tmp/x", "sha")
	require.ErrorContains(t, err, "ops client unavailable")

	// failure message
	a2, caller2 := deployAgentFixture(t)
	caller2.respFor[uint32(protocol.MsgUpdateProcessRequest)] = &opsv1.UpdateProcessResponse{Success: false, Message: "checksum mismatch"}
	_, err = a2.ApplyUpdate(context.Background(), "a1", "game", "/tmp/x", "sha")
	require.ErrorContains(t, err, "checksum mismatch")

	// success returns the fingerprint
	a3, caller3 := deployAgentFixture(t)
	caller3.respFor[uint32(protocol.MsgUpdateProcessRequest)] = &opsv1.UpdateProcessResponse{Success: true, NewFingerprint: "fp-42"}
	fp, err := a3.ApplyUpdate(context.Background(), "a1", "game", "/tmp/x", "sha")
	require.NoError(t, err)
	require.Equal(t, "fp-42", fp)
}

func TestHealthyNoRecentMetrics(t *testing.T) {
	// empty MetricsStore → GetLatest misses → "no recent metrics"
	metrics := registry.NewMetricsStore()
	a := &DeployAgent{ctx: &svc.ServiceContext{MetricsStore: metrics}}
	ok, msg := a.Healthy(context.Background(), "a1")
	require.False(t, ok)
	require.Equal(t, "no recent metrics", msg)

	// no store wired → treated as healthy
	a2 := &DeployAgent{ctx: &svc.ServiceContext{}}
	ok2, msg2 := a2.Healthy(context.Background(), "a1")
	require.True(t, ok2)
	require.Contains(t, msg2, "assume healthy")
}
