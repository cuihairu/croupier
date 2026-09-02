// 覆盖目标：handler 的 service 错误/JSON 绑定错误分支，
// service 层 List/Get/Upsert/Delete 的数据库错误分支、
// normalizeRules 非对象规则分支、parseRateLimitID 空白 ID 分支。
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

// brokenSvcCtx 返回底层 rate_limits 表已被删除的服务上下文，
// 用于触发 model 层数据库错误（非 NotFound）。
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

	// 在写入成功后立即删除表，触发 Upsert 内部 FindByKey 的数据库错误分支
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

// TestService_Upsert_ValdationBranches 覆盖参数校验各错误分支。
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
		{"zero limit", &RateLimitUpsertRequest{Name: "n", Resource: "function", Limit: 0, Window: 1, Action: "reject"}, "Limit"},
		{"zero window", &RateLimitUpsertRequest{Name: "n", Resource: "function", Limit: 1, Window: 0, Action: "reject"}, "Window"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Upsert(ctx, tc.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestModelNotFound 直接验证 model 错误映射依赖的 ErrNotFound 语义仍然成立。
func TestModelNotFound(t *testing.T) {
	svcCtx := setupSvcCtx(t)
	_, err := svcCtx.RateLimitModel.FindByKey(context.Background(), "missing")
	assert.ErrorIs(t, err, model.ErrNotFound)
}
