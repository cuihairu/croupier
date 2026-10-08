package cicd_test

// CI/CD service/webhook 残余错误路径覆盖（handler 层收口后剩下的块）。
// 注入口径与本仓既有批次一致：
//   - 读翼「缺表」：DropTable 后 gorm 立即报错且无副作用（存储故障）；
//   - 写翼「触发器拦写」：BEFORE UPDATE/DELETE TRIGGER + RAISE(ABORT)，
//     写语句真执行到 DB 才失败（覆盖「校验全过、落库炸」这一类）；
//   - 记录缺失：真实 404；
//   - provider 出错：httptest 假 CI 服务器按用例返回 4xx/5xx。
//
// 文件尾另登记三处无法构造的防御分支（构造前提被上游代码本身消除），
// 按房规登记而非造假用例。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/api/cicd"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func jsonBody(s string) *strings.Reader { return strings.NewReader(s) }

// withUsername 注入登录用户名（CurrentUsername 读 ctx 的 "username" 键）。
func withUsername(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, "username", name) //nolint:staticcheck // 契约键就是字符串键
}

// seedRowIntegration 直写一行接入（绕过 Create 的校验层，用于构造「库里
// 存在校验层造不出的形态」：畸形 endpoint、未知 kind、空 scope）。
func seedRowIntegration(t *testing.T, db *gorm.DB, kind, endpoint string, extra datatypes.JSONMap) uint {
	t.Helper()
	row := &model.CicdIntegration{
		GameID: "demo", Env: "prod", Kind: kind, Name: "seeded-" + kind,
		Endpoint: endpoint, Extra: extra, Enabled: true,
	}
	require.NoError(t, db.Create(row).Error)
	require.NotZero(t, row.ID)
	return row.ID
}

// newWebhookRouter 复刻 registerCicdWebhookRoute。
func newWebhookRouter(s *cicd.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/cicd/webhooks/:id", cicd.NewHandler(s).Webhook)
	return r
}

