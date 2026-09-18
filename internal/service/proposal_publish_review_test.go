package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// T10 发布分级（扩展）：auto 策略下 AcceptProposal 落 draft 后调用
// autoPublish，outcome.Published=true；draft 与提案 accepted 照常落库。
func TestAcceptProposal_AutoPolicyPublishes(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svcInstance := NewProposalService(db)

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.query", Enabled: true, Version: "1.0.0",
	}).Error)
	require.NoError(t, svcInstance.proposalModel.UpsertProposal(ctx,
		mustProposal(buildOperationProposal("op--auto-ok", "player.query", nil))))

	var calledWith []string
	svcInstance.SetPublishReviewHooks(
		func(env string) string {
			assert.Equal(t, "development", env)
			return config.PublishReviewAuto
		},
		func(c context.Context, gameID, env, proposalKey string) error {
			calledWith = []string{gameID, env, proposalKey}
			return nil
		},
	)

	outcome, err := svcInstance.AcceptProposal(ctx, "demo-game", "development", "operation:op--auto-ok")
	require.NoError(t, err)
	assert.True(t, outcome.Published)
	assert.Empty(t, outcome.PublishError)
	require.Len(t, calledWith, 3)
	assert.Equal(t, "demo-game", calledWith[0])
	assert.Equal(t, "development", calledWith[1])
	assert.Equal(t, "operation:op--auto-ok", calledWith[2])

	// accept 本体语义不因接续发布改变：draft 落库、提案 accepted。
	draft, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", "op--auto-ok")
	require.NoError(t, err)
	assert.Equal(t, "draft", draft.Status)
	stored, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:op--auto-ok")
	require.NoError(t, err)
	assert.Equal(t, dbenum.ProposalStatusAccepted, stored.Status)
}

// T10 发布分级（扩展）：required 策略维持现状——accept 只落 draft，
// autoPublish 不被调用。
func TestAcceptProposal_RequiredPolicyKeepsManualFlow(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svcInstance := NewProposalService(db)

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.query", Enabled: true, Version: "1.0.0",
	}).Error)
	require.NoError(t, svcInstance.proposalModel.UpsertProposal(ctx,
		mustProposal(buildOperationProposal("op--required", "player.query", nil))))

	called := false
	svcInstance.SetPublishReviewHooks(
		func(env string) string { return config.PublishReviewRequired },
		func(context.Context, string, string, string) error {
			called = true
			return nil
		},
	)

	outcome, err := svcInstance.AcceptProposal(ctx, "demo-game", "development", "operation:op--required")
	require.NoError(t, err)
	assert.False(t, outcome.Published)
	assert.Empty(t, outcome.PublishError)
	assert.False(t, called, "required 策略不应触发自动发布")

	draft, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", "op--required")
	require.NoError(t, err)
	assert.Equal(t, "draft", draft.Status)
}

// T10 发布分级（扩展）：auto 发布失败不回滚 accept——outcome 带回
// PublishError，draft 与提案 accepted 保留，可走手动发布重试。
func TestAcceptProposal_AutoPublishErrorKeepsDraft(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svcInstance := NewProposalService(db)

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.query", Enabled: true, Version: "1.0.0",
	}).Error)
	require.NoError(t, svcInstance.proposalModel.UpsertProposal(ctx,
		mustProposal(buildOperationProposal("op--auto-fail", "player.query", nil))))

	svcInstance.SetPublishReviewHooks(
		func(env string) string { return config.PublishReviewAuto },
		func(context.Context, string, string, string) error {
			return errors.New("quality gate rejected")
		},
	)

	outcome, err := svcInstance.AcceptProposal(ctx, "demo-game", "development", "operation:op--auto-fail")
	require.NoError(t, err, "发布失败不回滚 accept，accept 本身成功")
	assert.False(t, outcome.Published)
	assert.Contains(t, outcome.PublishError, "quality gate rejected")

	draft, err := model.NewPageSpecModel(db).FindByScopeAndPageKey(ctx, "demo-game", "development", "op--auto-fail")
	require.NoError(t, err, "draft 保留供手动发布")
	assert.Equal(t, "draft", draft.Status)
	stored, err := model.NewPageProposalModel(db).FindByScopeAndKey(ctx, "demo-game", "development", "operation:op--auto-fail")
	require.NoError(t, err)
	assert.Equal(t, dbenum.ProposalStatusAccepted, stored.Status)
}

