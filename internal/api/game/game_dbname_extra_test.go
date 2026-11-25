// Coverage targets: the game handler's store error branches (table drop) and
// the default naming branch of deriveGameDBName.
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
	// No Router: default naming (the Router branch is covered by the router package itself)
	s := NewService(&svc.ServiceContext{})
	assert.Equal(t, router.DefaultGameDBName("demo", "prod"), s.deriveGameDBName("demo", "prod"))
}
