package ops

// deploy_agent.go 补测：NewDeployAgent 默认下载命令与 DownloadCmd 切换、
// StagePackage 全部分支（对象存储未装配 / 签名 URL 失败 / 下载执行失败 /
// 非零退出码 / wget 与 curl 两条命令形态与成功路径）、ApplyUpdate 三分支
// （错误透传 / 失败消息 / 成功返回指纹）、Healthy 无最近指标分支。
//
// 复用 coverage_v3_test.go 的 fakeResolver/fakeCaller（respFor 按 msgID 回
// 罐头响应、lastBody 捕获请求体）；对象存储用本地 stubObjectStore。

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

// stubObjectStore 签名 URL 可注入失败的 objstore.Store 替身（其余方法 unused）。
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

// deployAgentFixture 装配一个 scope 就绪、会话可解析的 DeployAgent。
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

// setupOpsResolver 装配全局 ops client 的会话解析器并注册清理。
func setupOpsResolver(t *testing.T, caller *fakeCaller) {
	t.Helper()
	client := agentops.GetAgentOpsClient()
	client.SetSessionResolver(&fakeResolver{conns: map[string]transport.SessionCaller{"a1": caller}})
	t.Cleanup(func() { client.SetSessionResolver(nil) })
}

// opsGetClient 返回全局 ops client（测试内直接改会话解析器用）。
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
	// resolver 未装配 → GetClient 失败 → "download: ops client unavailable"
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
		// 断言实际下发给 agent 的命令形态
		var req opsv1.ExecuteCommandRequest
		require.NoError(t, proto.Unmarshal(caller.lastBody[uint32(protocol.MsgExecuteCommandRequest)], &req))
		require.Equal(t, tc.wantCmd, req.Command)
		require.Contains(t, req.Args, tc.wantArg)
		require.Contains(t, req.Args, "https://obj/p1.tgz")
	}
}

func TestApplyUpdateBranches(t *testing.T) {
	// 错误透传（resolver 未装配）
	a, _ := deployAgentFixture(t)
	opsGetClient().SetSessionResolver(nil)
	_, err := a.ApplyUpdate(context.Background(), "a1", "game", "/tmp/x", "sha")
	require.ErrorContains(t, err, "ops client unavailable")

	// 失败消息
	a2, caller2 := deployAgentFixture(t)
	caller2.respFor[uint32(protocol.MsgUpdateProcessRequest)] = &opsv1.UpdateProcessResponse{Success: false, Message: "checksum mismatch"}
	_, err = a2.ApplyUpdate(context.Background(), "a1", "game", "/tmp/x", "sha")
	require.ErrorContains(t, err, "checksum mismatch")

	// 成功返回指纹
	a3, caller3 := deployAgentFixture(t)
	caller3.respFor[uint32(protocol.MsgUpdateProcessRequest)] = &opsv1.UpdateProcessResponse{Success: true, NewFingerprint: "fp-42"}
	fp, err := a3.ApplyUpdate(context.Background(), "a1", "game", "/tmp/x", "sha")
	require.NoError(t, err)
	require.Equal(t, "fp-42", fp)
}

func TestHealthyNoRecentMetrics(t *testing.T) {
	// MetricsStore 空 → GetLatest 未命中 → "no recent metrics"
	metrics := registry.NewMetricsStore()
	a := &DeployAgent{ctx: &svc.ServiceContext{MetricsStore: metrics}}
	ok, msg := a.Healthy(context.Background(), "a1")
	require.False(t, ok)
	require.Equal(t, "no recent metrics", msg)

	// 未装配 store → 视作健康
	a2 := &DeployAgent{ctx: &svc.ServiceContext{}}
	ok2, msg2 := a2.Healthy(context.Background(), "a1")
	require.True(t, ok2)
	require.Contains(t, msg2, "assume healthy")
}
