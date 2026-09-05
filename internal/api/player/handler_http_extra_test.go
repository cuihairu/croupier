package player

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newPlayerHTTPRouter 搭建带真实 Service 的 HTTP 路由，覆盖 handler 分支。
func newPlayerHTTPRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	h := NewHandler(NewService(svcCtx))
	r := gin.New()
	r.GET("/players", h.List)
	r.POST("/players", h.Create)
	r.GET("/players/:id", h.Detail)
	r.PUT("/players/:id", h.Update)
	r.DELETE("/players/:id", h.Delete)
	r.POST("/players/:id/balance", h.Balance)
	return r
}

func doJSON(r *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandlerHTTP_AllEndpoints(t *testing.T) {
	r := newPlayerHTTPRouter(t)

	// List：绑定失败
	assert.Equal(t, http.StatusBadRequest, doJSON(r, http.MethodGet, "/players?page=abc", "").Code)
	// List：成功
	assert.Equal(t, http.StatusOK, doJSON(r, http.MethodGet, "/players?page=1&pageSize=5", "").Code)

	// Create：缺用户名 → service 错误（普通 error → 500 兜底）
	assert.Equal(t, http.StatusInternalServerError, doJSON(r, http.MethodPost, "/players",
		`{"password":"pw","gameId":"g1"}`).Code)
	// Create：成功
	w := doJSON(r, http.MethodPost, "/players",
		`{"username":"http_user","password":"pw","gameId":"g1","nickname":"Nu"}`)
	require.Equal(t, http.StatusOK, w.Code)

	// Detail：不存在 → 错误分支
	assert.NotEqual(t, http.StatusOK, doJSON(r, http.MethodGet, "/players/999999", "").Code)
	// Update：目标不存在 → 错误（404/500 均为错误分支）
	assert.NotEqual(t, http.StatusOK, doJSON(r, http.MethodPut, "/players/999999",
		`{"nickname":"x"}`).Code)

	// Delete：不存在 → 错误分支
	assert.NotEqual(t, http.StatusOK, doJSON(r, http.MethodDelete, "/players/999999", "").Code)

	// Balance：缺少原因（普通 error → 500 兜底）
	assert.Equal(t, http.StatusInternalServerError, doJSON(r, http.MethodPost, "/players/999999/balance",
		`{"amount":10}`).Code)
	// Balance：目标不存在 → 错误分支
	assert.NotEqual(t, http.StatusOK, doJSON(r, http.MethodPost, "/players/999999/balance",
		`{"amount":10,"reason":"adjust"}`).Code)
}

func TestHandlerHTTP_UpdateAndBalanceSuccess(t *testing.T) {
	r := newPlayerHTTPRouter(t)

	w := doJSON(r, http.MethodPost, "/players",
		`{"username":"http_upd2","password":"pw","gameId":"g1"}`)
	require.Equal(t, http.StatusOK, w.Code)

	// 从创建响应提取玩家 ID（响应体顶层即 player 对象）
	var created struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotZero(t, created.ID)
	target := "/players/" + strconv.FormatInt(created.ID, 10)

	// Update：无有效更新字段（vip=-1 不入 updates）→ 错误分支
	assert.Equal(t, http.StatusInternalServerError, doJSON(r, http.MethodPut, target, `{"vip":-1}`).Code)

	// Update 成功
	w = doJSON(r, http.MethodPut, target, `{"nickname":"Renamed"}`)
	assert.Equal(t, http.StatusOK, w.Code)

	// Balance 成功
	w = doJSON(r, http.MethodPost, target+"/balance", `{"amount":50,"reason":"bonus"}`)
	assert.Equal(t, http.StatusOK, w.Code)
}

// scope 不匹配 → requirePlayerScope 禁止分支（Detail/Update/Delete/Balance）。
func TestServiceScopeMismatchForbidden(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	s := NewService(svcCtx)

	created, err := s.Create(context.Background(), &PlayerCreateRequest{
		Username: "scope_user_x", Password: "pw", GameId: "gameA",
	})
	require.NoError(t, err)

	other := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "gameB", Env: "prod"})
	id := strconv.FormatInt(created.Player.Id, 10)

	_, err = s.Detail(other, &PlayerDetailRequest{ID: id})
	assert.Error(t, err)

	_, err = s.Update(other, &PlayerUpdateRequest{ID: id, Nickname: "x"})
	assert.Error(t, err)

	assert.Error(t, s.Delete(other, &PlayerDeleteRequest{ID: id}))

	_, err = s.Balance(other, &PlayerBalanceRequest{ID: id, Amount: 1, Reason: "r"})
	assert.Error(t, err)

	// 同 scope → 放行
	ok := svc.WithGameScope(context.Background(), svc.GameScope{GameID: "gameA", Env: "prod"})
	_, err = s.Detail(ok, &PlayerDetailRequest{ID: id})
	assert.NoError(t, err)
}

// 底层模型不可用：List/Create/Update/Balance 错误透传 + handler 错误分支。
func TestHandlerModelFailureBranches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	bare, err := gorm.Open(gsqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	nullCache := cache.NewNullCache()
	svcCtx := &svc.ServiceContext{
		DB:          bare,
		PlayerModel: model.NewPlayerModel(bare), // 未迁移表 → 全部查询失败
		Cache:       nullCache,
		CacheHelper: cache.NewCacheHelper(nullCache),
	}
	h := NewHandler(NewService(svcCtx))
	r := gin.New()
	r.GET("/players", h.List)
	r.POST("/players", h.Create)
	r.GET("/players/:id", h.Detail)
	r.PUT("/players/:id", h.Update)
	r.DELETE("/players/:id", h.Delete)
	r.POST("/players/:id/balance", h.Balance)

	assert.NotEqual(t, http.StatusOK, doJSON(r, http.MethodGet, "/players", "").Code)
	assert.NotEqual(t, http.StatusOK, doJSON(r, http.MethodPost, "/players",
		`{"username":"u","password":"p","gameId":"g"}`).Code)
	assert.NotEqual(t, http.StatusOK, doJSON(r, http.MethodDelete, "/players/1", "").Code)
	assert.NotEqual(t, http.StatusOK, doJSON(r, http.MethodPost, "/players/1/balance",
		`{"amount":1,"reason":"r"}`).Code)
}
