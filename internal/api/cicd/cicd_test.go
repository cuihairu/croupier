package cicd_test

// CI/CD management-plane tests (OPEN-ISSUES #58): integrations CRUD and credential
// masking, the full trigger chain (fake provider server -> build record landed), webhook
// idempotent write-back and token authentication, and build list filtering. :memory:
// sqlite gets a fresh database each time - model migrations run via AutoMigrate
// (the model is the source of truth).

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

	// Invalid kind / non-http endpoint -> 400
	_, err = svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		Kind: "teamcity", Name: "x", Endpoint: "http://x",
	})
	require.ErrorContains(t, err, "未知")
	_, err = svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		Kind: "jenkins", Name: "x", Endpoint: "ftp://x",
	})
	require.Error(t, err)
	// Missing required provider key -> 400 right at construction (gitlab-ci missing project)
	_, err = svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		Kind: "gitlab-ci", Name: "x", Endpoint: "http://x",
	})
	require.ErrorContains(t, err, "project")

	// Update: an empty token keeps the original credential; the masked string never lands
	upd, err := svcCicd.Update(ctx, &cicd.IntegrationUpdateRequest{
		ID: created.Integration.ID, Name: "主 CI 2", Enabled: boolPtr(false),
	})
	require.NoError(t, err)
	assert.Equal(t, "主 CI 2", upd.Integration.Name)
	assert.False(t, upd.Integration.Enabled)
	assert.Equal(t, "****9999", upd.Integration.TokenMasked)

	// The list carries the closed set of registered kinds
	list, err := svcCicd.List(ctx, &cicd.IntegrationListRequest{GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)
	assert.Equal(t, []string{"generic", "github-actions", "gitlab-ci", "jenkins"}, list.Kinds)

	// Delete cascades to clear builds
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

	// Trigger -> build record landed (queued) -> refresh pulls back running + a
	// normalized ExternalID
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

	// List filter: version hit
	lst, err := svcCicd.ListBuilds(ctx, &cicd.BuildListRequest{Version: "1.2.3", GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, lst.Total)
	// No hit
	lst2, err := svcCicd.ListBuilds(ctx, &cicd.BuildListRequest{Version: "9.9.9"})
	require.NoError(t, err)
	assert.EqualValues(t, 0, lst2.Total)

	// A disabled integration rejects triggering
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

	// Missing token -> 401
	_, err = ingest("")
	require.Error(t, err)
	// Wrong token -> 401
	_, err = ingest("wrong")
	require.Error(t, err)
	// Correct -> success (passed normalized) + finishedAt landed
	first, err := ingest("hook-secret")
	require.NoError(t, err)
	assert.Equal(t, model.CicdBuildSuccess, first.Build.Status)
	assert.Equal(t, "webhook", first.Build.TriggeredBy)
	require.NotNil(t, first.Build.FinishedAt)

	// Duplicate delivery -> idempotent update (status flips to failed), no new row
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

	// Missing externalId -> 400
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

	// Integration without a token -> open endpoint (edge case)
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

func TestCicdService_TestConnection(t *testing.T) {
	// Verify the connection-test success path against a fake server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
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

	resp, err := svcCicd.TestConnection(ctx, created.Integration.ID)
	require.NoError(t, err)
	assert.True(t, resp.OK)
	assert.Contains(t, resp.Message, "HTTP 200")
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

// ----- handler.List / handler.Trigger / handler.Webhook coverage supplement -----
// Only tests handler-layer binding/path errors (no fake provider server needed; the
// full chain is already covered by the service tests)

func setupHandlerCtx(t *testing.T) (*cicd.Service, *gin.Engine) {
	t.Helper()
	ctxSvc, db := setupCtx(t)
	_ = db
	svcCicd := cicd.NewService(ctxSvc)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return svcCicd, r
}

func TestCicdHandler_List_BindError(t *testing.T) {
	// ShouldBindQuery has no failure path for string form fields, so an empty query
	// returning 200 is enough
	svcCicd, r := setupHandlerCtx(t)
	h := cicd.NewHandler(svcCicd)
	r.GET("/cicd/integrations", h.List)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cicd/integrations", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCicdHandler_Trigger_BindAndPathErrors(t *testing.T) {
	svcCicd, r := setupHandlerCtx(t)
	h := cicd.NewHandler(svcCicd)
	r.POST("/cicd/integrations/:id/trigger", h.Trigger)
	// Invalid path id -> 400
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cicd/integrations/abc/trigger", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	// Invalid JSON -> 400
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/cicd/integrations/1/trigger", strings.NewReader("not json"))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestCicdHandler_Webhook_BindAndPathErrors(t *testing.T) {
	svcCicd, r := setupHandlerCtx(t)
	h := cicd.NewHandler(svcCicd)
	r.POST("/cicd/webhooks/:id", h.Webhook)
	// Invalid path id -> 400
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cicd/webhooks/abc", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	// Invalid JSON -> 400
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/cicd/webhooks/1", strings.NewReader("not json"))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)
	// Missing externalId -> create the integration first, then verify payload validation
	// (a non-existent integration would 404 first)
	ctx := context.Background()
	created, err := svcCicd.Create(ctx, &cicd.IntegrationCreateRequest{
		GameID: "demo", Env: "prod", Kind: "generic", Name: "hook-test",
		Endpoint: "http://ci.internal", Token: "hook-secret",
		Extra: map[string]string{"statusUrl": "http://x/{id}"},
	})
	require.NoError(t, err)
	w3 := httptest.NewRecorder()
	body3 := `{"status":"passed"}`
	req3 := httptest.NewRequest(http.MethodPost, "/cicd/webhooks/"+strconv.FormatUint(uint64(created.Integration.ID), 10), strings.NewReader(body3))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-CICD-Token", "hook-secret")
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusBadRequest, w3.Code)
}

func TestCicdHandler_TriggerFull(t *testing.T) {
	svcCicd, r := setupHandlerCtx(t)
	// The jenkins provider really sends outbound HTTP: stand up a local mock server to
	// catch it (same shape as TestCicdService_TriggerFullChain - build 201+Location,
	// queue executable pulls back the build number); after injecting via WithHTTPClient
	// the Endpoint points at the mock.
	mux := http.NewServeMux()
	mux.HandleFunc("/job/pack/build", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/queue/item/3/")
		w.WriteHeader(http.StatusCreated)
	})
	mux.HandleFunc("/queue/item/3/api/json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"executable": map[string]any{"number": 12}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	svcCicd = svcCicd.WithHTTPClient(srv.Client())

	// Create the integration first so the handler has data to query
	_, err := svcCicd.Create(context.Background(), &cicd.IntegrationCreateRequest{
		GameID: "demo", Env: "prod", Kind: "jenkins", Name: "trigger-test",
		Endpoint: srv.URL,
	})
	require.NoError(t, err)

	h := cicd.NewHandler(svcCicd)
	gin.SetMode(gin.TestMode)
	// The route must carry the :id param (same as production routes.go and the
	// BindAndPathErrors precedent in this file): pathID reads c.Param("id"); a literal
	// registration always yields the "invalid path id" 400.
	r.POST("/cicd/integrations/:id/trigger", h.Trigger)
	w := httptest.NewRecorder()
	body := `{"pipeline":"pack","version":"1.2.3"}`
	req := httptest.NewRequest(http.MethodPost, "/cicd/integrations/1/trigger", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var out struct {
		Integration cicd.Integration `json:"integration"`
		Build       cicd.Build       `json:"build"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, model.CicdBuildQueued, out.Build.Status)
	// Query the list again after triggering to verify the record landed
	lst, err := svcCicd.ListBuilds(context.Background(), &cicd.BuildListRequest{Version: "1.2.3", GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, lst.Total)
	// A non-matching version should not appear
	lst2, err := svcCicd.ListBuilds(context.Background(), &cicd.BuildListRequest{Version: "9.9.9", GameID: "demo", Env: "prod"})
	require.NoError(t, err)
	assert.EqualValues(t, 0, lst2.Total)
}

func TestCicdHandler_WebhookFull(t *testing.T) {
	svcCicd, r := setupHandlerCtx(t)
	// Create an integration with a token first so Webhook can validate the token; the
	// generic provider requires at least one of triggerUrl/statusUrl to be configured
	// (this case goes through the webhook entry and never actually calls out; statusUrl
	// exists only to pass Create validation).
	_, err := svcCicd.Create(context.Background(), &cicd.IntegrationCreateRequest{
		GameID: "demo", Env: "prod", Kind: "generic", Name: "hook-full-test",
		Endpoint: "http://ci.internal",
		Token:    "full-secret",
		Extra:    map[string]string{"statusUrl": "http://x/{id}"},
	})
	require.NoError(t, err)

	h := cicd.NewHandler(svcCicd)
	gin.SetMode(gin.TestMode)
	// Same as above: register a route with the :id param so c.Param("id") has a value
	r.POST("/cicd/webhooks/:id", h.Webhook)
	w := httptest.NewRecorder()
	body := `{"externalId":"b-3","status":"passed","version":"3.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/cicd/webhooks/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CICD-Token", "full-secret")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var out struct {
		Build cicd.Build `json:"build"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, model.CicdBuildSuccess, out.Build.Status)
	assert.NotNil(t, out.Build.FinishedAt)
	// Wrong token (the new engine must register the same route; an empty engine only
	// yields 404)
	r2 := gin.New()
	r2.POST("/cicd/webhooks/:id", h.Webhook)
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/cicd/webhooks/1", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-CICD-Token", "wrong-secret")
	r2.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
	// Missing externalId -> 400
	r3 := gin.New()
	r3.POST("/cicd/webhooks/:id", h.Webhook)
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/cicd/webhooks/1", strings.NewReader(`{"status":"passed"}`))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-CICD-Token", "full-secret")
	r3.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusBadRequest, w3.Code)
	// Integration without a token -> fail-closed (AUD-COV-001 P1: deliveries are
	// rejected whenever no token is configured)
	created2, _ := svcCicd.Create(context.Background(), &cicd.IntegrationCreateRequest{
		GameID: "demo", Env: "prod", Kind: "generic", Name: "no-token-e2e",
		Endpoint: "http://ci.internal",
		Extra:    map[string]string{"statusUrl": "http://x/{id}"},
	})
	r4 := gin.New()
	r4.POST("/cicd/webhooks/:id", h.Webhook)
	w4 := httptest.NewRecorder()
	body4 := `{"externalId":"b-4","status":"passed","version":"1.0.0"}`
	req4 := httptest.NewRequest(http.MethodPost, "/cicd/webhooks/"+strconv.FormatUint(uint64(created2.Integration.ID), 10), strings.NewReader(body4))
	req4.Header.Set("Content-Type", "application/json")
	// An integration with no configured token rejects the write-back (fail-closed)
	r4.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusUnauthorized, w4.Code)
}
