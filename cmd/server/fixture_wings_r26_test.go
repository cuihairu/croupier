// 覆盖率巡检第二十六轮（wt-api）：cmd/server dashboard_fixture.go 装配翼——
// 不做完整 boot 的确定性直测：纯 helper 矩阵（fixtureAddrWithPort /
// defaultFixtureBootstrapDir / fixtureSDKDir）+ StartDashboardFixture 前置
// 错误翼（五个 addr 解析失败、MkdirTemp 失败、空 BaseDir 的自有目录
// 分支与 cleanupOnError 删除链）+ start* 方法的手工构造 fixture 错误翼
// （addr 解析 / listen 被占 / MkdirAll 被占 / SDK cmd.Start 失败）+
// ensureUIScope nil 模型翼 + WaitReady/ready/Close/CleanupScope 的空态与
// 幂等翼 + fixture API 端点全方法臂（经真实 listener 真实 HTTP 请求，
// 含 audit 三态 404/500 解码失败/200）。
// 登记不可达（不造假用例、不删防御分支）：
//   - fixtureFreePort 的 net.Listen("tcp","127.0.0.1:0") 错误翼及其在
//     fixtureAddrWithPort 两分支的透传翼：环回随机端口绑定在测试进程内
//     无确定性失败注入面（fd 耗尽不可构造）；
//   - defaultFixtureBootstrapDir/fixtureSDKDir 候选循环内的 filepath.Abs
//     失败翼：进入循环前提是 Getwd 已成功，同进程内紧跟的 Abs（相对输入
//     走 Getwd）再失败的窗口只有并发删除 cwd，顺序用例无法稳定构造；
//   - startServer 的 NewTelemetryService 错误翼：第二十四轮六形态探针
//     已证伪（OTLP 客户端不预解析 endpoint，构造期恒 nil error）；
//   - startServer 内 ensureUIScope 错误透传翼（335）：boot 自带的
//     NewServiceContext 每次新建全量迁移的 DB，ensureUIScope 的深层错误
//     阶梯已由 TestEnsureUIScope_EscalationWings 直测收口，「在真实
//     boot 中途破坏其自有 DB」无注入缝；
//   - 三个 Serve goroutine 的非 ErrServerClosed 错误日志翼（http/agent/
//     provider/fixtureAPI 四处）：Close 走 Shutdown → ErrServerClosed 被
//     过滤，其余形态需存活 listener 突然故障，无确定性注入面；
//   - Game.SetEnvs 的 json.Marshal 错误翼：[]GameEnv 纯字符串/颜色字段，
//     Marshal 无失败路径；
//   - startSDKLocked 的 json.Marshal 错误翼：FixtureSDKFunction 全字符串
//     字段，Marshal 无失败路径；
//   - WaitReady 的 60s 超时翼：硬编码 deadline，等待成本与价值不成比例。
package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/dbtype"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/server"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/cuihairu/croupier/internal/telemetry"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFixtureAddrWithPort_Matrix(t *testing.T) {
	// 空串 → 环回随机端口
	addr, err := fixtureAddrWithPort("")
	require.NoError(t, err)
	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", host)
	assert.NotEqual(t, "0", port)

	// 前后空白被裁剪后仍走 ":0" 分支 → 环回随机端口
	addr2, err := fixtureAddrWithPort("  :0  ")
	require.NoError(t, err)
	_, port2, err := net.SplitHostPort(addr2)
	require.NoError(t, err)
	assert.NotEqual(t, "0", port2)

	// 0.0.0.0 归一到环回、显式端口保留
	got, err := fixtureAddrWithPort("0.0.0.0:1234")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:1234", got)

	// 命名主机保留（不归一）
	got, err = fixtureAddrWithPort("localhost:1234")
	require.NoError(t, err)
	assert.Equal(t, "localhost:1234", got)

	// 非数字端口在 addr 层放行（SplitHostPort 只要冒号）——契约行为，
	// 该形态由 startServer 的 Sscanf 翼拦截
	got, err = fixtureAddrWithPort("127.0.0.1:notaport")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:notaport", got)

	// 无冒号 → SplitHostPort 报错
	_, err = fixtureAddrWithPort("no-port")
	require.Error(t, err)
}

