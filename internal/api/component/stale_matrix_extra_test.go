// 补齐 component 包分支：computeStaleKeys 全能力矩阵、Regenerate 错误
// 传播、seed-demo-constants 缺表故障。
package component

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

var staleDBSeq atomic.Int64

func newStaleDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:stalemtx%d?mode=memory&cache=shared", staleDBSeq.Add(1))
	db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.ComponentTemplate{},
		&model.FunctionContract{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
	))
	return db
}

func seedStaleContracts(t *testing.T, db *gorm.DB) {
	t.Helper()
	contracts := []*model.FunctionContract{
		{GameID: "demo", Env: "dev", FunctionID: "player.list", ResourceKey: "player", Capability: dbenum.CapabilityCollectionQuery, Enabled: true,
			InputSchema:  model.JSON(`{"type":"object","properties":{"kw":{"type":"string"}}}`),
			OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array"}}}`)},
		{GameID: "demo", Env: "dev", FunctionID: "player.get", ResourceKey: "player", Capability: dbenum.CapabilityItemQuery, Enabled: true,
			InputSchema:  model.JSON(`{"type":"object","properties":{"id":{"type":"string"}}}`),
			OutputSchema: model.JSON(`{"type":"object"}`)},
		{GameID: "demo", Env: "dev", FunctionID: "player.create", ResourceKey: "player", Capability: dbenum.CapabilityCreate, Enabled: true},
		{GameID: "demo", Env: "dev", FunctionID: "player.update", ResourceKey: "player", Capability: dbenum.CapabilityUpdate, Enabled: true},
		{GameID: "demo", Env: "dev", FunctionID: "player.delete", ResourceKey: "player", Capability: dbenum.CapabilityDelete, Enabled: true},
		// 触发各 continue/回退分支：空函数 ID、空资源键、未知能力
		{GameID: "demo", Env: "dev", FunctionID: " ", Capability: dbenum.CapabilityAction, Enabled: true},
		{GameID: "demo", Env: "dev", FunctionID: "mystery.fn", Capability: dbenum.CapabilityUnknown, Enabled: true},
		{GameID: "demo", Env: "dev", FunctionID: "search.log", Capability: dbenum.CapabilityCollectionQuery, Enabled: true,
			InputSchema:  model.JSON(`{"type":"object","properties":{"kw":{"type":"string"}}}`),
			OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array"}}}`)},
	}
	for i := range contracts {
		require.NoError(t, db.Create(contracts[i]).Error)
	}
}

func loadStaleContracts(t *testing.T, db *gorm.DB) []*model.FunctionContract {
	t.Helper()
	var contracts []*model.FunctionContract
	require.NoError(t, db.Where("game_id = ? AND env = ?", "demo", "dev").Find(&contracts).Error)
	require.NotEmpty(t, contracts)
	return contracts
}

// 各能力契约 + builtin 模板 → computeStaleKeys 全分支（能力 switch、
// resource 分组、CRUD 组合、stale 标记与新鲜生成不误标）。
func TestComputeStaleKeysFullMatrix(t *testing.T) {
	db := newStaleDB(t)
	seedStaleContracts(t, db)
	h := NewHandler(model.NewComponentTemplateModel(db), model.NewFunctionContractModel(db))
	contracts := loadStaleContracts(t, db)
	require.NoError(t, h.RegenerateFromContracts(context.Background(), contracts))

	items, total, err := h.model.List(context.Background(), model.ComponentTemplateListOptions{
		PaginationOptions: model.PaginationOptions{Page: 1, PageSize: 100},
	})
	require.NoError(t, err)
	require.Positive(t, total)

	// 新鲜生成 → 不应有 stale
	stale := h.computeStaleKeys("demo", "dev", items)
	assert.Empty(t, stale, "刚生成的 builtin 模板不应 stale")

	// 篡改一个 builtin 模板 tree → stale
	tampered := items[0]
	require.NoError(t, db.Model(&model.ComponentTemplate{}).Where("key = ?", tampered.Key).
		Update("tree", model.JSON(`[{"id":"x","type":"text","props":{}}]`)).Error)
	items2, _, err := h.model.List(context.Background(), model.ComponentTemplateListOptions{
		PaginationOptions: model.PaginationOptions{Page: 1, PageSize: 100},
	})
	require.NoError(t, err)
	stale2 := h.computeStaleKeys("demo", "dev", items2)
	assert.True(t, stale2[tampered.Key], "被篡改的模板应标记 stale")
}

// List 带 scope 头 → computeStaleKeys 接线（handler 层 200）。
func TestListWithStaleHeaders(t *testing.T) {
	db := newStaleDB(t)
	seedStaleContracts(t, db)
	h := NewHandler(model.NewComponentTemplateModel(db), model.NewFunctionContractModel(db))
	contracts := loadStaleContracts(t, db)
	require.NoError(t, h.RegenerateFromContracts(context.Background(), contracts))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/api/v1/component-templates"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/component-templates", nil)
	req.Header.Set("X-Game-ID", "demo")
	req.Header.Set("X-Env", "dev")
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
	var resp struct {
		Items []TemplateDTO `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Items)
}

// seed-demo-constants 缺表：Create 失败 → 500 错误响应。
func TestSeedDemoConstantsCreateFailure(t *testing.T) {
	db := setupV4DB(t)
	require.NoError(t, db.Migrator().DropTable(&model.ComponentTemplate{}))
	h := NewHandler(model.NewComponentTemplateModel(db), nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/api/v1/component-templates"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/component-templates/seed-demo-constants", nil))
	assert.Equal(t, 500, w.Code)
}

// Regenerate 时 builtin 模板写入失败（缺表）：生成器逐条告警但整体不中断
// （设计行为：单条失败只 WarnContext，Regenerate 返回 200）。
func TestRegenerateContractError(t *testing.T) {
	db := setupV4DB(t)
	seedStaleContracts(t, db)
	h := NewHandler(model.NewComponentTemplateModel(db), model.NewFunctionContractModel(db))
	require.NoError(t, db.Migrator().DropTable(&model.ComponentTemplate{}))
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/api/v1/component-templates"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/component-templates/regenerate", nil))
	assert.Equal(t, 200, w.Code, "单条写入失败只告警，整体不中断")
}
