// 覆盖目标：Update/Create/Delete handler 全路径（含绑定错误、404、冲突）、queryInt/
// currentUser、loadContracts nil 分支与 DB 错误分支、生成器边界（nil/空 FunctionID 跳过、
// 无 ResourceKey 回退、delete 能力、Upsert 失败仅告警）、sanitizeKey 等纯函数。
package component

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newV4Router(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/api/v1/component-templates"))
	return r
}

func doReq(r *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var rd *strings.Reader
	if body == "" {
		rd = strings.NewReader("")
	} else {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, rd)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// ---- 纯函数 ----

func TestQueryInt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		query string
		want  int
	}{
		{"", 42},   // 缺省
		{"  ", 42}, // 空白
		{"7", 7},   // 正常数字
		{"1a", 42}, // 非数字回退缺省
		{" 9 ", 9}, // 带空白数字
		{"-3", 42}, // 负号非数字段回退
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/?page="+url.QueryEscape(tc.query), nil)
		assert.Equal(t, tc.want, queryInt(c, "page", 42), "query=%q", tc.query)
	}
}

func TestCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	assert.Empty(t, currentUser(c))

	c.Set("username", "alice")
	assert.Equal(t, "alice", currentUser(c))

	c.Set("username", 123) // 非字符串类型 → 空
	assert.Empty(t, currentUser(c))
}

func TestSanitizeKey(t *testing.T) {
	assert.Equal(t, "player.list", sanitizeKey("Player.List"))
	assert.Equal(t, "a-b", sanitizeKey("A B"))
	assert.Equal(t, "a--c", sanitizeKey("a!@c"))
	assert.Equal(t, "--", sanitizeKey("玩家")) // 非 ASCII 逐 rune 替换为 -
}

func TestLastSegment(t *testing.T) {
	assert.Equal(t, "send", lastSegment("mail.send"))
	assert.Equal(t, "solo", lastSegment("solo"))
}

func TestIconForView(t *testing.T) {
	assert.Equal(t, "TableOutlined", iconForView("table"))
	assert.Equal(t, "ProfileOutlined", iconForView("fields"))
	assert.Equal(t, "FormOutlined", iconForView("form"))
	assert.Equal(t, "FormOutlined", iconForView("unknown"))
}

func TestToJSONStringArray(t *testing.T) {
	assert.Equal(t, `[]`, toJSONStringArray(nil))
	assert.Equal(t, `["a","b"]`, toJSONStringArray([]string{"a", "b"}))
}

// ---- handler 层 ----

