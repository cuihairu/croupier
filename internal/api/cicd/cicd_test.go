package cicd_test

// CI/CD 管理面测试（OPEN-ISSUES #58）：integrations CRUD 与凭据掩码、
// 触发全链（provider 假服务器 → 构建记录落库）、webhook 幂等回写与令牌
// 鉴权、构建列表过滤。:memory: sqlite 每次独立建库——模型迁移走
// AutoMigrate（model 即真值）。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/api/cicd"
	_ "github.com/cuihairu/croupier/internal/cicd/providers"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCtx(t *testing.T) (*svc.ServiceContext, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.CicdIntegration{}, &model.CicdBuild{}))
	return &svc.ServiceContext{
		DB:                   db,
		CicdIntegrationModel: model.NewCicdIntegrationModel(db),
		CicdBuildModel:       model.NewCicdBuildModel(db),
	}, db
}

func boolPtr(b bool) *bool { return &b }

func TestCicdService_IntegrationCRUDAndMasking(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	svcCicd := cicd.NewService(ctxSvc)
	ctx := context.Background()

	created, err := svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		GameID: "demo", Env: "prod", Kind: "jenkins",
		Name: "主 CI", Endpoint: "https://ci.example.com",
		Token: "super-secret-token-9999",
	})
	require.NoError(t, err)
	assert.True(t, created.Integration.TokenSet)
	assert.Equal(t, "****9999", created.Integration.TokenMasked)
	assert.NotContains(t, created.Integration.TokenMasked, "super-secret")

	// 非法 kind / 非 http endpoint → 400
	_, err = svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		Kind: "teamcity", Name: "x", Endpoint: "http://x",
	})
	require.ErrorContains(t, err, "未知")
	_, err = svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		Kind: "jenkins", Name: "x", Endpoint: "ftp://x",
	})
	require.Error(t, err)
	// provider 必填键缺失 → 构造即 400（gitlab-ci 缺 project）
	_, err = svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		Kind: "gitlab-ci", Name: "x", Endpoint: "http://x",
	})
	require.ErrorContains(t, err, "project")

	// 更新：空 token 保留原凭据；掩码串不落库
	upd, err := svcCicd.Update(ctx, &cicd.IntegrationUpdateRequest{
		ID: created.Integration.ID, Name: "主 CI 2", Enabled: boolPtr(false),
	})
	require.NoError(t, err)
	assert.Equal(t, "主 CI 2", upd.Integration.Name)
	assert.False(t, upd.Integration.Enabled)
	assert.Equal(t, "****9999", upd.Integration.TokenMasked)

	// 列表含注册类型闭集
	list, err := svcCicd.List(ctx, &cicd.IntegrationListRequest{GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, []string{"generic", "github-actions", "gitlab-ci", "jenkins"}, list.Kinds)

	// 删除级联清构建
	require.NoError(t, svcCicd.Delete(ctx, created.Integration.ID))
	list2, err := svcCicd.List(ctx, &cicd.IntegrationListRequest{})
	require.NoError(t, err)
	assert.Empty(t, list2.Items)
}

func TestCicdService_TriggerFullChain(t *testing.T) {
	var triggerBody map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/job/pack/build", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/queue/item/3/")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/queue/item/3/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"executable": map[string]any{"number": 12}})
	})
	mux.HandleFunc("/job/pack/12/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"number": 12, "building": true, "url": "http://x/job/pack/12/",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctxSvc, _ := setupCtx(t)
	svcCicd := cicd.NewService(ctxSvc).WithHTTPClient(srv.Client())
	ctx := context.Background()
	created, err := svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		GameID: "demo", Env: "prod", Kind: "jenkins", Name: "ci",
		Endpoint: srv.URL,
	})
	require.NoError(t, err)

	// 触发 → 构建记录落库（queued）→ refresh 拉回 running + 规范化 ExternalID
	tr, err := svcCicd.Trigger(ctx, created.Integration.ID, &cicd.TriggerRequest{
		Pipeline: "pack", Version: "1.2.3",
	})
	require.NoError(t, err)
	assert.Equal(t, model.CicdBuildQueued, tr.Build.Status)
	assert.Equal(t, "1.2.3", tr.Build.Version)
	assert.Equal(t, "api", tr.Build.TriggeredBy)
	assert.NotEmpty(t, tr.Build.ExternalID)
	_ = triggerBody

	rb, err := svcCicd.RefreshBuild(ctx, tr.Build.ID)
	require.NoError(t, err)
	assert.Equal(t, model.CicdBuildRunning, rb.Build.Status)
	assert.Contains(t, rb.Build.ExternalID, "12")

	// 列表过滤：version 命中
	lst, err := svcCicd.ListBuilds(ctx, &cicd.BuildListRequest{Version: "1.2.3", GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, lst.Total)
	// 未命中
	lst2, err := svcCicd.ListBuilds(ctx, &cicd.BuildListRequest{Version: "9.9.9"})
	require.NoError(t, err)
	assert.EqualValues(t, 0, lst2.Total)

	// 已停用接入拒绝触发
	_, err = svcCicd.Update(ctx, &cicd.IntegrationUpdateRequest{ID: created.Integration.ID, Enabled: boolPtr(false)})
	require.NoError(t, err)
	_, err = svcCicd.Trigger(ctx, created.Integration.ID, &cicd.TriggerRequest{Pipeline: "pack"})
	require.ErrorContains(t, err, "停用")
}

