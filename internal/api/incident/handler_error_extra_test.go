package incident

// handler.go / external_handler.go 补测（错误臂口径）：各 HTTP 入口的
// service 失败透传臂（经 failpoint 注入或不存在 ID 触发）、DeleteCategory
// 的成功/内建拒绝两臂、ListIncidents 的 to 时间解析成功臂、
// TransitionIncident 的参数/绑定/成功三臂；外部面 ListIncidents 的
// from/to 解析臂与 service 失败臂、CreateBug 的未鉴权返回与 service 失败
// 臂、令牌管理四入口的失败/分支臂。
//
// 诚实跳过：无。

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// setupFullRouter 注册全部 incident 面板路由（setupRouter 只挂了子集）。
func setupFullRouter(t *testing.T) (*gin.Engine, *Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := setupService(t)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("username", "tester")
		c.Next()
	})
	h := NewHandler(s)
	cg := router.Group("/incident-categories")
	cg.GET("", h.ListCategories)
	cg.POST("", h.CreateCategory)
	cg.PUT("/:id", h.UpdateCategory)
	cg.GET("/:id/usage", h.CategoryUsage)
	cg.DELETE("/:id", h.DeleteCategory)
	ig := router.Group("/incidents")
	ig.GET("", h.ListIncidents)
	ig.POST("", h.CreateIncident)
	ig.GET("/:id", h.GetIncident)
	ig.PUT("/:id", h.UpdateIncident)
	ig.POST("/:id/status", h.TransitionIncident)
	return router, s
}

