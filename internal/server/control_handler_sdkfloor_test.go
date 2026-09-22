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

// 配置最低版本（registry.sdkVersionMinimums go=0.3.0）：即使没有任何
// 高水位观测，自报 0.1.0 的 provider 独占声明函数也不注册，拒绝以注册
// 警告 + response warnings 双通道传达，连接保持。
func TestSDKVersionFloor_ConfiguredMinimumRejects(t *testing.T) {
	svc := newScopeTestService(t)
	svc.SetSDKVersionMinimums(map[string]string{"go": "0.3.0"})
	ctx := context.Background()

	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-min",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.ban", Version: "1.0.0", Enabled: true},
		},
		Processes: []*agentv1.AgentProcess{
			{ServiceId: "svc-go", SdkLanguage: "go", SdkVersion: "0.1.0", FunctionIds: []string{"player.ban"}, GameId: "game-1", Env: "dev"},
		},
	}, "")
	require.NoError(t, err, "rejection must keep the connection (no register error)")

	sess := svc.registry.AgentsUnsafe()["agent-min"]
	require.NotNil(t, sess)
	assert.NotContains(t, sess.Functions, "player.ban", "below configured minimum must not register")
	require.Len(t, sess.Providers, 1, "provider record must be kept")

	got := svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{
		GameID: "game-1", Env: "dev", AgentID: "agent-min", Code: registry.WarningCodeSDKVersionBelowMinimum,
	})
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "below configured minimum 0.3.0")

	var found bool
	for _, w := range resp.GetWarnings() {
		if strings.Contains(w, "below configured minimum") && strings.Contains(w, "svc-go") {
			found = true
		}
	}
	assert.True(t, found, "response warnings must carry the rejection, got %v", resp.GetWarnings())
}

// 自报版本等于/高于配置最低版本：正常注册，无告警。
func TestSDKVersionFloor_ConfiguredMinimumAllowsAtOrAbove(t *testing.T) {
	svc := newScopeTestService(t)
	svc.SetSDKVersionMinimums(map[string]string{"go": "0.3.0"})
	ctx := context.Background()

	for _, tc := range []struct{ agent, version, fn string }{
		{"agent-min-eq", "0.3.0", "player.list"},
		{"agent-min-hi", "0.4.1", "player.get"},
	} {
		resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
			AgentId: tc.agent,
			GameId:  "game-1",
			Env:     "dev",
			Functions: []*agentv1.FunctionDescriptor{
				{Id: tc.fn, Version: "1.0.0", Enabled: true},
			},
			Processes: []*agentv1.AgentProcess{
				{ServiceId: "svc-go", SdkLanguage: "go", SdkVersion: tc.version, FunctionIds: []string{tc.fn}, GameId: "game-1", Env: "dev"},
			},
		}, "")
		require.NoError(t, err)
		assert.Empty(t, resp.GetWarnings(), "version %s at/above minimum must pass silently", tc.version)
		sess := svc.registry.AgentsUnsafe()[tc.agent]
		require.NotNil(t, sess)
		assert.Contains(t, sess.Functions, tc.fn)
	}
}

// 配置最低版本对解析失败的自报版本不生效（Below 恒 false），与高水位
// 门槛同样保守。
func TestSDKVersionFloor_ConfiguredMinimumUnparseablePasses(t *testing.T) {
	svc := newScopeTestService(t)
	svc.SetSDKVersionMinimums(map[string]string{"go": "0.3.0"})
	ctx := context.Background()

	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-min-unknown",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.list", Version: "1.0.0", Enabled: true},
		},
		Processes: []*agentv1.AgentProcess{
			{ServiceId: "svc-go", SdkLanguage: "go", SdkVersion: "unknown", FunctionIds: []string{"player.list"}, GameId: "game-1", Env: "dev"},
		},
	}, "")
	require.NoError(t, err)
	assert.Empty(t, resp.GetWarnings())
	sess := svc.registry.AgentsUnsafe()["agent-min-unknown"]
	require.NotNil(t, sess)
	assert.Contains(t, sess.Functions, "player.list")
}

