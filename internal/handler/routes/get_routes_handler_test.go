package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupHandlerSvc(t *testing.T) *svc.ServiceContext {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Function{}))
	return &svc.ServiceContext{FunctionModel: model.NewFunctionModel(db)}
}

// GetRoutesHandler 成功分支：HTTP 壳返回分组路由 JSON。
func TestGetRoutesHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svcCtx := setupHandlerSvc(t)
	require.NoError(t, svcCtx.FunctionModel.Create(nil, &model.Function{
		FunctionID: "player.getList", Resource: "player", Status: 1,
	}))

	r := gin.New()
	r.GET("/routes", GetRoutesHandler(svcCtx))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/routes", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "player")
}

// GetRoutesHandler 失败分支：底层查询失败时经统一错误响应返回 500。
func TestGetRoutesHandler_QueryError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	// 不迁移 function 表 → List 查询失败
	svcCtx := &svc.ServiceContext{FunctionModel: model.NewFunctionModel(db)}

	r := gin.New()
	r.GET("/routes", GetRoutesHandler(svcCtx))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/routes", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
