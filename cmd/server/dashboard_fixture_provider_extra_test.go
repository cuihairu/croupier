package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// doJSON 向 handler 发请求并返回状态码与解码后的响应体。
func doJSON(t *testing.T, h http.Handler, method, path, body string) (int, map[string]any) {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	out := map[string]any{}
	if w.Body.Len() > 0 {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w.Code, out
}

// playersProvider handler() 的 REST 全语义：list/get/create/update/delete/kick
// 加各自 404、create/update 的 400、方法不允许与 openapi.json。
func TestPlayersProviderHandlerCRUD(t *testing.T) {
	h := newPlayersProvider().handler()

	// openapi.json 可拉取且是合法 OpenAPI 文档
	code, doc := doJSON(t, h, http.MethodGet, "/openapi.json", "")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "3.0.3", doc["openapi"])

	// 列表默认分页
	code, list := doJSON(t, h, http.MethodGet, "/players", "")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(2), list["total"])
	items := list["items"].([]any)
	require.Len(t, items, 2)
	assert.Equal(t, "Ada", items[0].(map[string]any)["name"])

	// 分页参数越界收敛
	code, list = doJSON(t, h, http.MethodGet, "/players?page=99&page_size=1", "")
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, list["items"].([]any))
	code, list = doJSON(t, h, http.MethodGet, "/players?page=0&page_size=-1", "")
	require.Equal(t, http.StatusOK, code)
	require.Len(t, list["items"].([]any), 2)

	// 详情：命中与未命中
	code, one := doJSON(t, h, http.MethodGet, "/players/p-001", "")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "p-001", one["id"])
	code, _ = doJSON(t, h, http.MethodGet, "/players/none", "")
	assert.Equal(t, http.StatusNotFound, code)

	// 创建：成功 / 缺 name / 非法 JSON
	code, created := doJSON(t, h, http.MethodPost, "/players", `{"name":"Cid","level":30}`)
	require.Equal(t, http.StatusCreated, code)
	assert.Equal(t, "p-003", created["id"])
	code, _ = doJSON(t, h, http.MethodPost, "/players", `{"name":"  "}`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = doJSON(t, h, http.MethodPost, "/players", `{bad`)
	assert.Equal(t, http.StatusBadRequest, code)

	// 更新：部分字段 / 未命中 / 非法 JSON
	code, updated := doJSON(t, h, http.MethodPut, "/players/p-001", `{"level":11}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(11), updated["level"])
	assert.Equal(t, "Ada", updated["name"])
	code, _ = doJSON(t, h, http.MethodPut, "/players/none", `{}`)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = doJSON(t, h, http.MethodPut, "/players/p-001", `{bad`)
	assert.Equal(t, http.StatusBadRequest, code)

	// 删除：成功（204 无体）/ 未命中
	code, _ = doJSON(t, h, http.MethodDelete, "/players/p-003", "")
	require.Equal(t, http.StatusNoContent, code)
	code, list = doJSON(t, h, http.MethodGet, "/players", "")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(2), list["total"])
	code, _ = doJSON(t, h, http.MethodDelete, "/players/p-003", "")
	assert.Equal(t, http.StatusNotFound, code)

	// 行动作 kick：命中与未命中
	code, kicked := doJSON(t, h, http.MethodPost, "/players/p-001/kick", `{"reason":"test"}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, kicked["success"])
	code, _ = doJSON(t, h, http.MethodPost, "/players/none/kick", `{}`)
	assert.Equal(t, http.StatusNotFound, code)

	// 方法不允许（集合级 + 条目级）
	code, _ = doJSON(t, h, http.MethodPut, "/players", "")
	assert.Equal(t, http.StatusMethodNotAllowed, code)
	code, _ = doJSON(t, h, http.MethodPatch, "/players/p-001", "")
	assert.Equal(t, http.StatusMethodNotAllowed, code)

	// reset 后回到种子状态、调用日志清空
	p := newPlayersProvider()
	doJSON(t, p.handler(), http.MethodGet, "/players", "")
	require.NotEmpty(t, p.calls())
	p.reset()
	assert.Empty(t, p.calls())
	code, list = doJSON(t, p.handler(), http.MethodGet, "/players", "")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(2), list["total"])
}