// 未配置最低版本的语言不受配置门槛影响（仅走滑动高水位）；语言键大小写
// 不敏感（配置 "Go" 命中自报 "go"）。
func TestSDKVersionFloor_ConfiguredMinimumScopeAndCase(t *testing.T) {
	svc := newScopeTestService(t)
	svc.SetSDKVersionMinimums(map[string]string{"Go": "0.3.0"})
	ctx := context.Background()

	// python 未配置：0.0.1 也放行。
	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-py",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.list", Version: "1.0.0", Enabled: true},
		},
		Processes: []*agentv1.AgentProcess{
			{ServiceId: "svc-py", SdkLanguage: "python", SdkVersion: "0.0.1", FunctionIds: []string{"player.list"}, GameId: "game-1", Env: "dev"},
		},
	}, "")
	require.NoError(t, err)
	assert.Empty(t, resp.GetWarnings())
	assert.Contains(t, svc.registry.AgentsUnsafe()["agent-py"].Functions, "player.list")

	// go 自报 0.1.0：配置键 "Go" 归一后仍命中 → 拒。
	resp2, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-go",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.ban", Version: "1.0.0", Enabled: true},
		},
		Processes: []*agentv1.AgentProcess{
			{ServiceId: "svc-go", SdkLanguage: "go", SdkVersion: "0.1.0", FunctionIds: []string{"player.ban"}, GameId: "game-1", Env: "dev"},
		},
	}, "")
	require.NoError(t, err)
	assert.NotContains(t, svc.registry.AgentsUnsafe()["agent-go"].Functions, "player.ban")
	var found bool
	for _, w := range resp2.GetWarnings() {
		if strings.Contains(w, "below configured minimum") {
			found = true
		}
	}
	assert.True(t, found, "case-insensitive language key must match, got %v", resp2.GetWarnings())
}

// 低于配置最低版本的 provider 与放行 provider 交叉声明同一函数：函数
// 保留（拒绝不造成可用性缺口），但拒绝告警照常记录。
func TestSDKVersionFloor_ConfiguredMinimumCrossProvidedKept(t *testing.T) {
	svc := newScopeTestService(t)
	svc.SetSDKVersionMinimums(map[string]string{"go": "0.3.0"})
	ctx := context.Background()

	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-cross",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.list", Version: "1.0.0", Enabled: true},
		},
		Processes: []*agentv1.AgentProcess{
			{ServiceId: "svc-old", SdkLanguage: "go", SdkVersion: "0.1.0", FunctionIds: []string{"player.list"}, GameId: "game-1", Env: "dev"},
			{ServiceId: "svc-new", SdkLanguage: "go", SdkVersion: "0.3.0", FunctionIds: []string{"player.list"}, GameId: "game-1", Env: "dev"},
		},
	}, "")
	require.NoError(t, err)
	assert.Contains(t, svc.registry.AgentsUnsafe()["agent-cross"].Functions, "player.list",
		"cross-provided function must survive the rejection")

	got := svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{
		GameID: "game-1", Env: "dev", AgentID: "agent-cross", Code: registry.WarningCodeSDKVersionBelowMinimum,
	})
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "svc-old")
	var found bool
	for _, w := range resp.GetWarnings() {
		if strings.Contains(w, "below configured minimum") {
			found = true
		}
	}
	assert.True(t, found)
}

// 函数级最低函数版本（function_version_floors）：比的是描述符自身的
// version——player.ban 以 1.0.0 注册、门槛 2.0.0 时不物化（防契约回退），
// 同请求里无门槛的 player.list 照常注册；拒绝以
// function_version_below_minimum 告警 + response warnings 双通道传达。
func TestFunctionVersionFloor_RejectsOnlyFlooredFunction(t *testing.T) {
	svc := newScopeTestService(t)
	require.NoError(t, svc.registry.SetFunctionVersionFloor("game-1", "dev", "player.ban", "2.0.0", "admin"))
	ctx := context.Background()

	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-fnfloor",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.ban", Version: "1.0.0", Enabled: true},
			{Id: "player.list", Version: "1.0.0", Enabled: true},
		},
	}, "")
	require.NoError(t, err, "rejection must keep the connection (no register error)")

	sess := svc.registry.AgentsUnsafe()["agent-fnfloor"]
	require.NotNil(t, sess)
	assert.NotContains(t, sess.Functions, "player.ban", "old function version must not materialize")
	assert.Contains(t, sess.Functions, "player.list", "unfloored function must register")

	got := svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{
		GameID: "game-1", Env: "dev", AgentID: "agent-fnfloor", Code: registry.WarningCodeFunctionVersionBelowMinimum,
	})
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "player.ban")
	assert.Contains(t, got[0].Message, "version=1.0.0 below configured minimum 2.0.0")

	var found bool
	for _, w := range resp.GetWarnings() {
		if strings.Contains(w, "below configured minimum") && strings.Contains(w, "player.ban") {
			found = true
		}
	}
	assert.True(t, found, "response warnings must carry the rejection, got %v", resp.GetWarnings())
}

// 描述符版本等于/高于门槛：正常注册，无告警。
func TestFunctionVersionFloor_AllowsAtOrAbove(t *testing.T) {
	svc := newScopeTestService(t)
	require.NoError(t, svc.registry.SetFunctionVersionFloor("game-1", "dev", "player.ban", "1.0.0", "admin"))
	ctx := context.Background()

	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-fnok",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.ban", Version: "1.0.0", Enabled: true},
		},
	}, "")
	require.NoError(t, err)
	assert.Empty(t, resp.GetWarnings())
	assert.Contains(t, svc.registry.AgentsUnsafe()["agent-fnok"].Functions, "player.ban")
}