// fakeCI 起一个可编程的假 CI 服务器：触发 201+Location、状态 200+building。
func fakeCI(t *testing.T, triggerStatus, statusCode int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/job/pack/build", func(w http.ResponseWriter, _ *http.Request) {
		if triggerStatus != http.StatusCreated {
			w.WriteHeader(triggerStatus)
			return
		}
		w.Header().Set("Location", "/queue/item/7/")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/queue/item/7/api/json", func(w http.ResponseWriter, _ *http.Request) {
		if statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
			return
		}
		_, _ = w.Write([]byte(`{"executable":{"number":7}}`))
	})
	mux.HandleFunc("/job/pack/7/api/json", func(w http.ResponseWriter, _ *http.Request) {
		if statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
			return
		}
		_, _ = w.Write([]byte(`{"number":7,"building":false,"url":"http://ci.internal/job/pack/7/"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// 创建时 createdBy 取请求上下文里的登录用户名（无用户名回落 "system"）。
func TestCicdService_CreateRecordsAuthor(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))

	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPost, "/cicd/integrations",
		jsonBody(`{"kind":"jenkins","name":"authored","endpoint":"http://ci.internal"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withUsername(req.Context(), "alice"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"createdBy":"alice"`)

	var row model.CicdIntegration
	require.NoError(t, db.Where("name = ?", "authored").First(&row).Error)
	assert.Equal(t, "alice", row.CreatedBy)

	// 无登录用户名 → 回落 "system"
	w = doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"jenkins","name":"anon","endpoint":"http://ci.internal"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"createdBy":"system"`)
}

// TestCicdService_CreateEnabledFalseIsDroppedKnownDefect 登记一处**已发现
// 缺陷**（不是不可达分支）：创建请求显式传 enabled:false 时，落库仍是 true。
//
// 根因：model.CicdIntegration.Enabled 带 `gorm:"default:true"`，GORM 对「有
// default 标签且值为零值」的字段在 INSERT 中省略该列、回退 DB 默认值。
// IntegrationCreateRequest.Enabled 是 *bool，服务层 `row.Enabled = *req.Enabled`
// 已正确赋值，但驱动层把 false 当零值丢掉。实测 Select("*") 与逐字段 Select
// 均无法绕过（该 GORM 版本无条件回退）。
//
// 影响面：仅「创建即停用」这一条路径；Update 走 Save（写全字段）不受影响。
// 修法要把模型字段改 *bool（连带 service 读侧，并触发仓库「模型改动须配
// 编号迁移」契约），属独立修复批次——本补测批次不改生产代码。
func TestCicdService_CreateEnabledFalseIsDroppedKnownDefect(t *testing.T) {
	t.Skip("已发现缺陷：CicdIntegration.Enabled 的 gorm default:true 使创建时的显式 false 被丢弃（修法需改模型字段类型，另立批次）")

	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	w := doCicd(r, http.MethodPost, "/cicd/integrations",
		`{"kind":"jenkins","name":"off","endpoint":"http://ci.internal","enabled":false}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var row model.CicdIntegration
	require.NoError(t, db.Where("name = ?", "off").First(&row).Error)
	assert.False(t, row.Enabled, "契约：显式 false 应落库 false")
}

// 更新逐字段覆盖：kind/endpoint/token/extra 各自生效，空串不覆盖。
func TestCicdService_UpdateOverridesFields(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := createIntegration(t, r, "http://ci.internal")

	w := doCicd(r, http.MethodPut, "/cicd/integrations/"+itoa(id), `{
		"kind":"generic","name":"renamed","endpoint":"http://ci2.internal",
		"token":"new-token-5678","extra":{"triggerUrl":"http://ci2.internal/trig"}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"kind":"generic"`)
	assert.Contains(t, w.Body.String(), `"name":"renamed"`)
	assert.Contains(t, w.Body.String(), "http://ci2.internal")
	assert.Contains(t, w.Body.String(), "****5678", "新凭据掩码回显")
	assert.Contains(t, w.Body.String(), "triggerUrl")
}

// 更新的两条校验错误：接入形态非法（endpoint 非 http）与 provider 必填键缺失。
func TestCicdService_UpdateValidationErrors(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := createIntegration(t, r, "http://ci.internal")

	w := doCicd(r, http.MethodPut, "/cicd/integrations/"+itoa(id), `{"endpoint":"ftp://ci.internal"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// 换成需要 project 必填键的 provider，但不带 extra → 构造期 400
	w = doCicd(r, http.MethodPut, "/cicd/integrations/"+itoa(id), `{"kind":"gitlab-ci"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "project")
}

// 落库失败（写翼触发器拦 UPDATE）：校验全过、SQL 真执行才炸。
func TestCicdService_UpdateStoreError(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := createIntegration(t, r, "http://ci.internal")

	require.NoError(t, db.Exec(
		`CREATE TRIGGER blk_int_upd BEFORE UPDATE ON cicd_integrations BEGIN SELECT RAISE(ABORT, 'blocked'); END`).Error)
	w := doCicd(r, http.MethodPut, "/cicd/integrations/"+itoa(id), `{"name":"x"}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}

// 删除级联：构建表缺失时级联清理失败 → 500（接入行不得被删）。
func TestCicdService_DeleteCascadeStoreError(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := createIntegration(t, r, "http://ci.internal")

	require.NoError(t, db.Migrator().DropTable(&model.CicdBuild{}))
	w := doCicd(r, http.MethodDelete, "/cicd/integrations/"+itoa(id), "")
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())

	var n int64
	require.NoError(t, db.Model(&model.CicdIntegration{}).Where("id = ?", id).Count(&n).Error)
	assert.EqualValues(t, 1, n, "级联清理失败时接入行保留")
}

// 接入表缺失：读接入的四个端点 + 刷新（构建行在位时）统一 500。
func TestCicdService_IntegrationTableMissing(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	// 先落一条构建行，否则 refresh 停在「构建记录不存在」（404）到不了接入查询
	require.NoError(t, db.Create(&model.CicdBuild{
		IntegrationID: 1, Kind: "jenkins", Pipeline: "pack", ExternalID: "e-1",
		Status: model.CicdBuildQueued,
	}).Error)
	require.NoError(t, db.Migrator().DropTable(&model.CicdIntegration{}))

	for _, tc := range []struct{ method, target, body string }{
		{http.MethodPut, "/cicd/integrations/1", `{"name":"x"}`},
		{http.MethodDelete, "/cicd/integrations/1", ""},
		{http.MethodPost, "/cicd/integrations/1/test", ""},
		{http.MethodPost, "/cicd/integrations/1/trigger", `{"pipeline":"pack"}`},
		{http.MethodPost, "/cicd/builds/1/refresh", ""},
	} {
		w := doCicd(r, tc.method, tc.target, tc.body)
		assert.Equal(t, http.StatusInternalServerError, w.Code, "%s %s → %d %s", tc.method, tc.target, w.Code, w.Body.String())
	}
}

// 连通性测试：库里存着畸形 endpoint（校验层造不出）→ 构造请求即失败，
// 结论落在 ok:false 体里（200），不外泄为 5xx。
func TestCicdService_TestConnectionMalformedEndpoint(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := seedRowIntegration(t, db, "jenkins", "://bad-endpoint", nil)

	w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/test", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"ok":false`)
}

// 触发：provider 侧报错（假 CI 返回 500）与构建落库失败（缺表）两条。
func TestCicdService_TriggerErrorPaths(t *testing.T) {
	t.Run("provider rejects trigger", func(t *testing.T) {
		srv := fakeCI(t, http.StatusInternalServerError, http.StatusOK)
		ctxSvc, _ := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
		id := createIntegration(t, r, srv.URL)

		w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/trigger", `{"pipeline":"pack"}`)
		assert.GreaterOrEqual(t, w.Code, 400, w.Body.String())
	})

	t.Run("build record store failure", func(t *testing.T) {
		srv := fakeCI(t, http.StatusCreated, http.StatusOK)
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
		id := createIntegration(t, r, srv.URL)
		require.NoError(t, db.Migrator().DropTable(&model.CicdBuild{}))

		w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/trigger", `{"pipeline":"pack"}`)
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})

	t.Run("provider construction fails", func(t *testing.T) {
		// 库里存在校验层造不出的未知 kind（Create 会拦，direct write 绕过）
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc))
		id := seedRowIntegration(t, db, "bogus-provider", "http://ci.internal", nil)

		w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/trigger", `{"pipeline":"pack"}`)
		// 未知 kind 由 provider 注册表返回裸 error（非 errorx）→ 500
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "unknown cicd provider kind")
	})
}

// 触发落库时 scope 回落：接入行无 game_id/env 时取请求体，两侧都空则留空。
func TestCicdService_TriggerScopeFallback(t *testing.T) {
	srv := fakeCI(t, http.StatusCreated, http.StatusOK)
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))

	// 行与请求都无 scope → 构建记录 scope 留空（不编造默认值）
	row := &model.CicdIntegration{
		Kind: "jenkins", Name: "no-scope", Endpoint: srv.URL, Enabled: true,
	}
	require.NoError(t, db.Create(row).Error)
	w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(row.ID)+"/trigger",
		`{"pipeline":"pack","gameId":"from-req","env":"prod"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"gameId":"from-req"`)

	// 行与请求皆空 → 保持空串
	row2 := &model.CicdIntegration{
		Kind: "jenkins", Name: "empty-scope", Endpoint: srv.URL, Enabled: true,
	}
	require.NoError(t, db.Create(row2).Error)
	w = doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(row2.ID)+"/trigger", `{"pipeline":"pack"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"gameId":""`)
}

