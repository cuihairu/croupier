package component

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// V4-6 集成测试：契约→生成模板→CRUD API→模板结构断言（round-trip）。

var v4Seq int

func setupV4DB(t *testing.T) *gorm.DB {
	t.Helper()
	v4Seq++
	db, err := gorm.Open(
		gsqlite.Open(fmt.Sprintf("file:v4it%d?mode=memory&cache=shared", v4Seq)),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.ComponentTemplate{},
		&model.FunctionContract{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
	))
	return db
}

func seedContracts(t *testing.T, db *gorm.DB) {
	t.Helper()
	contracts := []*model.FunctionContract{
		{GameID: "demo", Env: "dev", FunctionID: "player.list", ResourceKey: "player", Capability: dbenum.CapabilityCollectionQuery},
		{GameID: "demo", Env: "dev", FunctionID: "player.get", ResourceKey: "player", Capability: dbenum.CapabilityItemQuery},
		{GameID: "demo", Env: "dev", FunctionID: "player.create", ResourceKey: "player", Capability: dbenum.CapabilityCreate},
		{GameID: "demo", Env: "dev", FunctionID: "player.update", ResourceKey: "player", Capability: dbenum.CapabilityUpdate},
		{GameID: "demo", Env: "dev", FunctionID: "mail.send", ResourceKey: "mail", Capability: dbenum.CapabilityAction},
	}
	for _, c := range contracts {
		require.NoError(t, db.Create(c).Error)
	}
}

func TestV4_GenerateAndCRUDRoundTrip(t *testing.T) {
	db := setupV4DB(t)
	seedContracts(t, db)

	h := NewHandler(
		model.NewComponentTemplateModel(db),
		model.NewFunctionContractModel(db),
	)
	ctx := context.Background()

	// 1. 从契约生成内置模板
	contracts, err := h.loadContractsForTest(ctx, "demo", "dev")
	require.NoError(t, err)
	require.NotEmpty(t, contracts)

	require.NoError(t, h.RegenerateFromContracts(ctx, contracts))

	// 2. 验证单函数模板
	tpls, total, err := h.model.List(ctx, model.ComponentTemplateListOptions{})
	require.NoError(t, err)
	assert.True(t, total >= 5, "至少 5 个模板（5 函数 + CRUD 组合），got %d", total)

	var playerListTpl *model.ComponentTemplate
	for i := range tpls {
		if tpls[i].Key == "fn--player.list" {
			playerListTpl = &tpls[i]
		}
	}
	require.NotNil(t, playerListTpl, "单函数模板 fn--player.list 应存在")
	assert.Equal(t, "table", extractViewFromTree(string(playerListTpl.Tree)))
	assert.Contains(t, string(playerListTpl.RequiredFunctions), "player.list")

	// 3. 验证 CRUD 组合模板
	var crudTpl *model.ComponentTemplate
	for i := range tpls {
		if tpls[i].Key == "crud--player" {
			crudTpl = &tpls[i]
		}
	}
	require.NotNil(t, crudTpl, "CRUD 组合模板 crud--player 应存在")
	treeStr := string(crudTpl.Tree)
	assert.Contains(t, treeStr, "player.list", "CRUD 应包含 list 函数")
	assert.Contains(t, treeStr, "player.get", "CRUD 应包含 get 函数")
	assert.Contains(t, treeStr, "player.create", "CRUD 应包含 create 函数")
	assert.Contains(t, treeStr, "player.update", "CRUD 应包含 update 函数")
	assert.Contains(t, treeStr, "detail-modal", "CRUD 应有详情弹窗")
	assert.Contains(t, treeStr, "create-modal", "CRUD 应有新建弹窗")
	assert.Contains(t, treeStr, "onSuccess", "CRUD 应有联动刷新")

	// 4. 验证非 CRUD 资源不生成组合模板（mail 只有 action）
	var mailCrud *model.ComponentTemplate
	for i := range tpls {
		if tpls[i].Key == "crud--mail" {
			mailCrud = &tpls[i]
		}
	}
	assert.Nil(t, mailCrud, "mail 无 collection_query+item_query，不应生成 CRUD 模板")

	// 5. 用户创建自定义模板
	err = h.model.Create(ctx, &model.ComponentTemplate{
		Key:               "custom--test",
		Name:              model.JSON(`{"zh-CN":"测试组件"}`),
		Category:          "自定义",
		RequiredFunctions: model.JSON(`["player.list"]`),
		Tree:              model.JSON(`[{"type":"fnTable","props":{"functionId":"player.list"}}]`),
		Builtin:           false,
	})
	require.NoError(t, err)

	// 6. 删除自定义模板
	var custom *model.ComponentTemplate
	custom, err = h.model.FindByKey(ctx, "custom--test")
	require.NoError(t, err)
	require.NoError(t, h.model.Delete(ctx, custom.ID))

	// 7. 删除内置模板应被拒
	err = h.model.Delete(ctx, playerListTpl.ID)
	assert.Error(t, err, "内置模板不可删除")

	// 8. 幂等：重新生成不重复
	require.NoError(t, h.RegenerateFromContracts(ctx, contracts))
	tpls2, total2, _ := h.model.List(ctx, model.ComponentTemplateListOptions{})
	assert.Equal(t, total, total2, "重新生成后数量不变（幂等 upsert）")
	_ = tpls2
}

func TestV4_HandlerAPI(t *testing.T) {
	db := setupV4DB(t)
	seedContracts(t, db)

	h := NewHandler(
		model.NewComponentTemplateModel(db),
		model.NewFunctionContractModel(db),
	)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/api/v1/component-templates"))

	// 1. 触发生成
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/component-templates/regenerate", nil)
	req.Header.Set("X-Game-ID", "demo")
	req.Header.Set("X-Env", "dev")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 2. 列表
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/component-templates", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "fn--player.list")
	assert.Contains(t, w.Body.String(), "crud--player")

	// 3. 详情
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/component-templates/crud--player", nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "player.list")

	// 4. 不存在
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/component-templates/nonexistent", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)

	// 5. 创建自定义
	body := `{"key":"custom--api","name":{"zh-CN":"API组件"},"tree":[{"type":"fnTable","props":{"functionId":"player.list"}}],"requiredFunctions":["player.list"]}`
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/component-templates", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 6. 删除自定义
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/component-templates/custom--api", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	// 7. 删除内置 → 500
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/component-templates/fn--player.list", nil))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// loadContractsForTest 直接按 scope 拉取契约（测试辅助）。
func (h *Handler) loadContractsForTest(ctx context.Context, gameID, env string) ([]*model.FunctionContract, error) {
	return h.contractMdl.ListByScope(ctx, gameID, env)
}

// extractViewFromTree 从 tree JSON 中提取第一个节点的 view 类型。
func extractViewFromTree(treeJSON string) string {
	if strings.Contains(treeJSON, `"type":"fnTable"`) {
		return "table"
	}
	if strings.Contains(treeJSON, `"type":"fnFields"`) {
		return "fields"
	}
	return "form"
}