// defaultFixtureBootstrapDir：候选缺失回落 "configs"（206）与 Getwd
// 失败回落（191，chdir 进已删除目录）。全程手动 chdir（不用 t.Chdir：
// 其 cleanup 会存下已删除的 doomed 路径），统一在 t.Cleanup 恢复。
func TestDefaultFixtureBootstrapDir_Wings(t *testing.T) {
	home, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(home) })

	doomed := t.TempDir()
	require.NoError(t, os.Chdir(doomed))
	require.NoError(t, os.RemoveAll(doomed))
	assert.Equal(t, "configs", defaultFixtureBootstrapDir(), "Getwd 失败回落")

	// 恢复到无 configs/admins.json 的目录再测候选缺失回落
	require.NoError(t, os.Chdir(t.TempDir()))
	assert.Equal(t, "configs", defaultFixtureBootstrapDir(), "无候选回落")
}

// fixtureSDKDir：Getwd 失败翼（511）与模块缺失错误翼（497）。
func TestFixtureSDKDir_Wings(t *testing.T) {
	home, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(home) })

	doomed := t.TempDir()
	require.NoError(t, os.Chdir(doomed))
	require.NoError(t, os.RemoveAll(doomed))
	_, err = fixtureSDKDir()
	require.Error(t, err, "Getwd 失败须报错")

	require.NoError(t, os.Chdir(t.TempDir()))
	_, err = fixtureSDKDir()
	require.ErrorContains(t, err, "sdks/go module not found")
}

// StartDashboardFixture 的五个 addr 解析错误翼（234-247）——坏 addr 在
// startServer 之前拦截，无需任何 boot；空 BaseDir 变体同时覆盖 MkdirTemp
// 分支（225）、ownsBaseDir=true（229）与 cleanupOnError 的 RemoveAll 链
// （255-258）；TMPDIR 指向文件覆盖 MkdirTemp 错误翼（226-228）。
func TestStartDashboardFixture_AddrWings(t *testing.T) {
	cases := []struct {
		name string
		opts DashboardFixtureOptions
	}{
		{"http", DashboardFixtureOptions{BaseDir: t.TempDir(), HTTPAddr: "no-port"}},
		{"control", DashboardFixtureOptions{BaseDir: t.TempDir(), ControlAddr: "no-port"}},
		{"agent-local", DashboardFixtureOptions{BaseDir: t.TempDir(), AgentLocalAddr: "no-port"}},
		{"provider", DashboardFixtureOptions{BaseDir: t.TempDir(), ProviderAddr: "no-port"}},
		{"fixture-api", DashboardFixtureOptions{BaseDir: t.TempDir(), FixtureAddr: "no-port"}},
		{"empty-basedir-mkdirtmp", DashboardFixtureOptions{HTTPAddr: "no-port"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := StartDashboardFixture(context.Background(), tc.opts)
			require.Error(t, err)
			assert.Nil(t, f)
		})
	}

	t.Run("mkdirtmp-error", func(t *testing.T) {
		occupied := filepath.Join(t.TempDir(), "occupied-file")
		require.NoError(t, os.WriteFile(occupied, nil, 0o600))
		t.Setenv("TMPDIR", occupied)
		f, err := StartDashboardFixture(context.Background(), DashboardFixtureOptions{})
		require.Error(t, err)
		assert.Nil(t, f)
	})
}

// startServer 的两处 addr 解析翼（293 SplitHostPort / 298 Sscanf）——
// 手工构造 fixture 直调，错误在 NewServiceContext 之前返回，零 boot 成本。
func TestStartServer_AddrParseWings(t *testing.T) {
	ctx := context.Background()

	f := &DashboardFixture{BaseDir: t.TempDir(), HTTPAddr: "no-port"}
	require.Error(t, f.startServer(ctx, DashboardFixtureOptions{BootstrapDir: t.TempDir()}))

	f2 := &DashboardFixture{BaseDir: t.TempDir(), HTTPAddr: "127.0.0.1:notaport"}
	// Sscanf 报 "expected integer"，不回显输入，断言错误形态即可
	require.ErrorContains(t, f2.startServer(ctx, DashboardFixtureOptions{BootstrapDir: t.TempDir()}), "expected integer")
}

