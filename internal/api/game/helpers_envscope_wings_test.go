package game

// Coverage sweep round 15 (wt-api): closing out the 3 remaining blocks in helpers.go —
// the LoadCurrentAdmin failure wing of gameEnvScopes (no identity in the context), the
// GetAdminEnvScopes storage error wing (grant table missing -> errorx internal error, never
// a raw SQL error passed through), and the authorizeGameEnv wing that passes gameEnvScopes
// errors through.
// Reuses the service_envscope_test.go fixtures (the #47 authorization guard domain, an older
// domain from this session).

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var gameWingsDBSeq atomic.Int64

func TestGameEnvScopes_ErrorWings(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	service := NewService(svcCtx)

	// LoadCurrentAdmin failure wing: no identity in the context
	_, _, err := service.gameEnvScopes(context.Background(), 1)
	require.Error(t, err)

	// GetAdminEnvScopes storage error wing: grant table missing -> wrapped as an internal
	// error.
	// setupTestDB is a cache=shared process-wide shared database (DropTable would poison
	// other cases), so the missing-table injection needs an isolated database.
	iso, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:game_envscope_wings_%d?mode=memory&cache=shared", gameWingsDBSeq.Add(1))), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(iso))
	seedTestPermissions(t, iso)
	svcCtx2 := setupTestServiceContext(t, iso)
	ctx, _ := createEnvScopedManager(t, iso, "scope_store_broken")
	require.NoError(t, iso.Migrator().DropTable("admin_game_env_scopes"))
	service2 := NewService(svcCtx2)

	_, _, err = service2.gameEnvScopes(ctx, 1)
	require.Error(t, err)
	_, ok := err.(*errorx.CodeError)
	assert.True(t, ok, "存储故障应包装为 CodeError，got %T: %v", err, err)

	// authorizeGameEnv passthrough wing: the underlying gameEnvScopes error is raised as-is
	err = service2.authorizeGameEnv(ctx, 1, "prod")
	require.Error(t, err)

	// EnvsList passthrough wing: after both permission and game addressing pass, it hits the
	// gameEnvScopes storage failure
	game := createScopeTestGame(t, iso, "envlist_store_broken", model.GameEnv{Env: "prod"})
	_, err = service2.EnvsList(ctx, &GameEnvsListRequest{ID: strconv.FormatUint(uint64(game.ID), 10)})
	require.Error(t, err)
}