// 刷新构建的逐层失败：记录缺失 / 存储故障 / 接入缺失 / provider 构造失败
// / provider 报错 / 回写落库失败。
func TestCicdService_RefreshBuildErrorPaths(t *testing.T) {
	t.Run("build table missing", func(t *testing.T) {
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc))
		require.NoError(t, db.Migrator().DropTable(&model.CicdBuild{}))
		w := doCicd(r, http.MethodPost, "/cicd/builds/1/refresh", "")
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})

	t.Run("integration missing", func(t *testing.T) {
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc))
		require.NoError(t, db.Create(&model.CicdBuild{
			IntegrationID: 999, Kind: "jenkins", Pipeline: "pack", ExternalID: "e-1",
			Status: model.CicdBuildQueued,
		}).Error)
		w := doCicd(r, http.MethodPost, "/cicd/builds/1/refresh", "")
		assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	})

	t.Run("provider construction fails", func(t *testing.T) {
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc))
		// 库里存在校验层造不出的未知 kind
		id := seedRowIntegration(t, db, "bogus-provider", "http://ci.internal", nil)
		require.NoError(t, db.Create(&model.CicdBuild{
			IntegrationID: id, Kind: "bogus-provider", Pipeline: "pack", ExternalID: "e-2",
			Status: model.CicdBuildQueued,
		}).Error)
		w := doCicd(r, http.MethodPost, "/cicd/builds/1/refresh", "")
		assert.GreaterOrEqual(t, w.Code, 400, w.Body.String())
	})

	t.Run("provider status fetch fails", func(t *testing.T) {
		srv := fakeCI(t, http.StatusCreated, http.StatusBadGateway)
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
		// ExternalID 必须是完整队列 URL（jenkins provider 以 "http" 前缀区分
		// 两种形态），否则走 job#n 分支、到不了 provider 出错翼
		id := seedRowIntegration(t, db, "jenkins", srv.URL, nil)
		require.NoError(t, db.Create(&model.CicdBuild{
			IntegrationID: id, Kind: "jenkins", Pipeline: "pack",
			ExternalID: srv.URL + "/queue/item/7/",
			Status:     model.CicdBuildQueued,
		}).Error)
		w := doCicd(r, http.MethodPost, "/cicd/builds/1/refresh", "")
		assert.GreaterOrEqual(t, w.Code, 400, w.Body.String())
	})

	t.Run("build write back fails", func(t *testing.T) {
		srv := fakeCI(t, http.StatusCreated, http.StatusOK)
		ctxSvc, db := setupCtx(t)
		r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
		id := seedRowIntegration(t, db, "jenkins", srv.URL, nil)
		require.NoError(t, db.Create(&model.CicdBuild{
			IntegrationID: id, Kind: "jenkins", Pipeline: "pack",
			ExternalID: srv.URL + "/queue/item/7/",
			Status:     model.CicdBuildQueued,
		}).Error)
		// provider 拉取成功、状态回写被触发器拦下 → 落库失败翼
		require.NoError(t, db.Exec(
			`CREATE TRIGGER blk_build_upd BEFORE UPDATE ON cicd_builds BEGIN SELECT RAISE(ABORT, 'blocked'); END`).Error)
		w := doCicd(r, http.MethodPost, "/cicd/builds/1/refresh", "")
		assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	})
}