// 版本不可解析（空串/"unknown"）：根本到不了门槛——上游注册校验已按
// invalid_version 拒绝（"invalid semver and skipped"），不产生
// function_version_below_minimum 告警。即平台上的函数版本恒为可解析
// semver，门槛的 Below 判定始终有意义。
func TestFunctionVersionFloor_UnparseableRejectedUpstream(t *testing.T) {
	svc := newScopeTestService(t)
	require.NoError(t, svc.registry.SetFunctionVersionFloor("game-1", "dev", "player.ban", "2.0.0", "admin"))
	ctx := context.Background()

	for _, tc := range []struct{ agent, version string }{
		{"agent-fn-v-empty", ""},
		{"agent-fn-v-unknown", "unknown"},
	} {
		resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
			AgentId: tc.agent,
			GameId:  "game-1",
			Env:     "dev",
			Functions: []*agentv1.FunctionDescriptor{
				{Id: "player.ban", Version: tc.version, Enabled: true},
			},
		}, "")
		require.NoError(t, err)
		assert.NotContains(t, svc.registry.AgentsUnsafe()[tc.agent].Functions, "player.ban",
			"unparseable version %q is rejected by upstream validation", tc.version)

		floorWarnings := svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{
			GameID: "game-1", Env: "dev", AgentID: tc.agent, Code: registry.WarningCodeFunctionVersionBelowMinimum,
		})
		assert.Empty(t, floorWarnings, "floor must not fire on unparseable versions")

		var found bool
		for _, w := range resp.GetWarnings() {
			if strings.Contains(w, "invalid semver") {
				found = true
			}
		}
		assert.True(t, found, "upstream invalid_version warning expected, got %v", resp.GetWarnings())
	}
}

// 清空函数级门槛后，此前被拒的旧版本照常注册（设置可逆）。
func TestFunctionVersionFloor_ClearedFloorRegisters(t *testing.T) {
	svc := newScopeTestService(t)
	require.NoError(t, svc.registry.SetFunctionVersionFloor("game-1", "dev", "player.ban", "2.0.0", "admin"))
	require.NoError(t, svc.registry.DeleteFunctionVersionFloor("game-1", "dev", "player.ban"))
	ctx := context.Background()

	resp, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-fnclear",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.ban", Version: "1.0.0", Enabled: true},
		},
	}, "")
	require.NoError(t, err)
	assert.Empty(t, resp.GetWarnings())
	assert.Contains(t, svc.registry.AgentsUnsafe()["agent-fnclear"].Functions, "player.ban")
}

// E9 配套：拦截警告必须携带 FunctionID/Version（执行侧 dispatcher 按
// (game,env,function) 过滤警告做门槛归因）；达标版本注册即清该函数的
// 历史拦截警告（生命周期跟随注册行为），保证「警告存在 ⟹ 最近注册仍
// 被拦」，执行侧不会把已恢复函数误述成被门槛拦截。
func TestFunctionVersionFloor_WarningCarriesFunctionAndClearsOnCompliant(t *testing.T) {
	svc := newScopeTestService(t)
	require.NoError(t, svc.registry.SetFunctionVersionFloor("game-1", "dev", "player.ban", "2.0.0", "admin"))
	ctx := context.Background()

	_, err := svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-fnflag",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.ban", Version: "1.0.0", Enabled: true},
		},
	}, "")
	require.NoError(t, err)
	got := svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{
		GameID: "game-1", Env: "dev", AgentID: "agent-fnflag", Code: registry.WarningCodeFunctionVersionBelowMinimum,
	})
	require.Len(t, got, 1)
	assert.Equal(t, "player.ban", got[0].FunctionID, "警告必须按函数可过滤")
	assert.Equal(t, "1.0.0", got[0].Version, "警告必须携带被拦版本")

	// 升级后达标注册：警告清除，函数物化。
	_, err = svc.handleRegisterRequest(ctx, &agentv1.RegisterRequest{
		AgentId: "agent-fnflag",
		GameId:  "game-1",
		Env:     "dev",
		Functions: []*agentv1.FunctionDescriptor{
			{Id: "player.ban", Version: "2.1.0", Enabled: true},
		},
	}, "")
	require.NoError(t, err)
	assert.Contains(t, svc.registry.AgentsUnsafe()["agent-fnflag"].Functions, "player.ban")
	got = svc.registry.ListRegistrationWarnings(registry.RegistrationWarningFilter{
		GameID: "game-1", Env: "dev", FunctionID: "player.ban", Code: registry.WarningCodeFunctionVersionBelowMinimum,
	})
	assert.Empty(t, got, "达标注册必须清历史拦截警告")
}
