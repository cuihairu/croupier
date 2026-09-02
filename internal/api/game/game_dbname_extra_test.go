// 覆盖目标：game handler 的存储错误分支（表删除）与
// deriveGameDBName 的默认命名分支。
package game

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cuihairu/croupier/internal/db/router"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGameHandler_List_StoreError(t *testing.T) {
	handler, db := setupGameHandlerTest(t)
	require.NoError(t, db.Migrator().DropTable("games"))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/games", nil)
	addGameAuthMiddleware(db)(ctx)
	if !ctx.IsAborted() {
		handler.List(ctx)
	}
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestGameHandler_EnvsList_StoreError(t *testing.T) {
	handler, db := setupGameHandlerTest(t)
	require.NoError(t, db.Migrator().DropTable("game_envs"))

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/games/demo/envs", nil)
	addGameAuthMiddleware(db)(ctx)
	if !ctx.IsAborted() {
		handler.EnvsList(ctx)
	}
	assert.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestDeriveGameDBName_Default(t *testing.T) {
	// 无 Router：默认命名（Router 分支由 router 包自身覆盖）
	s := NewService(&svc.ServiceContext{})
	assert.Equal(t, router.DefaultGameDBName("demo", "prod"), s.deriveGameDBName("demo", "prod"))
}