// startProvider / startAgent / startFixtureAPI 的监听与建目录错误翼——
// 手工构造 fixture，各自单点注入。
func TestStartComponents_ErrorWings(t *testing.T) {
	ctx := context.Background()

	t.Run("provider-listen-occupied", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = ln.Close() }()
		f := &DashboardFixture{ProviderAddr: ln.Addr().String()}
		require.ErrorContains(t, f.startProvider(), "listen")
	})

	t.Run("agent-mkdir-occupied", func(t *testing.T) {
		base := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(base, "agent"), nil, 0o600))
		f := &DashboardFixture{BaseDir: base}
		require.Error(t, f.startAgent(ctx))
	})

	t.Run("fixture-api-listen-occupied", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = ln.Close() }()
		f := &DashboardFixture{FixtureAddr: ln.Addr().String()}
		require.ErrorContains(t, f.startFixtureAPI(), "listen")
	})

	// CROUPIER_E2E_PROVIDER_BIN 指向目录：exec.Start 必失败（558-559）
	t.Run("sdk-cmd-start-fail", func(t *testing.T) {
		t.Setenv("CROUPIER_E2E_PROVIDER_BIN", t.TempDir())
		f := &DashboardFixture{
			BaseDir:        t.TempDir(),
			AgentLocalAddr: "127.0.0.1:19099",
			FixtureAddr:    "127.0.0.1:19098",
			GameID:         "g", Env: "e",
		}
		require.ErrorContains(t, f.startSDK(ctx, DefaultFixtureSDKFunctions()), "start e2eprovider")
	})
}

// ensureUIScope 的 nil 模型守卫翼。
func TestEnsureUIScope_NilModelsWing(t *testing.T) {
	f := &DashboardFixture{}
	require.ErrorContains(t, f.ensureUIScope(context.Background()), "fixture scope models are unavailable")
}

// WaitReady 的 ctx 取消翼 + ready 的 nil svcCtx 翼 + Close 的空态/幂等翼
// + CleanupScope 的 nil DB 早退翼。
func TestFixture_LifecycleCheapWings(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, (&DashboardFixture{}).WaitReady(ctx), context.Canceled)

	assert.False(t, (&DashboardFixture{}).ready(), "nil svcCtx 不得判就绪")

	f := &DashboardFixture{}
	require.NoError(t, f.Close(context.Background()))
	require.NoError(t, f.Close(context.Background()), "Close 幂等")

	require.NoError(t, (&DashboardFixture{}).CleanupScope(context.Background()), "nil DB 早退 nil")
}

// ensureUIScope 的底层错误透传翼（335-336）：手工装配 svcCtx（games 表
// 缺失）→ FindByGameIDString 报错 → Create 兜底也报错 → 包装上抛。
func TestEnsureUIScope_QueryErrorWing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:r26scope?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	f := &DashboardFixture{
		GameID: "r26-g",
		Env:    "r26-e",
		svcCtx: &svc.ServiceContext{
			GameModel:  model.NewGameModel(db),
			AdminModel: model.NewAdminModel(db),
		},
	}
	require.ErrorContains(t, f.ensureUIScope(context.Background()), "fixture game")
}