func TestHandler_ListCategories_ServiceError(t *testing.T) {
	router, s := setupFullRouter(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	w := doJSON(t, router, http.MethodGet, "/incident-categories", "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestHandler_UpdateCategory_ServiceError(t *testing.T) {
	router, _ := setupFullRouter(t)
	w := doJSON(t, router, http.MethodPut, "/incident-categories/999999", `{"name":"x"}`)
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestHandler_CategoryUsage_ServiceError(t *testing.T) {
	router, _ := setupFullRouter(t)
	w := doJSON(t, router, http.MethodGet, "/incident-categories/999999/usage", "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestHandler_DeleteCategory_BuiltinRefusedAndSuccess(t *testing.T) {
	router, s := setupFullRouter(t)

	// 内建行拒绝删除 → 错误臂
	w := doJSON(t, router, http.MethodDelete, "/incident-categories/"+itoa(t, mustCategory(t, s, model.IncidentCatQA).ID), "")
	require.GreaterOrEqual(t, w.Code, 400)

	// 非内建行删除成功 → response.Success(c, nil)
	tmp, err := s.CreateCategory(context.Background(), &CategoryUpsertRequest{Name: strPtr("可删域"), Slug: "dele"})
	require.NoError(t, err)
	w = doJSON(t, router, http.MethodDelete, "/incident-categories/"+itoa(t, tmp.ID), "")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_ListIncidents_ToTimeAndServiceError(t *testing.T) {
	router, s := setupFullRouter(t)

	// to 时间解析成功臂（RFC3339）
	w := doJSON(t, router, http.MethodGet, "/incidents?to=2026-11-09T00:00:00Z", "")
	require.Equal(t, http.StatusOK, w.Code)

	// service 失败臂
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: 1, err: errFailpoint})
	w = doJSON(t, router, http.MethodGet, "/incidents", "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestHandler_CreateIncident_ServiceError(t *testing.T) {
	router, _ := setupFullRouter(t)
	w := doJSON(t, router, http.MethodPost, "/incidents", `{"title":"x","categoryId":999999}`)
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestHandler_UpdateIncident_ServiceError(t *testing.T) {
	router, _ := setupFullRouter(t)
	w := doJSON(t, router, http.MethodPut, "/incidents/999999", `{"title":"x"}`)
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestHandler_TransitionIncident_ParamBindAndSuccess(t *testing.T) {
	router, s := setupFullRouter(t)
	qa := mustCategory(t, s, model.IncidentCatQA)
	body := `{"title":"流转","categoryId":` + itoa(t, qa.ID) + `}`
	w := doJSON(t, router, http.MethodPost, "/incidents", body)
	require.Equal(t, http.StatusCreated, w.Code)

	// 参数非法臂
	w = doJSON(t, router, http.MethodPost, "/incidents/abc/status", `{"status":"acknowledged"}`)
	require.GreaterOrEqual(t, w.Code, 400)

	// 绑定失败臂（空 body）
	w = doJSON(t, router, http.MethodPost, "/incidents/1/status", "")
	require.GreaterOrEqual(t, w.Code, 400)

	// 成功臂
	w = doJSON(t, router, http.MethodPost, "/incidents/1/status", `{"status":"acknowledged"}`)
	require.Equal(t, http.StatusOK, w.Code)
}

// ---- external_handler.go ----

func TestExternalHandler_ListIncidents_FromToAndError(t *testing.T) {
	router, _, db, plain := setupExternalRouter(t, []string{"admin"})

	// from/to 解析成功臂
	w := extReq(t, router, http.MethodGet,
		"/ext/incidents?from=2026-01-01T00:00:00Z&to=2026-12-31T00:00:00Z", "", plain, "")
	require.Equal(t, http.StatusOK, w.Code)

	// service 失败臂 + audit fail
	applyDBFailpoint(t, db, dbFailpoint{table: "incidents", op: "query", n: 1, err: errFailpoint})
	w = extReq(t, router, http.MethodGet, "/ext/incidents", "", plain, "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestExternalHandler_CreateBug_AuthFailAndServiceError(t *testing.T) {
	router, _, db, plain := setupExternalRouter(t, []string{"admin"})

	// 未鉴权臂（tok==nil 直接返回）
	w := extReq(t, router, http.MethodPost, "/ext/bugs", `{"title":"b"}`, "", "")
	require.GreaterOrEqual(t, w.Code, 400)

	// service 失败臂 + audit fail
	applyDBFailpoint(t, db, dbFailpoint{table: "bugs", op: "create", n: 1, err: errFailpoint})
	w = extReq(t, router, http.MethodPost, "/ext/bugs", `{"title":"b"}`, plain, "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestExternalHandler_TokenCreate_ListErrors(t *testing.T) {
	router, _, db, _ := setupExternalRouter(t, []string{"admin"})

	// CreateToken 落库失败
	applyDBFailpoint(t, db, dbFailpoint{table: "external_tokens", op: "create", n: 1, err: errFailpoint})
	w := extReq(t, router, http.MethodPost, "/ext/tokens", `{"name":"t1"}`, "", "")
	require.GreaterOrEqual(t, w.Code, 400)

	// ListTokens 失败
	applyDBFailpoint(t, db, dbFailpoint{table: "external_tokens", op: "query", n: 1, err: errFailpoint})
	w = extReq(t, router, http.MethodGet, "/ext/tokens", "", "", "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestExternalHandler_TokenUpdate_Branches(t *testing.T) {
	router, _, db, _ := setupExternalRouter(t, []string{"admin"})

	// name 分支
	var tok model.ExternalToken
	require.NoError(t, db.First(&tok).Error)
	w := extReq(t, router, http.MethodPut, "/ext/tokens/"+itoa(t, tok.ID), `{"name":"renamed"}`, "", "")
	require.Equal(t, http.StatusNoContent, w.Code)

	// scope 分支（json.Marshal 成功落库）
	w = extReq(t, router, http.MethodPut, "/ext/tokens/"+itoa(t, tok.ID), `{"scope":{"categories":["qa"]}}`, "", "")
	require.Equal(t, http.StatusNoContent, w.Code)

	// Update 失败臂
	applyDBFailpoint(t, db, dbFailpoint{table: "external_tokens", op: "update", n: 1, err: errFailpoint})
	w = extReq(t, router, http.MethodPut, "/ext/tokens/"+itoa(t, tok.ID), `{"name":"again"}`, "", "")
	require.GreaterOrEqual(t, w.Code, 400)
}

func TestExternalHandler_TokenDelete_ParamErrorAndDeleteError(t *testing.T) {
	router, _, db, _ := setupExternalRouter(t, []string{"admin"})

	// 参数非法臂
	w := extReq(t, router, http.MethodDelete, "/ext/tokens/abc", "", "", "")
	require.GreaterOrEqual(t, w.Code, 400)

	// Delete 失败臂
	applyDBFailpoint(t, db, dbFailpoint{table: "external_tokens", op: "delete", n: 1, err: errFailpoint})
	w = extReq(t, router, http.MethodDelete, "/ext/tokens/1", "", "", "")
	require.GreaterOrEqual(t, w.Code, 400)
}

// ---- helpers ----

func itoa(t *testing.T, v uint) string {
	t.Helper()
	return strconv.FormatUint(uint64(v), 10)
}