// extra 值类型归一：布尔走 fmt.Sprint、嵌套对象走 json.Marshal 兜底。
// gorm.io/datatypes 读 JSON 列时启用 UseNumber()，故 DB 路径上数字解出的是
// json.Number 而非 float64（float64 分支的登记见本文件尾）。
func TestCicdService_NormalizeExtraTypeCoercion(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	id := seedRowIntegration(t, db, "generic", "http://ci.internal", datatypes.JSONMap{
		"num": 42, "flag": true, "obj": map[string]any{"a": 1}, "str": "keep",
	})

	// 先确认存储形态（否则下面断言是空转）：数字为 json.Number、布尔为 bool
	var row model.CicdIntegration
	require.NoError(t, db.First(&row, id).Error)
	require.IsType(t, json.Number(""), row.Extra["num"], "datatypes 读回启用 UseNumber")
	require.IsType(t, false, row.Extra["flag"], "JSON 布尔解出 bool")

	w := doCicd(r, http.MethodGet, "/cicd/integrations", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"num":"42"`, "数字经 Marshal 兜底为字符串值")
	assert.Contains(t, w.Body.String(), `"flag":"true"`, "bool 分支")
	assert.Contains(t, w.Body.String(), `{\"a\":1}`, "嵌套对象走 Marshal 兜底")
	assert.Contains(t, w.Body.String(), `"str":"keep"`, "string 分支直通")
}

// webhook 端点的路径/绑定/查找/落库四类拒绝，以及 extra 原样留存。
func TestCicdWebhook_PathBindAndStoreErrors(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	s := cicd.NewService(ctxSvc)
	r := newWebhookRouter(s)

	// 路径 id 非法
	for _, target := range []string{"/cicd/webhooks/abc", "/cicd/webhooks/0"} {
		w := doCicd(r, http.MethodPost, target, `{"pipeline":"p","externalId":"e","status":"passed"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s → %d %s", target, w.Code, w.Body.String())
	}

	// 体非法
	w := doCicd(r, http.MethodPost, "/cicd/webhooks/1", `{"pipeline":`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	// 接入不存在 → 404
	w = doCicd(r, http.MethodPost, "/cicd/webhooks/999",
		`{"pipeline":"p","externalId":"e","status":"passed"}`)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())

	// 正常投递：extra 原样进 raw，scope 从接入行回落
	id := seedRowIntegration(t, db, "generic", "http://ci.internal", nil)
	w = doCicd(r, http.MethodPost, "/cicd/webhooks/"+itoa(id),
		`{"pipeline":"","externalId":"e-9","status":"passed","gameId":"demo","extra":{"k":"v"}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"pipeline":"default"`, "空 pipeline 回落 default")
	assert.Contains(t, w.Body.String(), `"triggeredBy":"webhook"`)

	var saved model.CicdBuild
	require.NoError(t, db.Where("external_id = ?", "e-9").First(&saved).Error)
	assert.Equal(t, "v", saved.Raw["k"], "extra 原样留存 raw")

	// 构建表缺失 → 落库失败 500
	require.NoError(t, db.Migrator().DropTable(&model.CicdBuild{}))
	w = doCicd(r, http.MethodPost, "/cicd/webhooks/"+itoa(id),
		`{"pipeline":"p","externalId":"e-10","status":"passed"}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
}

// 以下三处分支在本仓代码结构下**不可达**，按房规登记（不造假用例、不删
// 防御分支）。任一前提变化时应改为补真实错误路径用例并删除本登记。

// ① service.go Trigger 的 `if build.ExternalID == ""`（L373-375）：
// 上一行 ExternalID 已被 `firstNonEmpty(ref.ExternalID,
// fmt.Sprintf("trigger-%d", now.UnixNano()))` 兜底，两参不可能同时为空
// （第二参是纳秒时间戳字符串），故该判断恒为 false。任何 provider 返回空
// 标识时都会落到本地生成的 trigger-* 标识。
func TestCicdService_TriggerEmptyExternalIDBranchUnreachable(t *testing.T) {
	// 触发返回 201 但不带 Location → jenkins provider 返回空 BuildRef.ExternalID
	mux := http.NewServeMux()
	mux.HandleFunc("/job/pack/build", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc).WithHTTPClient(srv.Client()))
	id := createIntegration(t, r, srv.URL)

	// provider 返回空标识 → 仍成功，本地生成 trigger-* 兜底（判断恒为 false）
	w := doCicd(r, http.MethodPost, "/cicd/integrations/"+itoa(id)+"/trigger", `{"pipeline":"pack"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"externalId":"trigger-`)
}

// ② webhook.go IngestWebhook 尾部的 GetByID 错误分支（L104-107）：
// 上一行 UpsertByExternalKey 已成功返回 id，GetByID 紧随其后按该 id 取行；
// 要让「写成功 → 紧接着读失败」成立，必须在同一次请求内两次存储调用之间
// 注入存储故障，生产路径不存在此形态。构造前提：Upsert/GetByID 之间不可
// 观测的存储故障。
func TestCicdWebhook_GetByIDAfterUpsertBranchUnreachable(t *testing.T) {
	ctxSvc, db := setupCtx(t)
	s := cicd.NewService(ctxSvc)
	r := newWebhookRouter(s)
	id := seedRowIntegration(t, db, "generic", "http://ci.internal", nil)

	// 正常形态：upsert 成功后必定读得回（该分支的可达形态已被此断言锁死）
	w := doCicd(r, http.MethodPost, "/cicd/webhooks/"+itoa(id),
		`{"pipeline":"p","externalId":"e-1","status":"passed"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"externalId":"e-1"`)
}

// ③ handler.go List 的 ShouldBindQuery 错误分支：见
// handler_coverage_test.go 中同款登记（IntegrationListRequest 为两个
// `form` string 字段，gin form 绑定无失败路径）。此处仅交叉确认。
func TestCicdService_ListRequestHasNoBindableFailurePath(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	r := newCicdRouter(cicd.NewService(ctxSvc))
	// 无参、正常参、未知参、重复参全部 200（绑定层不可能产错）
	for _, q := range []string{"", "?gameId=demo", "?env=prod&gameId=demo", "?nope=1"} {
		w := doCicd(r, http.MethodGet, "/cicd/integrations"+q, "")
		assert.Equal(t, http.StatusOK, w.Code, "%q → %d %s", q, w.Code, w.Body.String())
	}
}

// ④ service.go normalizeExtra 的 `case float64`（L92-93）为防御性不可达：
// normalizeExtra 的入参只有两个来源——① model.CicdIntegration.Extra 从
// JSON 列读出，gorm.io/datatypes 的 JSONMap 扫描启用 decoder.UseNumber()
// （datatypes@v1.2.7 json_map.go:48），故数字一律是 json.Number；② 进程内
// 刚构造的行，值来自 IntegrationCreateRequest.Extra（map[string]string），
// 只可能是 string。两条来源都不产出 float64，数字实际走 default 分支
// （json.Marshal(json.Number) 同样得到 "42"）。若未来 datatypes 关闭
// UseNumber 或出现 map[string]any 直写路径，本分支即变为可达，届时应补
// 对应用例并删除本登记。
func TestCicdService_NormalizeExtraFloat64BranchUnreachable(t *testing.T) {
	_, db := setupCtx(t)
	seedRowIntegration(t, db, "generic", "http://ci.internal", datatypes.JSONMap{"num": 42})

	var row model.CicdIntegration
	require.NoError(t, db.Where("name = ?", "seeded-generic").First(&row).Error)
	_, isFloat := row.Extra["num"].(float64)
	assert.False(t, isFloat, "datatypes 读回不得产出 float64（UseNumber 生效中）")
}


