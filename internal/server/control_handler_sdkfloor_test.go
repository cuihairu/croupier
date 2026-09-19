package server

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/platform/registry"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
)

// 首个 SDK 版本注册：无高水位 → 放行、函数进会话、注册成功后抬升高水位。
func TestSDKVersionFloor_FirstRegistrationRaises(t *testing.T) {
	svc := newScopeTestService(t)
	ctx := context.Background()

	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-floor",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.list", Version: "1.0.0", Enabled: true},
		},
		Processes: []*agentv1.AgentProcess{
			{ServiceId: "svc-go", SdkLanguage: "go", SdkVersion: "0.3.0", FunctionIds: []string{"player.list"}, GameId: "game-1", Env: "dev"},
		},
	}, "")
	require.NoError(t, err)
	assert.Empty(t, resp.GetWarnings())

	sess := svc.registry.AgentsUnsafe()["agent-floor"]
	require.NotNil(t, sess)
	assert.Contains(t, sess.Functions, "player.list")
	assert.Equal(t, "0.3.0", svc.registry.GetSDKVersionFloor("game-1", "dev", "go"))
}

// 落后高水位 2 个 minor：函数注册被拒（不进会话/契约），连接保持
// （handleRegisterRequest 不返回 error），拒绝以注册警告 + response
// warnings 传达；高水位不被低版本拉低。
func TestSDKVersionFloor_RejectBehindTwoMinors(t *testing.T) {
	svc := newScopeTestService(t)
	ctx := context.Background()
	seed := func() {
		_, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
			AgentId: "agent-floor",
			GameId:  "game-1",
			Env:     "dev",
			Functions: []*agentv1.FunctionDescriptor{
				{Id: "player.list", Version: "1.0.0", Enabled: true},
			},
			Processes: []*agentv1.AgentProcess{
				{ServiceId: "svc-go", SdkLanguage: "go", SdkVersion: "0.3.0", FunctionIds: []string{"player.list"}, GameId: "game-1", Env: "dev"},
			},
		}, "")
		require.NoError(t, err)
	}
	seed()

	// 0.1.4 比 0.3.0 低 2 个 minor → 拒绝其独占函数。
	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-floor",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.ban", Version: "1.0.0", Enabled: true},
		},
		Processes: []*agentv1.AgentProcess{
			{ServiceId: "svc-go", SdkLanguage: "go", SdkVersion: "0.1.4", FunctionIds: []string{"player.ban"}, GameId: "game-1", Env: "dev"},
		},
	}, "")
	require.NoError(t, err, "rejection must keep the connection (no register error)")

	sess := svc.registry.AgentsUnsafe()["agent-floor"]
	require.NotNil(t, sess)
	assert.NotContains(t, sess.Functions, "player.ban", "rejected function must not enter the session")
	// provider 进程记录保留（进程存在是事实，调度第一道门是 agent.Functions）。
	require.Len(t, sess.Providers, 1)
	assert.Equal(t, "0.1.4", sess.Providers[0].SDKVersion)

	assert.Equal(t, "0.3.0", svc.registry.GetSDKVersionFloor("game-1", "dev", "go"), "low version must not lower the floor")

	// 注册警告 + response warnings 双通道。
	got := svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{
		GameID: "game-1", Env: "dev", AgentID: "agent-floor", Code: registry.WarningCodeSDKVersionFloorRejected,
	})
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "svc-go")
	assert.Contains(t, got[0].Message, "function registration rejected")

	var found bool
	for _, w := range resp.GetWarnings() {
		if strings.Contains(w, "function registration rejected") && strings.Contains(w, "svc-go") {
			found = true
		}
	}
	assert.True(t, found, "response warnings must carry the rejection, got %v", resp.GetWarnings())
}

// 落后 1 个 minor（容忍窗内）：放行 + 警告，高水位不抬升。
func TestSDKVersionFloor_AllowBehindOneMinorWithWarning(t *testing.T) {
	svc := newScopeTestService(t)
	ctx := context.Background()
	// 直接经 store 抬升基础高水位。
	svc.registry.ObserveSDKVersion("game-1", "dev", "go", "0.3.0")

	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-warn",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "order.list", Version: "1.0.0", Enabled: true},
		},
		Processes: []*agentv1.AgentProcess{
			{ServiceId: "svc-go", SdkLanguage: "go", SdkVersion: "0.2.5", FunctionIds: []string{"order.list"}, GameId: "game-1", Env: "dev"},
		},
	}, "")
	require.NoError(t, err)

	sess := svc.registry.AgentsUnsafe()["agent-warn"]
	require.NotNil(t, sess)
	assert.Contains(t, sess.Functions, "order.list", "within-tolerance version must register")
	assert.Equal(t, "0.3.0", svc.registry.GetSDKVersionFloor("game-1", "dev", "go"), "behind version must not raise the floor")

	got := svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{
		GameID: "game-1", Env: "dev", AgentID: "agent-warn", Code: registry.WarningCodeSDKVersionBehind,
	})
	require.Len(t, got, 1)
	var found bool
	for _, w := range resp.GetWarnings() {
		if strings.Contains(w, "behind high watermark") {
			found = true
		}
	}
	assert.True(t, found, "response warnings must carry the behind notice, got %v", resp.GetWarnings())
}

