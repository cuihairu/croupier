// 覆盖目标：validateTemplateParams 的 JSON 解析错误、collectTreeNodeIDs 的
// tree 解析错误、List 的 stale 检测（非 builtin 跳过 / builtin 家族 key 失配）、
// Create/Update 的 params 校验失败、Update 的 params 持久化与 DB 更新失败、
// RegenerateFromContracts 中查询组合模板 Upsert 失败的告警分支。
// 不改变产品语义：仅注入 DB 故障与非常规输入。
package component

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// ---- gorm 错误注入 ----

var covSchemaCache = &sync.Map{}

func covStmtTable(tx *gorm.DB) string {
	if tx.Statement == nil {
		return ""
	}
	if tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	for _, v := range []interface{}{tx.Statement.Model, tx.Statement.Dest} {
		if v == nil {
			continue
		}
		if s, err := schema.Parse(v, covSchemaCache, schema.NamingStrategy{}); err == nil {
			return s.Table
		}
	}
	return ""
}

type covFailureInjector struct {
	mu      sync.Mutex
	counts  map[string]int
	failAt  map[string]int
	failAll map[string]bool
}

func newCovFailureInjector() *covFailureInjector {
	return &covFailureInjector{
		counts:  map[string]int{},
		failAt:  map[string]int{},
		failAll: map[string]bool{},
	}
}

func (f *covFailureInjector) register(db *gorm.DB) {
	callback := func(op string) func(tx *gorm.DB) {
		return func(tx *gorm.DB) {
			table := covStmtTable(tx)
			if table == "" {
				return
			}
			key := op + ":" + table
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.failAll[key] {
				_ = tx.AddError(fmt.Errorf("injected %s failure on %s", op, table))
				return
			}
			if n, ok := f.failAt[key]; ok {
				f.counts[key]++
				if f.counts[key] >= n {
					_ = tx.AddError(fmt.Errorf("injected %s failure on %s", op, table))
				}
			}
		}
	}
	_ = db.Callback().Create().Before("gorm:create").Register("cov_fail_create", callback("create"))
	_ = db.Callback().Query().Before("gorm:query").Register("cov_fail_query", callback("query"))
	_ = db.Callback().Row().Before("gorm:row").Register("cov_fail_row", callback("query"))
	_ = db.Callback().Update().Before("gorm:update").Register("cov_fail_update", callback("update"))
	_ = db.Callback().Delete().Before("gorm:delete").Register("cov_fail_delete", callback("delete"))
}

// ---- 纯函数 ----

// validateTemplateParams：params 非合法 JSON。
func TestValidateTemplateParamsCov_UnmarshalError(t *testing.T) {
	out, err := validateTemplateParams(json.RawMessage(`not-json`), json.RawMessage(`[{"id":"n1"}]`))
	require.Error(t, err)
	assert.Nil(t, out)
	assert.Contains(t, err.Error(), "params 格式无效")
}

// collectTreeNodeIDs：tree 非法 JSON 时直接返回（不收集任何节点）。
func TestCollectTreeNodeIDSCov_BadTreeJSON(t *testing.T) {
	out := map[string]bool{}
	collectTreeNodeIDs(json.RawMessage(`{oops`), out)
	assert.Empty(t, out)

	// 非数组但合法的 JSON 同样无法解码为节点列表。
	out2 := map[string]bool{}
	collectTreeNodeIDs(json.RawMessage(`{"id":"n1"}`), out2)
	assert.Empty(t, out2)
}

// ---- handler ----