func TestCreateHandler_Errors(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	r := newV4Router(h)

	// JSON 语法错误 → 400。
	w := doReq(r, http.MethodPost, "/api/v1/component-templates", `{"key":`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// key 空白 → 400。
	w = doReq(r, http.MethodPost, "/api/v1/component-templates",
		`{"key":"  ","name":{"zh-CN":"x"},"tree":[]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "key")

	// 缺必填 name → 400。
	w = doReq(r, http.MethodPost, "/api/v1/component-templates", `{"key":"k","tree":[]}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 重复 key → 唯一索引冲突 → 500。
	body := `{"key":"dup--x","name":{"zh-CN":"x"},"tree":[{"type":"fnForm"}]}`
	w = doReq(r, http.MethodPost, "/api/v1/component-templates", body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	w = doReq(r, http.MethodPost, "/api/v1/component-templates", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestCreateHandler_CurrentUser(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("username", "qa-er"); c.Next() })
	h.Register(r.Group("/api/v1/component-templates"))

	w := doReq(r, http.MethodPost, "/api/v1/component-templates",
		`{"key":"mine--1","name":{"zh-CN":"我的"},"tree":[]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"createdBy":"qa-er"`)

	tpl, err := h.model.FindByKey(context.Background(), "mine--1")
	require.NoError(t, err)
	assert.Equal(t, "qa-er", tpl.CreatedBy)
}

func TestUpdateHandler(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	r := newV4Router(h)

	body := `{"key":"u--1","name":{"zh-CN":"原名"},"tree":[{"type":"fnForm"}]}`
	require.Equal(t, http.StatusOK, doReq(r, http.MethodPost, "/api/v1/component-templates", body).Code)

	// 全量更新：name/tree/description/category/icon/requiredFunctions。
	patch := `{"key":"u--1","name":{"zh-CN":"新名"},"tree":[{"type":"fnTable"}],"description":{"zh-CN":"d"},"category":"自定义","icon":"StarOutlined","requiredFunctions":["a.b"]}`
	w := doReq(r, http.MethodPut, "/api/v1/component-templates/u--1", patch)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"updated":"u--1"`)

	tpl, err := h.model.FindByKey(context.Background(), "u--1")
	require.NoError(t, err)
	assert.Contains(t, string(tpl.Name), "新名")
	assert.Contains(t, string(tpl.Description), "d")
	assert.Equal(t, "自定义", tpl.Category)
	assert.Equal(t, "StarOutlined", tpl.Icon)
	assert.Contains(t, string(tpl.RequiredFunctions), "a.b")
	assert.False(t, tpl.Builtin)

	// 不存在 → 404。
	w = doReq(r, http.MethodPut, "/api/v1/component-templates/none", patch)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// JSON 语法错误 → 400。
	w = doReq(r, http.MethodPut, "/api/v1/component-templates/u--1", `{`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteHandler_NotFound(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	r := newV4Router(h)
	w := doReq(r, http.MethodDelete, "/api/v1/component-templates/ghost", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListHandler_PaginationAndDbError(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	r := newV4Router(h)
	for i := range 3 {
		body := `{"key":"p--` + string(rune('a'+i)) + `","name":{"zh-CN":"x"},"tree":[]}`
		require.Equal(t, http.StatusOK, doReq(r, http.MethodPost, "/api/v1/component-templates", body).Code)
	}

	// page=2&pageSize=1 → 第 2 条；非法分页参数回退缺省。
	w := doReq(r, http.MethodGet, "/api/v1/component-templates?page=2&pageSize=1", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total":3`)
	assert.Contains(t, w.Body.String(), "p--b")

	w = doReq(r, http.MethodGet, "/api/v1/component-templates?page=zz", "")
	assert.Equal(t, http.StatusOK, w.Code)

	// DB 错误 → 500。
	require.NoError(t, db.Migrator().DropTable("component_templates"))
	w = doReq(r, http.MethodGet, "/api/v1/component-templates", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRegenerateHandler_NilContractModel(t *testing.T) {
	db := setupV4DB(t)
	// contractMdl 为 nil：loadContracts 返回 nil,nil，regenerate 仍成功。
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	r := newV4Router(h)
	w := doReq(r, http.MethodPost, "/api/v1/component-templates/regenerate", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"regenerated":0`)
}

func TestRegenerateHandler_LoadContractsError(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), model.NewFunctionContractModel(db))
	r := newV4Router(h)
	require.NoError(t, db.Migrator().DropTable("function_contracts"))
	w := doReq(r, http.MethodPost, "/api/v1/component-templates/regenerate", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---- 生成器 ----

func TestGenerateSingleFunctionTemplates_Edges(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	ctx := context.Background()

	// nil 契约、空 FunctionID 被跳过；无 ResourceKey 的 action 契约按 fid 前缀推导。
	contracts := []*model.FunctionContract{
		nil,
		{GameID: "demo", Env: "dev", FunctionID: "  "},
		{GameID: "demo", Env: "dev", FunctionID: "ticket.create", Capability: dbenum.CapabilityAction},
		{GameID: "demo", Env: "dev", FunctionID: "mail.get", ResourceKey: "mail", Capability: dbenum.CapabilityItemQuery},
	}
	require.NoError(t, h.GenerateSingleFunctionTemplates(ctx, contracts))

	_, total, err := h.model.List(ctx, model.ComponentTemplateListOptions{})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	tpl, err := h.model.FindByKey(ctx, "fn--ticket.create")
	require.NoError(t, err)
	assert.Equal(t, "函数组件", tpl.Category)
	assert.Equal(t, "FormOutlined", tpl.Icon) // action → form 视图
	assert.Contains(t, string(tpl.Name), "ticket·create")

	tpl, err = h.model.FindByKey(ctx, "fn--mail.get")
	require.NoError(t, err)
	assert.Equal(t, "ProfileOutlined", tpl.Icon) // item_query → fields 视图
}

func TestGenerateCRUDTemplate_MixedAndDelete(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	ctx := context.Background()

	contracts := []*model.FunctionContract{
		{GameID: "demo", Env: "dev", FunctionID: "order.list", ResourceKey: "order", Capability: dbenum.CapabilityCollectionQuery},
		{GameID: "demo", Env: "dev", FunctionID: "order.get", ResourceKey: "order", Capability: dbenum.CapabilityItemQuery},
		{GameID: "demo", Env: "dev", FunctionID: "order.delete", ResourceKey: "order", Capability: dbenum.CapabilityDelete},
		{GameID: "demo", Env: "dev", FunctionID: "other.list", ResourceKey: "other", Capability: dbenum.CapabilityCollectionQuery}, // 非目标资源被跳过
	}
	require.NoError(t, h.GenerateCRUDTemplate(ctx, "order", contracts))

	tpl, err := h.model.FindByKey(ctx, "crud--order")
	require.NoError(t, err)
	assert.Equal(t, "资源管理", tpl.Category)
	assert.Contains(t, string(tpl.RequiredFunctions), "order.list")
	assert.Contains(t, string(tpl.RequiredFunctions), "order.get")

	// 缺 list → 不生成。
	require.NoError(t, h.GenerateCRUDTemplate(ctx, "other", contracts))
	_, err = h.model.FindByKey(ctx, "crud--other")
	assert.Error(t, err)
}

func TestRegenerateFromContracts_UpsertErrorsLogged(t *testing.T) {
	db := setupV4DB(t)
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	ctx := context.Background()
	contracts := []*model.FunctionContract{
		{GameID: "demo", Env: "dev", FunctionID: "a.list", ResourceKey: "a", Capability: dbenum.CapabilityCollectionQuery},
		{GameID: "demo", Env: "dev", FunctionID: "a.get", ResourceKey: "a", Capability: dbenum.CapabilityItemQuery},
	}
	// 表被删除：单函数与 CRUD upsert 均失败，但只记日志不返回错误。
	require.NoError(t, db.Migrator().DropTable("component_templates"))
	assert.NoError(t, h.RegenerateFromContracts(ctx, contracts))
}