func TestCicdService_WebhookIngest(t *testing.T) {
	ctxSvc, _ := setupCtx(t)
	svcCicd := cicd.NewService(ctxSvc)
	ctx := context.Background()
	created, err := svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		GameID: "demo", Env: "prod", Kind: "generic", Name: "self-ci",
		Endpoint: "http://ci.internal",
		Token:    "hook-secret",
		Extra:    map[string]string{"statusUrl": "http://ci.internal/s/{id}"},
	})
	require.NoError(t, err)

	ingest := func(token string) (*cicd.WebhookResponse, error) {
		h := cicd.NewHandler(svcCicd)
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/cicd/webhooks/:id", h.Webhook)
		w := httptest.NewRecorder()
		body, _ := json.Marshal(map[string]any{
			"externalId": "b-1", "status": "passed", "version": "2.0.0",
			"artifactUrl": "http://ci.internal/a.zip", "checksum": "sha256:ff",
			"webUrl": "http://ci.internal/b/1",
		})
		req := httptest.NewRequest(http.MethodPost, "/cicd/webhooks/"+itoa(created.Integration.ID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("X-CICD-Token", token)
		}
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			return nil, errFromCode(w.Code)
		}
		var out struct {
			Build cicd.Build `json:"build"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return &cicd.WebhookResponse{Build: out.Build}, nil
	}

	// 令牌缺失 → 401
	_, err = ingest("")
	require.Error(t, err)
	// 令牌错误 → 401
	_, err = ingest("wrong")
	require.Error(t, err)
	// 正确 → success（passed 归一）+ finishedAt 落
	first, err := ingest("hook-secret")
	require.NoError(t, err)
	assert.Equal(t, model.CicdBuildSuccess, first.Build.Status)
	assert.Equal(t, "webhook", first.Build.TriggeredBy)
	require.NotNil(t, first.Build.FinishedAt)

	// 重复投递 → 幂等更新（状态翻转 failed），不新建行
	mut := func(status string) *cicd.WebhookResponse {
		h := cicd.NewHandler(svcCicd)
		gin.SetMode(gin.TestMode)
		r := gin.New()
		r.POST("/cicd/webhooks/:id", h.Webhook)
		w := httptest.NewRecorder()
		body, _ := json.Marshal(map[string]any{"externalId": "b-1", "status": status})
		req := httptest.NewRequest(http.MethodPost, "/cicd/webhooks/"+itoa(created.Integration.ID), bytes.NewReader(body))
		req.Header.Set("X-CICD-Token", "hook-secret")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		var out struct {
			Build cicd.Build `json:"build"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return &cicd.WebhookResponse{Build: out.Build}
	}
	second := mut("failed")
	assert.Equal(t, model.CicdBuildFailed, second.Build.Status)
	assert.Equal(t, first.Build.ID, second.Build.ID, "externalId 相同应幂等更新")

	// externalId 缺失 → 400
	h := cicd.NewHandler(svcCicd)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/cicd/webhooks/:id", h.Webhook)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cicd/webhooks/"+itoa(created.Integration.ID),
		stringsReader(`{"status":"success"}`))
	req.Header.Set("X-CICD-Token", "hook-secret")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 无令牌接入 → 开放端点（边界）
	opened, err := svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		GameID: "demo", Env: "prod", Kind: "generic", Name: "no-token",
		Endpoint: "http://ci.internal",
		Extra:    map[string]string{"statusUrl": "http://x/{id}"},
	})
	require.NoError(t, err)
	assert.False(t, opened.Integration.TokenSet)
}

func TestMaskToken(t *testing.T) {
	assert.Empty(t, model.MaskToken("  "))
	assert.Equal(t, "****", model.MaskToken("abc"))
	assert.Equal(t, "****9999", model.MaskToken("super-secret-9999"))
}

func itoa(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}

func errFromCode(code int) error {
	return fmt.Errorf("http %d", code)
}

func stringsReader(s string) *strings.Reader {
	return strings.NewReader(s)
}