// Create：params 校验失败 → 400。
func TestCreateHandlerCov_ParamsInvalid(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	r := newV4Router(h)

	w := doReq(r, http.MethodPost, "/api/v1/component-templates",
		`{"key":"p--bad","name":{"zh-CN":"x"},"params":"oops","tree":[{"id":"n1"}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "params")
}

// Update：params 校验失败 → 400。
func TestUpdateHandlerCov_ParamsInvalid(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	r := newV4Router(h)

	body := `{"key":"u--p","name":{"zh-CN":"x"},"tree":[{"id":"n1"}]}`
	require.Equal(t, http.StatusOK, doReq(r, http.MethodPost, "/api/v1/component-templates", body).Code)

	w := doReq(r, http.MethodPut, "/api/v1/component-templates/u--p",
		`{"key":"u--p","name":{"zh-CN":"y"},"params":[{"key":"a"}],"tree":[{"id":"n1"}]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Update：合法 params 被持久化（updates["params"] 分支）。
func TestUpdateHandlerCov_ParamsPersisted(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	r := newV4Router(h)

	body := `{"key":"u--pp","name":{"zh-CN":"x"},"tree":[{"id":"n1"}]}`
	require.Equal(t, http.StatusOK, doReq(r, http.MethodPost, "/api/v1/component-templates", body).Code)

	patch := `{"key":"u--pp","name":{"zh-CN":"y"},"params":[{"key":"title1","label":{"zh-CN":"标题"},"nodeId":"n1","prop":"title","default":"你好"}],"tree":[{"id":"n1"}]}`
	w := doReq(r, http.MethodPut, "/api/v1/component-templates/u--pp", patch)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	tpl, err := h.model.FindByKey(context.Background(), "u--pp")
	require.NoError(t, err)
	assert.Contains(t, string(tpl.Params), `"title1"`)
}

// Update：model.Update DB 失败 → 500（FindByKey 先行成功后注入 update 错误）。
func TestUpdateHandlerCov_UpdateDBError(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	r := newV4Router(h)

	body := `{"key":"u--db","name":{"zh-CN":"x"},"tree":[{"id":"n1"}]}`
	require.Equal(t, http.StatusOK, doReq(r, http.MethodPost, "/api/v1/component-templates", body).Code)

	inj := newCovFailureInjector()
	inj.register(db)
	inj.failAll["update:component_templates"] = true

	w := doReq(r, http.MethodPut, "/api/v1/component-templates/u--db",
		`{"key":"u--db","name":{"zh-CN":"y"},"tree":[{"id":"n2"}]}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---- List 的 stale 检测 ----

// 契约存在时：非 builtin 模板跳过 stale 检测；builtin 家族 key 无对应
// 生成条件（函数已删除/能力变更）时标记 stale。
func TestListHandlerCov_StaleDetectionBranches(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), model.NewFunctionContractModel(db))
	r := newV4Router(h)

	// 契约：一个 action 函数（生成 fn--cov.fn 期望值）。
	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "covgame", Env: "dev", FunctionID: "cov.fn", Version: "1.0.0",
		Enabled: true, Capability: dbenum.CapabilityAction, Execution: "sync",
	}).Error)

	// 非 builtin 模板：即使 key 不在期望集也不参与 stale 检测。
	require.NoError(t, db.Create(&model.ComponentTemplate{
		Key: "custom--x", Name: model.JSON(`{"zh-CN":"自定义"}`),
		Tree: model.JSON(`[]`), Builtin: false, CreatedBy: "cov",
	}).Error)
	// builtin 家族 key 但无生成条件 → stale。
	require.NoError(t, db.Create(&model.ComponentTemplate{
		Key: "fn--ghost", Name: model.JSON(`{"zh-CN":"幽灵"}`),
		Tree: model.JSON(`[]`), Builtin: true,
	}).Error)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/component-templates", nil)
	req.Header.Set("X-Game-ID", "covgame")
	req.Header.Set("X-Env", "dev")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"stale":true`)
	assert.Contains(t, w.Body.String(), `"fn--ghost"`)
}

// ---- 生成器 ----

// RegenerateFromContracts：带查询参数的 collection_query 契约生成查询组合
// 模板时 Upsert 失败 → 仅记告警，整体仍成功（slog 分支）。
func TestRegenerateFromContractsCov_QueryTemplateUpsertFailure(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), model.NewFunctionContractModel(db))

	contracts := []*model.FunctionContract{{
		GameID: "covgame", Env: "dev", FunctionID: "cov.search", Version: "1.0.0",
		Enabled: true, Capability: dbenum.CapabilityCollectionQuery, Execution: "sync",
		InputSchema: model.JSON(`{"type":"object","properties":{"kw":{"type":"string"}}}`),
	}}

	// 表删除后单函数/查询组合/CRUD 的 upsert 全部失败，但只告警不返回错误。
	require.NoError(t, db.Migrator().DropTable("component_templates"))
	require.NoError(t, h.RegenerateFromContracts(context.Background(), contracts))
}
