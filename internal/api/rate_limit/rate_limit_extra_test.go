// Coverage target: the handler's service-error / JSON-binding error branches,
// the database error branches of the service-layer List/Get/Upsert/Delete, the
// normalizeRules non-object rule branch, and the parseRateLimitID blank ID
// branch.
package rate_limit

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// brokenSvcCtx returns a service context whose underlying rate_limits table
// has been dropped, used to trigger model-layer database errors (not
// NotFound).
func brokenSvcCtx(t *testing.T) *svc.ServiceContext {
	t.Helper()
	svcCtx := setupSvcCtx(t)
	require.NoError(t, svcCtx.DB.Migrator().DropTable("rate_limits"))
	return svcCtx
}

func TestService_List_DBError(t *testing.T) {
	svcCtx := brokenSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.List(context.Background(), &RateLimitsListRequest{})
	require.Error(t, err)
}

func TestService_Get_DBError(t *testing.T) {
	svcCtx := brokenSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Get(context.Background(), &RateLimitGetRequest{ID: "rl-1"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "不存在", "should be a raw db error, not NotFound mapping")
}

func TestService_Delete_DBError(t *testing.T) {
	svcCtx := brokenSvcCtx(t)
	svc := NewService(svcCtx)

	err := svc.Delete(context.Background(), &RateLimitDeleteRequest{ID: "rl-1"})
	require.Error(t, err)
}

func TestService_Delete_BlankID(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	err := svc.Delete(context.Background(), &RateLimitDeleteRequest{ID: "   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不能为空")
}

func TestService_Get_BlankID(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Get(context.Background(), &RateLimitGetRequest{ID: " "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不能为空")
}

func TestService_Upsert_RulesNotObject(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Upsert(context.Background(), &RateLimitUpsertRequest{
		Name:     "cap",
		Resource: "function",
		Limit:    5,
		Window:   30,
		Action:   "reject",
		Rules:    []interface{}{"not-a-map"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "规则必须为对象")
}

func TestService_Upsert_ModelError(t *testing.T) {
	svcCtx := brokenSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Upsert(context.Background(), &RateLimitUpsertRequest{
		Name:     "cap",
		Resource: "function",
		Limit:    5,
		Window:   30,
		Action:   "reject",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "保存限流规则失败")
}

func TestService_Upsert_FindByKeyAfterWriteFails(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	// Drop the table right after a successful write to trigger the database
	// error branch of FindByKey inside Upsert
	require.NoError(t, svcCtx.DB.Callback().Create().After("gorm:create").Register("test_drop_table", func(tx *gorm.DB) {
		_ = tx.Migrator().DropTable("rate_limits")
	}))
	t.Cleanup(func() {
		_ = svcCtx.DB.Callback().Create().Remove("test_drop_table")
	})

	_, err := svc.Upsert(context.Background(), &RateLimitUpsertRequest{
		Name:     "cap",
		Resource: "function",
		Limit:    5,
		Window:   30,
		Action:   "reject",
	})
	require.Error(t, err)
}

func TestService_Preview_RulesNotObject(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	svc := NewService(svcCtx)

	_, err := svc.Preview(context.Background(), &RateLimitPreviewRequest{
		Rules: "plain-string-rules",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "规则必须为对象")
}

func TestHandler_List_ServiceError(t *testing.T) {
	handler := NewHandler(NewService(brokenSvcCtx(t)))
	router := newRouter(handler)

	rec := doJSON(t, router, http.MethodGet, "/rate-limits", "")
	assertStatus(t, rec, http.StatusInternalServerError)
	assertErrorShape(t, rec)
}

func TestHandler_Preview_InvalidJSON(t *testing.T) {
	handler := setupHandler(t)
	router := newRouter(handler)

	rec := doJSON(t, router, http.MethodPost, "/rate-limits/preview", `{invalid`)
	assert.NotEqual(t, http.StatusOK, rec.Code)
	assertErrorShape(t, rec)
}

// TestService_Upsert_ValdationBranches covers each parameter validation error
// branch.
func TestService_Upsert_ValdationBranches(t *testing.T) {
	svc := NewService(setupSvcCtx(t))
	ctx := context.Background()

	cases := []struct {
		name string
		req  *RateLimitUpsertRequest
		want string
	}{
		{"empty name", &RateLimitUpsertRequest{Name: " ", Resource: "function", Limit: 1, Window: 1, Action: "reject"}, "名称不能为空"},
		{"empty resource", &RateLimitUpsertRequest{Name: "n", Resource: " ", Limit: 1, Window: 1, Action: "reject"}, "资源类型不能为空"},
		{"zero limit", &RateLimitUpsertRequest{Name: "n", Resource: "function", Limit: 0, Window: 1, Action: "reject"}, "limit 必须大于0"},
		{"zero window", &RateLimitUpsertRequest{Name: "n", Resource: "function", Limit: 1, Window: 0, Action: "reject"}, "window 必须大于0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Upsert(ctx, tc.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestModelNotFound directly verifies that the ErrNotFound semantics the model
// error mapping depends on still hold.
func TestModelNotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	_, err := svcCtx.RateLimitModel.FindByKey(context.Background(), "missing")
	assert.ErrorIs(t, err, model.ErrNotFound)
}