// fixture API 端点全方法臂：真实 listener + 真实 HTTP 请求驱动 mux 的
// 每个 handler 分支（health / sdk/functions 三态 / sdk/calls 五臂 /
// audit 三态 / provider 两端点）。
func TestFixtureAPI_Endpoints(t *testing.T) {
	port, err := fixtureFreePort()
	require.NoError(t, err)
	base := "http://127.0.0.1:" + strconv.Itoa(port)

	db, err := gorm.Open(sqlite.Open("file:r26fixture?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&audit.AuditModel{}))

	f := &DashboardFixture{
		GameID:       "r26-g",
		Env:          "r26-e",
		HTTPAddr:     "127.0.0.1:19095",
		ProviderAddr: "127.0.0.1:19094",
		FixtureAddr:  "127.0.0.1:" + strconv.Itoa(port),
		provider:     newPlayersProvider(),
		svcCtx:       &svc.ServiceContext{DB: db},
	}
	require.NoError(t, f.startFixtureAPI())
	t.Cleanup(func() { _ = f.Close(context.Background()) })

	get := func(path string) (int, string) {
		resp, err := http.Get(base + path)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		raw, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(raw)
	}
	do := func(method, path, body string) (int, string) {
		req, err := http.NewRequest(method, base+path, strings.NewReader(body))
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		raw, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(raw)
	}

	t.Run("health", func(t *testing.T) {
		code, body := get("/__fixture__/health")
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, body, `"status":"ok"`)
		assert.Contains(t, body, `"agentConnected":false`)
	})

	t.Run("sdk-functions", func(t *testing.T) {
		code, _ := do(http.MethodGet, "/__fixture__/sdk/functions", "")
		require.Equal(t, http.StatusMethodNotAllowed, code)

		code, body := do(http.MethodPost, "/__fixture__/sdk/functions", "{not json")
		require.Equal(t, http.StatusBadRequest, code)
		assert.Contains(t, body, "invalid_json")

		// 合法载荷 + 替换失败（BIN 指向目录）→ 500
		t.Setenv("CROUPIER_E2E_PROVIDER_BIN", t.TempDir())
		code, body = do(http.MethodPost, "/__fixture__/sdk/functions", `{"functions":[]}`)
		require.Equal(t, http.StatusInternalServerError, code)
		assert.Contains(t, body, "sdk_replace_failed")
	})

	t.Run("sdk-calls", func(t *testing.T) {
		code, body := get("/__fixture__/sdk/calls")
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, body, `"calls":[]`)

		code, _ = do(http.MethodPost, "/__fixture__/sdk/calls", "{bad")
		require.Equal(t, http.StatusBadRequest, code)

		code, _ = do(http.MethodPost, "/__fixture__/sdk/calls", `{"functionId":"mail.send"}`)
		require.Equal(t, http.StatusOK, code)
		code, body = get("/__fixture__/sdk/calls")
		assert.Contains(t, body, "mail.send", "POST 后调用须可查询")

		code, _ = do(http.MethodDelete, "/__fixture__/sdk/calls", "")
		require.Equal(t, http.StatusOK, code)
		code, body = get("/__fixture__/sdk/calls")
		assert.Contains(t, body, `"calls":[]`, "DELETE 后清空")

		code, _ = do(http.MethodPatch, "/__fixture__/sdk/calls", "")
		require.Equal(t, http.StatusMethodNotAllowed, code)
	})

	t.Run("audit-page-execute", func(t *testing.T) {
		// 空表 → 404
		code, body := get("/__fixture__/audit/page-execute")
		require.Equal(t, http.StatusNotFound, code)
		assert.Contains(t, body, "page_execute_audit_not_found")

		// 垃圾 ActorJSON → ToRecord 解码失败 → 500
		bad := &audit.AuditModel{
			AuditID: "r26-bad", Timestamp: time.Now(),
			EventType: string(audit.EventPageExecute), Category: "page",
			Severity: "info", Outcome: "success",
			ActorJSON: dbtype.JSON("not-json"),
			ChainHash: "h", ChainSequence: 1, CreatedAt: time.Now(),
		}
		require.NoError(t, db.Create(bad).Error)
		code, body = get("/__fixture__/audit/page-execute")
		require.Equal(t, http.StatusInternalServerError, code)
		assert.Contains(t, body, "audit_decode_failed")

		// 合法行（JSON 列全空）→ 200
		require.NoError(t, db.Unscoped().Where("audit_id = ?", "r26-bad").Delete(&audit.AuditModel{}).Error)
		good := &audit.AuditModel{
			AuditID: "r26-good", Timestamp: time.Now(),
			EventType: string(audit.EventPageExecute), Category: "page",
			Severity: "info", Action: "exec", Outcome: "success",
			ChainHash: "h2", ChainSequence: 2, CreatedAt: time.Now(),
		}
		require.NoError(t, db.Create(good).Error)
		code, body = get("/__fixture__/audit/page-execute")
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, body, `"eventType":"`+string(audit.EventPageExecute)+`"`)
	})

	t.Run("provider-endpoints", func(t *testing.T) {
		code, body := get("/__fixture__/provider/calls")
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, body, `"calls":`)

		code, _ = do(http.MethodGet, "/__fixture__/provider/reset", "")
		require.Equal(t, http.StatusMethodNotAllowed, code)
		code, _ = do(http.MethodPost, "/__fixture__/provider/reset", "")
		require.Equal(t, http.StatusOK, code)
	})
}

// occupyTCP 占住一个环回端口，返回其 addr（监听期贯穿整个用例）。
func occupyTCP(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String()
}