func TestReadBody(t *testing.T) {
	// 空 body → json decode EOF 报错
	raw, err := readBody(httptest.NewRequest(http.MethodPost, "/x", nil))
	assert.Error(t, err)
	assert.Nil(t, raw)

	raw, err = readBody(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"a":1}`)))
	require.NoError(t, err)
	assert.JSONEq(t, `{"a":1}`, string(raw))
}

func TestFixtureAddrWithPort(t *testing.T) {
	// 空 → 回环随机端口
	addr, err := fixtureAddrWithPort("")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(addr, "127.0.0.1:"), addr)

	// 端口 0 → 换成已取到的空闲端口
	addr, err = fixtureAddrWithPort("127.0.0.1:0")
	require.NoError(t, err)
	assert.False(t, strings.HasSuffix(addr, ":0"), addr)

	// 0.0.0.0 / 空 host 归一为回环
	addr, err = fixtureAddrWithPort("0.0.0.0:18999")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:18999", addr)
	addr, err = fixtureAddrWithPort(":18999")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:18999", addr)

	// 缺端口 → 解析错误
	_, err = fixtureAddrWithPort("no-port-here")
	assert.Error(t, err)

	// free port 原语本身可用
	port, err := fixtureFreePort()
	require.NoError(t, err)
	assert.NotZero(t, port)
}

func TestDefaultFixtureBootstrapDir(t *testing.T) {
	// 从 cmd/server 工作目录能定位到仓库根的 configs（含 admins.json）
	dir := defaultFixtureBootstrapDir()
	require.NotEmpty(t, dir)
	_, err := os.Stat(filepath.Join(dir, "admins.json"))
	assert.NoError(t, err)
}

// newFixtureAPIFixture 构造一个只带 provider/存储句柄的轻量 fixture，
// startFixtureAPI 的全部端点经真实监听回环验证。
func newFixtureAPIFixture(t *testing.T) *DashboardFixture {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(filepath.Join(t.TempDir(), "fx.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	require.NoError(t, reg.MigrateAgentSessions(db))
	// audit_records 不在 model.AutoMigrate 清单内（svc 启动时补建），端点查询需要它。
	require.NoError(t, db.AutoMigrate(&audit.AuditModel{}))

	store := reg.NewStoreWithDB(db)
	now := time.Now()
	require.NoError(t, store.UpsertAgent(&reg.AgentSession{
		AgentID:  "real-dashboard-agent",
		GameID:   "e2e-game",
		Env:      "e2e",
		ExpireAt: now.Add(time.Minute),
		LastSeen: now,
		Functions: map[string]reg.FunctionMeta{
			"mail.send": {Enabled: true},
		},
	}))

	fixtureAddr, err := fixtureAddrWithPort("") // 先解析具体端口（startFixtureAPI 不做端口回写）
	require.NoError(t, err)
	f := &DashboardFixture{
		GameID:      "e2e-game",
		Env:         "e2e",
		FixtureAddr: fixtureAddr,
		BaseDir:     t.TempDir(),
		svcCtx:      &svc.ServiceContext{DB: db, RegistryStore: store},
		provider:    newPlayersProvider(),
	}
	require.NoError(t, f.startFixtureAPI())
	t.Cleanup(func() { _ = f.Close(context.Background()) })
	return f
}

func TestStartFixtureAPI_Endpoints(t *testing.T) {
	f := newFixtureAPIFixture(t)
	base := "http://" + f.FixtureAddr

	get := func(path string) (int, map[string]any) {
		t.Helper()
		resp, err := http.Get(base + path)
		require.NoError(t, err)
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		out := map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &out)
		}
		return resp.StatusCode, out
	}
	call := func(method, path, body string) (int, map[string]any) {
		t.Helper()
		resp, err := http.NewRequest(method, base+path, strings.NewReader(body))
		require.NoError(t, err)
		got, err := http.DefaultClient.Do(resp)
		require.NoError(t, err)
		defer got.Body.Close()
		raw, err := io.ReadAll(got.Body)
		require.NoError(t, err)
		out := map[string]any{}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &out)
		}
		return got.StatusCode, out
	}

	// health：svcCtx + registry 有 agent → agentConnected + 函数清单
	code, health := get("/__fixture__/health")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", health["status"])
	assert.Equal(t, true, health["agentConnected"])
	assert.Contains(t, health["functions"], "mail.send")

	// provider calls/reset：先打一条 provider 调用再读回、重置
	doJSON(t, f.provider.handler(), http.MethodGet, "/players", "")
	code, calls := get("/__fixture__/provider/calls")
	require.Equal(t, http.StatusOK, code)
	require.NotEmpty(t, calls["calls"].([]any))
	code, _ = call(http.MethodGet, "/__fixture__/provider/reset", "")
	assert.Equal(t, http.StatusMethodNotAllowed, code)
	code, _ = call(http.MethodPost, "/__fixture__/provider/reset", "")
	require.Equal(t, http.StatusOK, code)
	code, calls = get("/__fixture__/provider/calls")
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, calls["calls"].([]any))

	// sdk/calls：POST 记录 / GET 读回 / DELETE 清空 / 非法 JSON 400 / 其他方法 405
	code, _ = call(http.MethodPost, "/__fixture__/sdk/calls", `{"functionId":"mail.send","payload":{"a":1}}`)
	require.Equal(t, http.StatusOK, code)
	code, calls = get("/__fixture__/sdk/calls")
	require.Equal(t, http.StatusOK, code)
	require.Len(t, calls["calls"].([]any), 1)
	code, _ = call(http.MethodPost, "/__fixture__/sdk/calls", `{bad`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = call(http.MethodPut, "/__fixture__/sdk/calls", `{}`)
	assert.Equal(t, http.StatusMethodNotAllowed, code)
	code, _ = call(http.MethodDelete, "/__fixture__/sdk/calls", "")
	require.Equal(t, http.StatusOK, code)
	code, calls = get("/__fixture__/sdk/calls")
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, calls["calls"].([]any))

	// sdk/functions：GET 405 / 非法 JSON 400 / POST 交给 /bin/true 假 provider
	code, _ = get("/__fixture__/sdk/functions")
	assert.Equal(t, http.StatusMethodNotAllowed, code)
	code, _ = call(http.MethodPost, "/__fixture__/sdk/functions", `{bad`)
	assert.Equal(t, http.StatusBadRequest, code)
	t.Setenv("CROUPIER_E2E_PROVIDER_BIN", "/bin/true")
	code, replaced := call(http.MethodPost, "/__fixture__/sdk/functions",
		`{"functions":[{"id":"mail.send","enabled":true}]}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, float64(1), replaced["count"])
	assert.Equal(t, []FixtureSDKFunction{{ID: "mail.send", Enabled: true}}, f.SDKFunctions())

	// audit/page-execute：有 DB 无行 → 404；方法不允许 → 405
	code, _ = get("/__fixture__/audit/page-execute")
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = call(http.MethodPost, "/__fixture__/audit/page-execute", "")
	assert.Equal(t, http.StatusMethodNotAllowed, code)

	// 无 DB 的 fixture → 503（database_unavailable）
	f2Addr, err := fixtureAddrWithPort("")
	require.NoError(t, err)
	f2 := &DashboardFixture{FixtureAddr: f2Addr, provider: newPlayersProvider()}
	require.NoError(t, f2.startFixtureAPI())
	t.Cleanup(func() { _ = f2.Close(context.Background()) })
	resp, err := http.Get("http://" + f2.FixtureAddr + "/__fixture__/audit/page-execute")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// listen 地址非法时 startFixtureAPI 报错（net.Listen 失败分支）。
func TestStartFixtureAPI_ListenError(t *testing.T) {
	f := &DashboardFixture{FixtureAddr: "bad-addr-without-port", provider: newPlayersProvider()}
	assert.Error(t, f.startFixtureAPI())
}

// 轻量工具面：DB/WaitReady/ready/SDKCalls 在未启动 fixture 时的防御语义。
func TestDashboardFixture_UnstartedHelpers(t *testing.T) {
	f := &DashboardFixture{}
	assert.Nil(t, f.DB())
	assert.False(t, f.ready())
	assert.Empty(t, f.SDKFunctions())
	assert.Empty(t, f.SDKCalls())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, f.WaitReady(ctx), context.Canceled)

	// svcCtx 缺 RegistryStore 的 ready 分支
	f.svcCtx = &svc.ServiceContext{}
	assert.False(t, f.ready())

	// ReplaceSDKFunctions：rootCtx 缺省时回落 WithoutCancel；
	// 外部 provider 二进制用 /bin/true（立即退出，Start 成功即可）。
	t.Setenv("CROUPIER_E2E_PROVIDER_BIN", "/bin/true")
	f2 := &DashboardFixture{BaseDir: t.TempDir(), FixtureAddr: "127.0.0.1:1", AgentLocalAddr: "127.0.0.1:1"}
	require.NoError(t, f2.ReplaceSDKFunctions(context.Background(), []FixtureSDKFunction{{ID: "x", Enabled: true}}))
	assert.Equal(t, []FixtureSDKFunction{{ID: "x", Enabled: true}}, f2.SDKFunctions())
}
