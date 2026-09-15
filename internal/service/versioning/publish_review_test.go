package versioning

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/service"
	"github.com/cuihairu/croupier/internal/svc"
)

func compositeSectionsForReview() []service.CompositeSectionRequest {
	return []service.CompositeSectionRequest{
		{FunctionID: "player.get", View: "fields", Title: "玩家信息"},
		{FunctionID: "order.list", View: "table", Title: "订单", RefreshOn: []string{"player.get"}},
	}
}

// T10 发布分级：auto 策略下 composite 保存成功后调用 autoPublish，
// outcome.Published=true；提案本体照常落库。
func TestCreateCompositePage_AutoPolicyPublishes(t *testing.T) {
	db := setupTestDB(t)
	seedCompositeContractsV9(t, db)
	ctx := context.Background()

	svc := NewService(db)
	var calledWith []string
	svc.SetPublishReviewHooks(
		func(env string) string {
			assert.Equal(t, "development", env)
			return config.PublishReviewAuto
		},
		func(c context.Context, gameID, env, proposalKey string) error {
			calledWith = []string{gameID, env, proposalKey}
			return nil
		},
	)

	outcome, err := svc.CreateCompositePage(ctx, "demo-game", "development", "composite--auto", compositeSectionsForReview(), nil)
	require.NoError(t, err)
	require.NotNil(t, outcome.Proposal)
	assert.True(t, outcome.Published)
	assert.Empty(t, outcome.PublishError)
	require.Len(t, calledWith, 3)
	assert.Equal(t, "demo-game", calledWith[0])
	assert.Equal(t, "development", calledWith[1])
	assert.Equal(t, outcome.Proposal.ProposalKey, calledWith[2])

	// 提案本体照常落库（pending，人工链可追溯）
	stored, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", outcome.Proposal.ProposalKey)
	require.NoError(t, err)
	assert.Equal(t, dbenum.ProposalStatusPending, stored.Status)
}

// T10 发布分级：required 策略维持现状——只建提案，autoPublish 不被调用。
func TestCreateCompositePage_RequiredPolicyKeepsManualFlow(t *testing.T) {
	db := setupTestDB(t)
	seedCompositeContractsV9(t, db)

	svc := NewService(db)
	called := false
	svc.SetPublishReviewHooks(
		func(env string) string { return config.PublishReviewRequired },
		func(context.Context, string, string, string) error {
			called = true
			return nil
		},
	)

	outcome, err := svc.CreateCompositePage(context.Background(), "demo-game", "development", "composite--required", compositeSectionsForReview(), nil)
	require.NoError(t, err)
	assert.False(t, outcome.Published)
	assert.Empty(t, outcome.PublishError)
	assert.False(t, called, "required 策略不应触发自动发布")
}

// T10 发布分级：auto 发布失败不回滚保存——outcome 带回 PublishError，
// 提案仍在，CreateCompositePage 本身不报错（前端降级人工链）。
func TestCreateCompositePage_AutoPublishErrorKeepsProposal(t *testing.T) {
	db := setupTestDB(t)
	seedCompositeContractsV9(t, db)
	ctx := context.Background()

	svc := NewService(db)
	svc.SetPublishReviewHooks(
		func(env string) string { return config.PublishReviewAuto },
		func(context.Context, string, string, string) error {
			return errors.New("quality gate rejected")
		},
	)

	outcome, err := svc.CreateCompositePage(ctx, "demo-game", "development", "composite--failed", compositeSectionsForReview(), nil)
	require.NoError(t, err, "发布失败不回滚保存，保存本身成功")
	assert.False(t, outcome.Published)
	assert.Contains(t, outcome.PublishError, "quality gate rejected")

	_, err = model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", outcome.Proposal.ProposalKey)
	require.NoError(t, err, "提案保留供人工链重试")
}

// T10 发布分级：hooks 未注入（缺省装配）时保存链维持「只建提案」现状。
func TestCreateCompositePage_WithoutHooksKeepsDefault(t *testing.T) {
	db := setupTestDB(t)
	seedCompositeContractsV9(t, db)

	outcome, err := NewService(db).CreateCompositePage(context.Background(), "demo-game", "development", "composite--default", compositeSectionsForReview(), nil)
	require.NoError(t, err)
	assert.False(t, outcome.Published)
	assert.Empty(t, outcome.PublishError)
}

// T10 发布分级：handler 响应契约——auto 成功带 published:true；auto 发布
// 失败带 publishError（无 published 字段歧义：false）；required 只回
// published:false 且不带 publishError 字段。
func TestHandler_CreateCompositePage_PublishReviewResponse(t *testing.T) {
	type respPayload struct {
		ProposalKey  string `json:"proposalKey"`
		Published    bool   `json:"published"`
		PublishError string `json:"publishError"`
	}
	compositeBody := func(pageKey string) string {
		return `{"pageKey":"` + pageKey + `","sections":[
			{"functionId":"player.get","view":"fields","title":"玩家信息"},
			{"functionId":"order.list","view":"table","title":"订单","refreshOn":["player.get"]}
		]}`
	}

	t.Run("auto publish success", func(t *testing.T) {
		db := setupTestDB(t)
		seedCompositeContractsV9(t, db)
		svcInstance := NewService(db)
		svcInstance.SetPublishReviewHooks(
			func(env string) string { return config.PublishReviewAuto },
			func(context.Context, string, string, string) error { return nil },
		)
		router := newPublishReviewRouter(t, svcInstance)

		rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite", compositeBody("composite--resp-ok"))
		require.Equal(t, http.StatusOK, rec.Code)
		var payload respPayload
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		assert.True(t, payload.Published)
		assert.Empty(t, payload.PublishError)
	})

	t.Run("auto publish failure carries publishError", func(t *testing.T) {
		db := setupTestDB(t)
		seedCompositeContractsV9(t, db)
		svcInstance := NewService(db)
		svcInstance.SetPublishReviewHooks(
			func(env string) string { return config.PublishReviewAuto },
			func(context.Context, string, string, string) error { return errors.New("quality gate rejected") },
		)
		router := newPublishReviewRouter(t, svcInstance)

		rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite", compositeBody("composite--resp-fail"))
		require.Equal(t, http.StatusOK, rec.Code, "发布失败不改变保存成功语义")
		var payload respPayload
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		assert.False(t, payload.Published)
		assert.Contains(t, payload.PublishError, "quality gate rejected")
	})

	t.Run("required omits publishError field", func(t *testing.T) {
		db := setupTestDB(t)
		seedCompositeContractsV9(t, db)
		svcInstance := NewService(db)
		svcInstance.SetPublishReviewHooks(
			func(env string) string { return config.PublishReviewRequired },
			func(context.Context, string, string, string) error { return nil },
		)
		router := newPublishReviewRouter(t, svcInstance)

		rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite", compositeBody("composite--resp-manual"))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"published":false`)
		assert.NotContains(t, rec.Body.String(), "publishError")
	})
}

func newPublishReviewRouter(t *testing.T, svcInstance *Service) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewHandler(svcInstance)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := svc.WithGameScope(c.Request.Context(), svc.GameScope{
			GameID: c.GetHeader("X-Game-ID"),
			Env:    c.GetHeader("X-Env"),
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.POST("/api/versioning/pages/composite", handler.CreateCompositePage)
	return router
}