// T10 发布分级（扩展）：hooks 未注入（缺省装配）时 accept 维持「只落
// draft」现状。
func TestAcceptProposal_WithoutHooksKeepsDefault(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svcInstance := NewProposalService(db)

	require.NoError(t, db.Create(&model.FunctionContract{
		GameID: "demo-game", Env: "development", FunctionID: "player.query", Enabled: true, Version: "1.0.0",
	}).Error)
	require.NoError(t, svcInstance.proposalModel.UpsertProposal(ctx,
		mustProposal(buildOperationProposal("op--default", "player.query", nil))))

	outcome, err := svcInstance.AcceptProposal(ctx, "demo-game", "development", "operation:op--default")
	require.NoError(t, err)
	assert.False(t, outcome.Published)
	assert.Empty(t, outcome.PublishError)
}

// T10 发布分级（扩展）：handler 响应契约——auto 成功带 published:true；
// auto 发布失败带 publishError；required 只回 published:false 且不带
// publishError 字段（对齐 versioning composite 响应模式）。
func TestHandler_AcceptProposal_PublishReviewResponse(t *testing.T) {
	type respPayload struct {
		Message      string `json:"message"`
		Published    bool   `json:"published"`
		PublishError string `json:"publishError"`
	}
	seed := func(t *testing.T, db *gorm.DB, pageKey string) {
		t.Helper()
		require.NoError(t, db.Create(&model.FunctionContract{
			GameID: "demo-game", Env: "development", FunctionID: "player.query", Enabled: true, Version: "1.0.0",
		}).Error)
		svcInstance := NewProposalService(db)
		require.NoError(t, svcInstance.proposalModel.UpsertProposal(proposalTestContext(),
			mustProposal(buildOperationProposal(pageKey, "player.query", nil))))
	}

	t.Run("auto publish success", func(t *testing.T) {
		db := setupTestDB(t)
		seed(t, db, "op--resp-ok")
		svcInstance := NewProposalService(db)
		svcInstance.SetPublishReviewHooks(
			func(env string) string { return config.PublishReviewAuto },
			func(context.Context, string, string, string) error { return nil },
		)
		router := newAcceptReviewRouter(t, svcInstance)

		rec := doAcceptReviewRequest(router, "operation:op--resp-ok")
		require.Equal(t, http.StatusOK, rec.Code)
		var payload respPayload
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		assert.True(t, payload.Published)
		assert.Empty(t, payload.PublishError)
	})

	t.Run("auto publish failure carries publishError", func(t *testing.T) {
		db := setupTestDB(t)
		seed(t, db, "op--resp-fail")
		svcInstance := NewProposalService(db)
		svcInstance.SetPublishReviewHooks(
			func(env string) string { return config.PublishReviewAuto },
			func(context.Context, string, string, string) error {
				return errors.New("quality gate rejected")
			},
		)
		router := newAcceptReviewRouter(t, svcInstance)

		rec := doAcceptReviewRequest(router, "operation:op--resp-fail")
		require.Equal(t, http.StatusOK, rec.Code, "发布失败不改变 accept 成功语义")
		var payload respPayload
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
		assert.False(t, payload.Published)
		assert.Contains(t, payload.PublishError, "quality gate rejected")
	})

	t.Run("required omits publishError field", func(t *testing.T) {
		db := setupTestDB(t)
		seed(t, db, "op--resp-manual")
		svcInstance := NewProposalService(db)
		svcInstance.SetPublishReviewHooks(
			func(env string) string { return config.PublishReviewRequired },
			func(context.Context, string, string, string) error { return nil },
		)
		router := newAcceptReviewRouter(t, svcInstance)

		rec := doAcceptReviewRequest(router, "operation:op--resp-manual")
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"published":false`)
		assert.NotContains(t, rec.Body.String(), "publishError")
	})
}

func newAcceptReviewRouter(t *testing.T, svcInstance *ProposalService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewProposalHandler(svcInstance)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		ctx := svc.WithGameScope(c.Request.Context(), svc.GameScope{
			GameID: "demo-game",
			Env:    "development",
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.POST("/api/v1/proposals/:proposalKey/accept", handler.AcceptProposal)
	return router
}

func doAcceptReviewRequest(router *gin.Engine, proposalKey string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/proposals/"+proposalKey+"/accept", nil))
	return rec
}