// StartDashboardFixture 的五步失败翼（262-283）：① 廉价——addr 过
// fixtureAddrWithPort 但端口非数字 → startServer Sscanf 即败（在
// NewServiceContext 之前返回），cleanupOnError 首次真实执行（rootCancel +
// 自有 BaseDir RemoveAll）；②-⑤ 各自要求 startServer 真实全量启动后在
// 目标步骤失败（provider 端口被占 / agent 目录被占 / SDK cmd.Start 失败 /
// fixture API 端口被占）。
func TestStartDashboardFixture_StepFailureWings(t *testing.T) {
	t.Run("server-addr-sscanf", func(t *testing.T) {
		f, err := StartDashboardFixture(context.Background(), DashboardFixtureOptions{HTTPAddr: "127.0.0.1:notaport"})
		require.ErrorContains(t, err, "start fixture server")
		assert.Nil(t, f)
	})

	t.Run("provider-listen-occupied", func(t *testing.T) {
		f, err := StartDashboardFixture(context.Background(), DashboardFixtureOptions{
			BaseDir:      t.TempDir(),
			ProviderAddr: occupyTCP(t),
		})
		require.ErrorContains(t, err, "start players provider")
		assert.Nil(t, f)
	})

	t.Run("agent-dir-occupied", func(t *testing.T) {
		base := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(base, "agent"), nil, 0o600))
		f, err := StartDashboardFixture(context.Background(), DashboardFixtureOptions{BaseDir: base})
		require.ErrorContains(t, err, "start fixture agent")
		assert.Nil(t, f)
	})

	t.Run("sdk-cmd-start-fail", func(t *testing.T) {
		t.Setenv("CROUPIER_E2E_PROVIDER_BIN", t.TempDir())
		f, err := StartDashboardFixture(context.Background(), DashboardFixtureOptions{BaseDir: t.TempDir()})
		require.ErrorContains(t, err, "start fixture sdk")
		assert.Nil(t, f)
	})

	t.Run("fixture-api-listen-occupied", func(t *testing.T) {
		f, err := StartDashboardFixture(context.Background(), DashboardFixtureOptions{
			BaseDir:     t.TempDir(),
			FixtureAddr: occupyTCP(t),
		})
		require.ErrorContains(t, err, "start fixture api")
		assert.Nil(t, f)
	})

	// startServer 的 listen 错误翼（362）：HTTPAddr 显式占住端口——
	// fixtureAddrWithPort 放行、Sscanf 过、NewServiceContext/ensureUIScope/
	// env 读取全走完后在 listen 处失败；同 boot 顺带覆盖
	// CROUPIER_E2E_PUBLISH_REVIEW 环境覆盖翼（321）。
	t.Run("server-listen-occupied", func(t *testing.T) {
		t.Setenv("CROUPIER_E2E_PUBLISH_REVIEW", "auto")
		f, err := StartDashboardFixture(context.Background(), DashboardFixtureOptions{
			BaseDir:  t.TempDir(),
			HTTPAddr: occupyTCP(t),
		})
		require.ErrorContains(t, err, "listen")
		assert.Nil(t, f)
	})
}

// startAgent 的 providers.yaml 写失败翼（448-449）：agent 目录存在但
// providers.yaml 被目录占位 → WriteFile 报错。
func TestStartAgent_ProvidersYAMLOccupiedWing(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(base, "agent", "providers.yaml"), 0o755))
	f := &DashboardFixture{BaseDir: base}
	require.Error(t, f.startAgent(context.Background()))
}

// startSDKLocked 的 fixtureSDKDir 失败翼（541-542）与 go build 失败翼
// （550-551）。
func TestStartSDK_SearchAndBuildWings(t *testing.T) {
	ctx := context.Background()

	t.Run("sdk-dir-missing", func(t *testing.T) {
		home, err := os.Getwd()
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.Chdir(home) })
		require.NoError(t, os.Chdir(t.TempDir())) // 无 sdks/go 候选
		f := &DashboardFixture{
			BaseDir:        t.TempDir(),
			AgentLocalAddr: "127.0.0.1:19093",
			FixtureAddr:    "127.0.0.1:19092",
			GameID:         "g", Env: "e",
		}
		require.ErrorContains(t, f.startSDK(ctx, DefaultFixtureSDKFunctions()), "sdks/go module not found")
	})

	t.Run("build-fail", func(t *testing.T) {
		// GOOS 非法值使 go build 子进程必败（sdkDir 正常解析、bin 未预置）
		t.Setenv("GOOS", "invalidos")
		f := &DashboardFixture{
			BaseDir:        t.TempDir(),
			AgentLocalAddr: "127.0.0.1:19091",
			FixtureAddr:    "127.0.0.1:19090",
			GameID:         "g", Env: "e",
		}
		require.ErrorContains(t, f.startSDK(ctx, DefaultFixtureSDKFunctions()), "build e2eprovider")
	})
}

