package rate_limit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupTestDB builds a fresh in-memory SQLite database with the rate-limit
// tables migrated. Mirrors the pattern used by internal/api/task tests.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

func setupSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db := setupTestDB(t)
	return &svc.ServiceContext{
		DB:             db,
		RateLimitModel: model.NewRateLimitModel(db),
	}
}

// newTestContext builds a gin test context backed by a response recorder.
func newTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected status %d, got %d body=%s", want, rec.Code, rec.Body.String())
	}
}

// assertErrorShape verifies the body is a unified error object
// {"error": "...", "message": "..."}.
func assertErrorShape(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	body := unmarshalBody(t, rec)
	errCode, ok := body["error"].(string)
	require.True(t, ok && errCode != "", "response should have a non-empty 'error' field, body=%s", rec.Body.String())
	msg, ok := body["message"].(string)
	require.True(t, ok && msg != "", "response should have a non-empty 'message' field, body=%s", rec.Body.String())
}

func unmarshalBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	out := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), "response body is not a JSON object: %s", rec.Body.String())
	return out
}

func TestService_List_Empty(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.List(context.Background(), &RateLimitsListRequest{})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.Items)
}

func TestService_UpsertAndGet_RoundTrip(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	upsertResp, err := svc.Upsert(context.Background(), &RateLimitUpsertRequest{
		Name:     "player-cap",
		Resource: "function",
		Limit:    10,
		Window:   60,
		Action:   "reject",
		Rules:    map[string]interface{}{"gameId": "demo"},
	})
	require.NoError(t, err)
	require.NotNil(t, upsertResp)
	assert.Equal(t, "player-cap", upsertResp.Name)
	assert.Equal(t, "function", upsertResp.Resource)
	assert.Equal(t, 10, upsertResp.Limit)
	assert.Equal(t, "reject", upsertResp.Action)

	getResp, err := svc.Get(context.Background(), &RateLimitGetRequest{ID: upsertResp.Id})
	require.NoError(t, err)
	require.NotNil(t, getResp)
	assert.Equal(t, upsertResp.Id, getResp.Id)
	assert.Equal(t, "player-cap", getResp.Name)
}

func TestService_Get_NotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.Get(context.Background(), &RateLimitGetRequest{ID: "missing"})
	require.Error(t, err)
	assert.Nil(t, resp)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusNotFound, codeErr.Code)
}

func TestService_Get_EmptyID(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.Get(context.Background(), &RateLimitGetRequest{ID: "  "})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Upsert_ValidationErrors(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	tests := []struct {
		name string
		req  RateLimitUpsertRequest
	}{
		{"empty name", RateLimitUpsertRequest{Resource: "function", Limit: 1, Window: 1, Action: "reject"}},
		{"empty resource", RateLimitUpsertRequest{Name: "n", Limit: 1, Window: 1, Action: "reject"}},
		{"zero limit", RateLimitUpsertRequest{Name: "n", Resource: "function", Limit: 0, Window: 1, Action: "reject"}},
		{"zero window", RateLimitUpsertRequest{Name: "n", Resource: "function", Limit: 1, Window: 0, Action: "reject"}},
		{"bad action", RateLimitUpsertRequest{Name: "n", Resource: "function", Limit: 1, Window: 1, Action: "drop"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.Upsert(context.Background(), &tt.req)
			require.Error(t, err)
			assert.Nil(t, resp)
		})
	}
}

func TestService_Delete_Unknown_ReturnsNotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	err := svc.Delete(context.Background(), &RateLimitDeleteRequest{ID: "no-such-rule"})
	require.Error(t, err)
	var codeErr *errorx.CodeError
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, http.StatusNotFound, codeErr.Code)
}

func TestService_Delete_AfterUpsert(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	upsertResp, err := svc.Upsert(context.Background(), &RateLimitUpsertRequest{
		Name: "to-delete", Resource: "function", Limit: 1, Window: 1, Action: "reject",
	})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(context.Background(), &RateLimitDeleteRequest{ID: upsertResp.Id}))

	resp, err := svc.Get(context.Background(), &RateLimitGetRequest{ID: upsertResp.Id})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_Preview_OK(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.Preview(context.Background(), &RateLimitPreviewRequest{
		Rules: map[string]interface{}{"gameId": "demo", "env": "prod"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Matches)
	assert.NotNil(t, resp.Impact)
}

func TestService_Preview_EmptyRules(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	resp, err := svc.Preview(context.Background(), &RateLimitPreviewRequest{})
	require.Error(t, err)
	assert.Nil(t, resp)
}

func TestService_List_FilterByResource(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Upsert(context.Background(), &RateLimitUpsertRequest{
		Name: "a", Resource: "function", Limit: 1, Window: 1, Action: "reject",
	})
	require.NoError(t, err)
	_, err = svc.Upsert(context.Background(), &RateLimitUpsertRequest{
		Name: "b", Resource: "api", Limit: 1, Window: 1, Action: "throttle",
	})
	require.NoError(t, err)

	resp, err := svc.List(context.Background(), &RateLimitsListRequest{Resource: "function"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "function", resp.Items[0].Resource)
}

func TestNormalizeRules(t *testing.T) {
	tests := []struct {
		name    string
		payload interface{}
		want    map[string]interface{}
		wantErr bool
	}{
		{"nil payload", nil, nil, false},
		{"map payload", map[string]interface{}{"key": "value"}, map[string]interface{}{"key": "value"}, false},
		{"struct payload", struct{ Name string }{Name: "test"}, map[string]interface{}{"Name": "test"}, false},
		{"invalid payload", make(chan int), nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeRules(tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestGenerateRateLimitID(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		nameArg  string
		want     string
	}{
		{"simple", "function", "login", "function-login"},
		{"with spaces", " function ", " login ", "function-login"},
		{"with special chars", "function@name", "login@test", "function-name-login-test"},
		{"empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateRateLimitID(tt.resource, tt.nameArg)
			if tt.want == "" {
				// Empty input generates UUID
				assert.NotEmpty(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestEncodeRules(t *testing.T) {
	tests := []struct {
		name  string
		rules map[string]interface{}
		want  int
	}{
		{"nil rules", nil, 0},
		{"empty rules", map[string]interface{}{}, 0},
		{"with rules", map[string]interface{}{"key": "value"}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeRules(tt.rules)
			assert.Len(t, got, tt.want)
		})
	}
}

func TestSummarizeRuleKeys(t *testing.T) {
	tests := []struct {
		name  string
		rules map[string]interface{}
		want  []string
	}{
		{"nil rules", nil, nil},
		{"empty rules", map[string]interface{}{}, nil},
		{"with rules", map[string]interface{}{"b": 1, "a": 2}, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeRuleKeys(tt.rules)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClassifyComplexity(t *testing.T) {
	tests := []struct {
		name string
		keys int
		want string
	}{
		{"low complexity", 0, "low"},
		{"low complexity 1 key", 1, "low"},
		{"medium complexity 2 keys", 2, "medium"},
		{"medium complexity 3 keys", 3, "medium"},
		{"high complexity 4 keys", 4, "high"},
		{"high complexity 5 keys", 5, "high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyComplexity(tt.keys)
			assert.Equal(t, tt.want, got)
		})
	}
}
