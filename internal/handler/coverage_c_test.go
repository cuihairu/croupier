package handler

// C 批覆盖补齐（routes.go）：
//   - RegisterHandlers 的契约→组件模板自动重建装配分支（regen != nil）；
//   - registerProposalRoutes / registerVersioningRoutes 注入的 autoPublish
//     方法值（routes.go:998 / 1018 求出的 page.Service.AutoPublishComposite
//     包装）被真实调用——两者分别是 proposal accept 与 versioning composite
//     保存链在 pages.publishReview=auto 时触发的发布回调，装配测试只求值
//     不调用，故必须走一次端到端 HTTP 请求驱动。发布回调内部失败是允许
//     的（PublishError 语义：不回滚 accept/保存），覆盖点只在回调被调用。
//
//     scope / actor 通过测试 middleware 注入（与生产 scope 中间件同源的
//     svc.WithGameScope / "username" 约定键），无时序依赖。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// scopeCtxMiddlewareC 模拟生产 scope 中间件：注入 game/env scope；可选注入
// username（与 logicutils.CurrentUsername 的约定键一致）。
func scopeCtxMiddlewareC(gameID, env, username string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := svc.WithGameScope(c.Request.Context(), svc.GameScope{GameID: gameID, Env: env})
		if username != "" {
			ctx = context.WithValue(ctx, "username", username)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// newCoverageCDB 打开内存 sqlite 并迁移 proposal/page 链所需表集
// （与 internal/service 契约链测试的表集同源）。
func newCoverageCDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file:covc_handler_"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.FunctionContract{},
		&model.FunctionContractVersion{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageProposalVersion{},
		&model.BlockedProposalIssue{},
		&model.PageSpec{},
		&model.PublishedPageSpec{},
		&model.PageVersion{},
		&model.TermDictionary{},
		&model.Alert{},
	))
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// RegisterHandlers：serverCtx.DB 非 nil 时 buildContractTemplateRegenerator
// 返回非 nil 闭包并被注入（regen != nil 分支）。测试结束重置包级注册器，
// 避免泄漏到其它用例。
func TestCoverageC_RegisterHandlers_WiresContractTemplateRegenerator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defer service.SetContractTemplateRegenerator(nil)

	r := gin.New()
	RegisterHandlers(r, &svc.ServiceContext{DB: newCoverageCDB(t)})
}

// proposal accept 链（pages.publishReview=auto）：accept 落 draft 后调用
// routes.go 装配的 page.Service.AutoPublishComposite 回调（998 行方法值）。
// 回调内部返回的错误只进 PublishError，accept 本身成功。
func TestCoverageC_ProposalAccept_AutoPolicyInvokesPublishHook(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newCoverageCDB(t)

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.query", Enabled: true, Version: "1.0.0",
	}).Error)

	page := map[string]interface{}{
		"pageKey":     "op--covc-auto",
		"type":        "operation",
		"resourceKey": "player",
		"title":       map[string]string{"zh-CN": "覆盖探针"},
		"category":    map[string]interface{}{"key": "player"},
		"operation": map[string]interface{}{
			"form": map[string]interface{}{
				"jsonSchema": json.RawMessage(`{"type":"object","properties":{"playerId":{"type":"string"}}}`),
				"layout":     "vertical",
			},
		},
		"bindings": []map[string]interface{}{
			{
				"id": "query", "functionId": "player.query", "usage": "query",
				"execution": map[string]interface{}{"mode": "sync"},
			},
		},
	}
	specJSON, err := json.Marshal(page)
	require.NoError(t, err)
	require.NoError(t, model.NewPageProposalModel(db).UpsertProposal(context.Background(), &model.PageProposal{
		GameID:      "demo-game",
		Env:         "development",
		ProposalKey: "operation:op--covc-auto",
		PageKey:     "op--covc-auto",
		PageType:    "operation",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      dbenum.ProposalStatusPending,
		PageSpec:    specJSON,
	}))

	serverCtx := &svc.ServiceContext{DB: db}
	serverCtx.Config.Pages.PublishReview = "auto"
	// 发布回调（page.Service）按模型字段寻路，与生产装配同构补齐
	serverCtx.PageSpecModel = model.NewPageSpecModel(db)
	r := gin.New()
	g := r.Group("/p") // 前缀非 "/"：gin 对 "" 与 "/" 路由在同根组会判重
	g.Use(scopeCtxMiddlewareC("demo-game", "development", ""))
	registerProposalRoutes(g, serverCtx)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/p/operation:op--covc-auto/accept", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "accept 应成功: %s", w.Body.String())
	require.Contains(t, w.Body.String(), `"published"`, "响应应携带 auto 接续发布结果")
}

// versioning composite 保存链（pages.publishReview=auto）：composite 提案
// 创建成功后调用 routes.go 装配的 page.Service.AutoPublishComposite 回调
// （1018 行方法值）。发布失败不回滚保存——PublishError 随响应带回。
func TestCoverageC_VersioningComposite_AutoPolicyInvokesPublishHook(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newCoverageCDB(t)

	serverCtx := &svc.ServiceContext{DB: db}
	serverCtx.Config.Pages.PublishReview = "auto"
	// 发布回调（page.Service）按模型字段寻路，与生产装配同构补齐
	serverCtx.PageSpecModel = model.NewPageSpecModel(db)
	r := gin.New()
	g := r.Group("/v")
	g.Use(scopeCtxMiddlewareC("demo-game", "development", ""))
	registerVersioningRoutes(g, serverCtx)

	// 生成区块必须有可物化契约（纯 static 输入会被生成器以「空页面」拒绝）：
	// 经公开的 ContractService 物化一份最小契约。
	contractSvc := service.NewContractService(db)
	require.NoError(t, contractSvc.RebuildContractFromFunctionMeta(context.Background(),
		"demo-game", "development", "agent-covc", spec.FunctionContractInput{
			ID: "player.get", Resource: "player", Capability: "item_query",
			Execution: "sync", Enabled: true,
			InputSchema:  `{"type":"object","properties":{"id":{"type":"string"}}}`,
			OutputSchema: `{"type":"object","properties":{"player":{"type":"object"}}}`,
		}))

	body := map[string]interface{}{
		"pageKey": "covc-composite",
		"sections": []map[string]interface{}{
			{
				"key":    "intro",
				"static": true,
				"form": map[string]interface{}{
					"jsonSchema": json.RawMessage(`{"type":"object","properties":{"note":{"type":"string"}}}`),
					"layout":     "vertical",
				},
			},
			{"functionId": "player.get", "view": "fields"},
		},
	}
	blob, err := json.Marshal(body)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v/pages/composite", strings.NewReader(string(blob)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "composite 保存应成功: %s", w.Body.String())
	require.Contains(t, w.Body.String(), `"proposalKey"`, "响应应携带提案标识")
}