// ensureUIScope 的深层错误翼阶梯：FindEnvBinding 缺表 / AddEnvBinding 被
// 触发器拒写 / FindByUsername 缺表 / UpdateLastScope 被触发器拒写。
func TestEnsureUIScope_EscalationWings(t *testing.T) {
	ctx := context.Background()
	newDB := func(name string) *gorm.DB {
		db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		return db
	}
	newFixture := func(db *gorm.DB, gameID string) *DashboardFixture {
		return &DashboardFixture{
			GameID: gameID,
			Env:    "r26-e",
			svcCtx: &svc.ServiceContext{
				GameModel:  model.NewGameModel(db),
				AdminModel: model.NewAdminModel(db),
			},
		}
	}

	t.Run("env-binding-table-missing", func(t *testing.T) {
		db := newDB("r26_esc1")
		require.NoError(t, db.AutoMigrate(&model.Game{}))
		require.ErrorContains(t, newFixture(db, "esc1").ensureUIScope(ctx), "find fixture environment binding")
	})

	t.Run("env-binding-write-refused", func(t *testing.T) {
		db := newDB("r26_esc2")
		require.NoError(t, db.AutoMigrate(&model.Game{}, &model.GameEnvBinding{}))
		require.NoError(t, db.Exec("CREATE TRIGGER block_env_insert BEFORE INSERT ON game_envs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)
		// Router 非 nil 翼（412）：databaseName 改走 Router.NameForGame
		f := newFixture(db, "esc2")
		f.svcCtx.Router = router.New(router.Config{}, db)
		require.ErrorContains(t, f.ensureUIScope(ctx), "create fixture environment binding")
	})

	t.Run("admins-table-missing", func(t *testing.T) {
		db := newDB("r26_esc3")
		require.NoError(t, db.AutoMigrate(&model.Game{}, &model.GameEnvBinding{}))
		require.ErrorContains(t, newFixture(db, "esc3").ensureUIScope(ctx), "find fixture admin")
	})

	t.Run("update-last-scope-refused", func(t *testing.T) {
		db := newDB("r26_esc4")
		require.NoError(t, db.AutoMigrate(&model.Game{}, &model.GameEnvBinding{}, &model.Admin{}))
		require.NoError(t, db.Create(&model.Admin{Username: "admin"}).Error)
		require.NoError(t, db.Exec("CREATE TRIGGER block_admin_update BEFORE UPDATE ON admins BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)
		require.ErrorContains(t, newFixture(db, "esc4").ensureUIScope(ctx), "select fixture scope for admin")
	})
}

// fixture API 的 health agent 在线分支（880-881）与 ready 的深层翼：
// 经 UpsertAgent 注入带函数契约的会话后，health 报 agentConnected=true；
// ready 走完函数核对后在 DB 为 nil 处判未就绪；空 store 的 ready 直接
// 走 agent 缺席翼。
func TestFixtureAPI_HealthAndReady_WithAgent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:r26agent?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, registry.MigrateAgentSessions(db))
	store := registry.NewStoreWithDB(db)

	fns := DefaultFixtureSDKFunctions()
	fnMap := map[string]registry.FunctionMeta{}
	for _, fn := range fns {
		fnMap[fn.ID] = registry.FunctionMeta{}
	}
	require.NoError(t, store.UpsertAgent(&registry.AgentSession{
		AgentID:   "real-dashboard-agent",
		GameID:    "r26-g",
		Env:       "r26-e",
		Functions: fnMap,
		ExpireAt:  time.Now().Add(time.Minute),
		LastSeen:  time.Now(),
	}))

	port, err := fixtureFreePort()
	require.NoError(t, err)
	f := &DashboardFixture{
		GameID:       "r26-g",
		Env:          "r26-e",
		HTTPAddr:     "127.0.0.1:19089",
		ProviderAddr: "127.0.0.1:19088",
		FixtureAddr:  "127.0.0.1:" + strconv.Itoa(port),
		provider:     newPlayersProvider(),
		svcCtx:       &svc.ServiceContext{RegistryStore: store},
		sdkFns:       fns,
	}
	require.NoError(t, f.startFixtureAPI())
	t.Cleanup(func() { _ = f.Close(context.Background()) })

	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/__fixture__/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(raw), `"agentConnected":true`)
	assert.Contains(t, string(raw), "mail.send")

	assert.False(t, f.ready(), "函数齐备但 DB 为 nil → 仍未就绪")

	emptyDB, err := gorm.Open(sqlite.Open("file:r26empty?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, registry.MigrateAgentSessions(emptyDB))
	empty := &DashboardFixture{
		svcCtx: &svc.ServiceContext{RegistryStore: registry.NewStoreWithDB(emptyDB)},
		sdkFns: fns,
		GameID: "r26-g",
		Env:    "r26-e",
	}
	assert.False(t, empty.ready(), "无 agent 会话 → 未就绪")

	// 函数缺失翼（668）：agent 会话在但少一个 sdkFn → 核对 break 判未就绪
	partialDB, err := gorm.Open(sqlite.Open("file:r26partial?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, registry.MigrateAgentSessions(partialDB))
	partialMap := map[string]registry.FunctionMeta{}
	for _, fn := range fns[1:] { // 跳过 fns[0]，Functions 缺该函数
		partialMap[fn.ID] = registry.FunctionMeta{}
	}
	partialStore := registry.NewStoreWithDB(partialDB)
	require.NoError(t, partialStore.UpsertAgent(&registry.AgentSession{
		AgentID:   "partial-dashboard-agent",
		GameID:    "r26-g",
		Env:       "r26-e",
		Functions: partialMap,
		ExpireAt:  time.Now().Add(time.Minute),
		LastSeen:  time.Now(),
	}))
	partial := &DashboardFixture{
		svcCtx: &svc.ServiceContext{RegistryStore: partialStore, DB: partialDB},
		sdkFns: fns,
		GameID: "r26-g",
		Env:    "r26-e",
	}
	assert.False(t, partial.ready(), "agent 函数缺一 → 未就绪")

	// 契约计数翼（668）：agent 函数齐备 + DB 非 nil 但 function_contracts
	// 缺表 → 契约核对报错判未就绪（须在函数核对通过之后才触达）
	missingDB, err := gorm.Open(sqlite.Open("file:r26nocontract?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, registry.MigrateAgentSessions(missingDB))
	fullStore := registry.NewStoreWithDB(missingDB)
	require.NoError(t, fullStore.UpsertAgent(&registry.AgentSession{
		AgentID:   "real-dashboard-agent",
		GameID:    "r26-g",
		Env:       "r26-e",
		Functions: fnMap,
		ExpireAt:  time.Now().Add(time.Minute),
		LastSeen:  time.Now(),
	}))
	contractsMissing := &DashboardFixture{
		svcCtx: &svc.ServiceContext{RegistryStore: fullStore, DB: missingDB},
		sdkFns: fns,
		GameID: "r26-g",
		Env:    "r26-e",
	}
	assert.False(t, contractsMissing.ready(), "契约缺表 → 未就绪")
}

// fixture API 的 audit 查询失败翼（880-881）：DB 非 nil 但 audit_records
// 缺表 → 500 audit_query_failed。
func TestFixtureAPI_AuditQueryFailedWing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:r26noaudit?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	port, err := fixtureFreePort()
	require.NoError(t, err)
	f := &DashboardFixture{
		GameID:       "r26-g",
		Env:          "r26-e",
		HTTPAddr:     "127.0.0.1:19087",
		ProviderAddr: "127.0.0.1:19086",
		FixtureAddr:  "127.0.0.1:" + strconv.Itoa(port),
		provider:     newPlayersProvider(),
		svcCtx:       &svc.ServiceContext{DB: db},
	}
	require.NoError(t, f.startFixtureAPI())
	t.Cleanup(func() { _ = f.Close(context.Background()) })

	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/__fixture__/audit/page-execute")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, string(raw), "audit_query_failed")
}

// Close 的运行时句柄分支（754-793）：control 监听 + 双 HTTP 服务 +
// telemetry + 自有 BaseDir 删除链全走完。
func TestClose_PopulatedRuntimeWings(t *testing.T) {
	svcCtxCtl, _ := newControlServerSvcCtx(t)
	c := &config.Config{}
	c.Control.Addr = "127.0.0.1:0"
	ctx, cancel := context.WithCancel(context.Background())
	rt := startControlServer(ctx, c, svcCtxCtl, server.NewAgentSessionStore())

	telSvc, err := telemetry.NewGameTelemetryService(telemetry.TelemetryConfig{}, slog.Default())
	require.NoError(t, err)

	base := t.TempDir()
	f := &DashboardFixture{
		control:    rt,
		httpSrv:    &http.Server{},
		fixtureSrv: &http.Server{},
		rootCancel: cancel,
		svcCtx: &svc.ServiceContext{
			Telemetry: telSvc,
			// Router 非 nil 翼（781）：Close 尾部走 Router.Close（空缓存，
			// 恒 nil error）
			Router: router.New(router.Config{}, nil),
		},
		BaseDir:     base,
		ownsBaseDir: true,
	}
	require.NoError(t, f.Close(context.Background()))

	_, statErr := os.Stat(base)
	assert.True(t, os.IsNotExist(statErr), "自有 BaseDir 须随 Close 删除")
}

// CleanupScope 的错误翼阶梯（689-735）：六处 return 逐级注入——前两处用
// 缺表（子查询报错），第三处用 scoped 循环内缺 PageSpec 表，后三处用
// BEFORE UPDATE/DELETE 触发器在「前面全过、目标步骤被拒」处精确引爆。
// 每臂独立内存库，触发器互不干扰。
func TestCleanupScope_EscalationWings(t *testing.T) {
	ctx := context.Background()
	newDB := func(name string) *gorm.DB {
		db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		return db
	}
	migrateAll := func(db *gorm.DB, skipPageSpec bool) {
		tables := []interface{}{
			&model.CapabilitySemantics{}, &model.CapabilitySemanticVersion{},
			&model.PageProposal{}, &model.PageProposalVersion{},
			&model.FunctionContract{}, &model.ResourceCapability{},
			&model.BlockedProposalIssue{},
			&model.PublishedPageSpec{}, &model.PageVersion{},
			&model.OpenAPISource{}, &model.OpenAPISourceBinding{},
			&model.Game{}, &model.GameEnvBinding{}, &model.Admin{},
			&registry.AgentSessionDB{}, &registry.AgentRegistrationOperationDB{},
		}
		if !skipPageSpec {
			tables = append(tables, &model.PageSpec{})
		}
		require.NoError(t, db.AutoMigrate(tables...))
	}

	t.Run("semantic-versions-missing", func(t *testing.T) {
		db := newDB("r26_cs1")
		f := &DashboardFixture{GameID: "cs1", Env: "e", svcCtx: &svc.ServiceContext{DB: db}}
		require.ErrorContains(t, f.CleanupScope(ctx), "cleanup capability semantic versions")
	})

	t.Run("proposal-versions-missing", func(t *testing.T) {
		db := newDB("r26_cs2")
		require.NoError(t, db.AutoMigrate(&model.CapabilitySemantics{}, &model.CapabilitySemanticVersion{}))
		f := &DashboardFixture{GameID: "cs2", Env: "e", svcCtx: &svc.ServiceContext{DB: db}}
		require.ErrorContains(t, f.CleanupScope(ctx), "cleanup page proposal versions")
	})

	t.Run("scoped-pagespec-missing", func(t *testing.T) {
		db := newDB("r26_cs3")
		migrateAll(db, true)
		f := &DashboardFixture{GameID: "cs3", Env: "e", svcCtx: &svc.ServiceContext{DB: db}}
		require.ErrorContains(t, f.CleanupScope(ctx), "cleanup *model.PageSpec")
	})

	t.Run("admin-scope-restore-refused", func(t *testing.T) {
		db := newDB("r26_cs4")
		migrateAll(db, false)
		admin := &model.Admin{Username: "cs4-admin"}
		require.NoError(t, db.Create(admin).Error)
		// UPDATE 须命中真实行——零行更新不报错，触发器无从引爆
		require.NoError(t, db.Exec("CREATE TRIGGER block_admin_upd BEFORE UPDATE ON admins BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)
		f := &DashboardFixture{
			GameID: "cs4", Env: "e",
			svcCtx:                 &svc.ServiceContext{DB: db, AdminModel: model.NewAdminModel(db)},
			scopeAdminScopeUpdated: true,
			scopeAdminID:           admin.ID,
			scopeAdminPrevious:     model.LastScope{GameID: "old", Env: "old"},
		}
		require.ErrorContains(t, f.CleanupScope(ctx), "restore fixture admin scope")
	})

	t.Run("env-binding-removal-refused", func(t *testing.T) {
		db := newDB("r26_cs5")
		migrateAll(db, false)
		// DELETE 须命中真实绑定行
		require.NoError(t, db.Create(&model.GameEnvBinding{GameID: "cs5", Env: "e", DatabaseName: "cs5db"}).Error)
		require.NoError(t, db.Exec("CREATE TRIGGER block_env_del BEFORE DELETE ON game_envs BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)
		f := &DashboardFixture{
			GameID: "cs5", Env: "e",
			svcCtx:              &svc.ServiceContext{DB: db, GameModel: model.NewGameModel(db)},
			scopeBindingCreated: true,
		}
		require.ErrorContains(t, f.CleanupScope(ctx), "cleanup fixture environment binding")
	})

	t.Run("game-delete-refused", func(t *testing.T) {
		db := newDB("r26_cs6")
		migrateAll(db, false)
		// DELETE 须命中真实游戏行
		require.NoError(t, db.Create(&model.Game{GameID: "cs6", Name: "cs6"}).Error)
		require.NoError(t, db.Exec("CREATE TRIGGER block_game_del BEFORE DELETE ON games BEGIN SELECT RAISE(ABORT, 'blocked'); END").Error)
		f := &DashboardFixture{
			GameID: "cs6", Env: "e",
			svcCtx:           &svc.ServiceContext{DB: db},
			scopeGameCreated: true,
		}
		require.ErrorContains(t, f.CleanupScope(ctx), "cleanup fixture game")
	})
}