// 版本解析失败（"unknown"）：一律放行，不当高水位。
func TestSDKVersionFloor_UnparseableVersionPasses(t *testing.T) {
	svc := newScopeTestService(t)
	ctx := context.Background()
	svc.registry.ObserveSDKVersion("game-1", "dev", "python", "0.3.0")

	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-unknown",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "mail.send", Version: "1.0.0", Enabled: true},
		},
		Processes: []*agentv1.AgentProcess{
			{ServiceId: "svc-py", SdkLanguage: "python", SdkVersion: "unknown", FunctionIds: []string{"mail.send"}, GameId: "game-1", Env: "dev"},
		},
	}, "")
	require.NoError(t, err)
	assert.Empty(t, resp.GetWarnings())

	sess := svc.registry.AgentsUnsafe()["agent-unknown"]
	require.NotNil(t, sess)
	assert.Contains(t, sess.Functions, "mail.send")
	assert.Equal(t, "0.3.0", svc.registry.GetSDKVersionFloor("game-1", "dev", "python"))
}

// 交叉提供保护：被拒进程独占的函数剔除，放行进程也声明的函数保留。
func TestSDKVersionFloor_CrossProvidedFunctionKept(t *testing.T) {
	svc := newScopeTestService(t)
	ctx := context.Background()
	svc.registry.ObserveSDKVersion("game-1", "dev", "go", "1.0.0")

	_, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-cross",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "solo.fn", Version: "1.0.0", Enabled: true},
			{Id: "shared.fn", Version: "1.0.0", Enabled: true},
		},
		Processes: []*agentv1.AgentProcess{
			// go/0.8.0 低 1 个 major → 拒绝，独占 solo.fn、共提 shared.fn。
			{ServiceId: "svc-old", SdkLanguage: "go", SdkVersion: "0.8.0", FunctionIds: []string{"solo.fn", "shared.fn"}, GameId: "game-1", Env: "dev"},
			// python 当前版本 → 放行，声明 shared.fn。
			{ServiceId: "svc-py", SdkLanguage: "python", SdkVersion: "0.3.0", FunctionIds: []string{"shared.fn"}, GameId: "game-1", Env: "dev"},
		},
	}, "")
	require.NoError(t, err)

	sess := svc.registry.AgentsUnsafe()["agent-cross"]
	require.NotNil(t, sess)
	assert.NotContains(t, sess.Functions, "solo.fn", "rejected-process-only function must be dropped")
	assert.Contains(t, sess.Functions, "shared.fn", "cross-provided function must survive")
}

// 无 SDK 自报的进程（自定义游戏服直连）：不参与门槛，函数照常放行，
// 不抬升高水位。
func TestSDKVersionFloor_PlainGameServerUnaffected(t *testing.T) {
	svc := newScopeTestService(t)
	ctx := context.Background()

	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-plain",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "guild.kick", Version: "1.0.0", Enabled: true},
		},
		Processes: []*agentv1.AgentProcess{
			{ServiceId: "gamesvr-1", FunctionIds: []string{"guild.kick"}, GameId: "game-1", Env: "dev"},
		},
	}, "")
	require.NoError(t, err)
	assert.Empty(t, resp.GetWarnings())

	sess := svc.registry.AgentsUnsafe()["agent-plain"]
	require.NotNil(t, sess)
	assert.Contains(t, sess.Functions, "guild.kick")
	assert.Empty(t, svc.registry.GetSDKVersionFloor("game-1", "dev", ""))
}

// 追赶闭环：落后进程升级到高水位后，同函数重新注册成功且无新警告。
func TestSDKVersionFloor_CatchUpReRegisters(t *testing.T) {
	svc := newScopeTestService(t)
	ctx := context.Background()

	reg := func(version string) *agentv1.RegisterResponse {
		resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
			AgentId: "agent-catchup",
			GameId:  "game-1",
			Env:     "dev",
			Functions: []*agentv1.FunctionDescriptor{
				{Id: "player.query", Version: "1.0.0", Enabled: true},
			},
			Processes: []*agentv1.AgentProcess{
				{ServiceId: "svc-go", SdkLanguage: "go", SdkVersion: version, FunctionIds: []string{"player.query"}, GameId: "game-1", Env: "dev"},
			},
		}, "")
		require.NoError(t, err)
		return resp
	}

	reg("0.3.0")
	reg("0.1.0") // 低 2 minor 被拒
	sess := svc.registry.AgentsUnsafe()["agent-catchup"]
	require.NotNil(t, sess)
	assert.NotContains(t, sess.Functions, "player.query")

	final := reg("0.3.0") // 追赶回高水位 → 重新注册成功
	sess = svc.registry.AgentsUnsafe()["agent-catchup"]
	require.NotNil(t, sess)
	assert.Contains(t, sess.Functions, "player.query")
	for _, w := range final.GetWarnings() {
		assert.NotContains(t, w, "sdk_version_floor_rejected")
	}
}
