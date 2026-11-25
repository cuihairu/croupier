package game

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When a game does not exist (including after deletion), the detail and env endpoints must
// return 404 not_found, and must not pass a plain model-layer error through as
// 500 internal_error.
// Regression from a production walkthrough: create -> delete -> GET detail once returned
// 500 "game not found".
func TestGameLookup_MissingGameReturns404(t *testing.T) {
	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	svc := NewService(svcCtx)

	ctx := createPermUser(t, db, "perm_missing_404", "games:read", "games:write", "games:manage")

	resp, err := svc.Create(ctx, &GameCreateRequest{Name: "missing_404_game"})
	require.NoError(t, err)
	gameID := strconv.FormatUint(uint64(resp.Game.ID), 10)
	require.NoError(t, svc.Delete(ctx, &GameDeleteRequest{ID: gameID}))

	assertNotFound := func(t *testing.T, err error) {
		t.Helper()
		require.Error(t, err)
		var codeErr *errorx.CodeError
		require.ErrorAs(t, err, &codeErr)
		assert.Equal(t, http.StatusNotFound, codeErr.Code)
		assert.Equal(t, "not_found", codeErr.ErrorCode())
	}

	// Model layer: a sentinel recognizable via errors.Is, with the message kept as
	// "game not found".
	_, err = svcCtx.GameModel.FindOne(context.Background(), 999999)
	require.ErrorIs(t, err, model.ErrGameNotFound)
	assert.Contains(t, err.Error(), "game not found")

	// Detail after deletion (the main walkthrough regression path: previously 500).
	_, err = svc.Detail(ctx, &GameDetailRequest{ID: gameID})
	assertNotFound(t, err)

	// Numeric key addressing does not pre-check existence; a failed load is likewise 404.
	_, err = svc.Detail(ctx, &GameDetailRequest{ID: "999999"})
	assertNotFound(t, err)

	// Business string addressing with no match -> 404 (the resolveGameID path).
	_, err = svc.Detail(ctx, &GameDetailRequest{ID: "ghost_game"})
	assertNotFound(t, err)

	// Updating a missing game: the read-back after writing 0 rows is likewise 404.
	_, err = svc.Update(ctx, &GameUpdateRequest{ID: gameID, AliasName: "不该成功"})
	assertNotFound(t, err)

	// The four env endpoints.
	_, err = svc.EnvsList(ctx, &GameEnvsListRequest{ID: gameID})
	assertNotFound(t, err)

	_, err = svc.EnvAdd(ctx, &GameEnvAddRequest{ID: gameID, Name: "ghostenv", Type: "dev"})
	assertNotFound(t, err)

	_, err = svc.EnvUpdate(ctx, &GameEnvUpdateRequest{ID: gameID, EnvID: "prod", Name: "prod2"})
	assertNotFound(t, err)

	_, err = svc.EnvDelete(ctx, &GameEnvDeleteRequest{ID: gameID, EnvID: "prod"})
	assertNotFound(t, err)
}

// HTTP contract layer: handler + response.Error must produce 404 {"error":"not_found"},
// not 500 internal_error.
func TestGameDetail_HTTP_MissingGameContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupTestDB(t)
	seedTestPermissions(t, db)
	svcCtx := setupTestServiceContext(t, db)
	svc := NewService(svcCtx)
	handler := NewHandler(svc)

	permCtx := createPermUser(t, db, "perm_http_404", "games:read", "games:write", "games:manage")

	resp, err := svc.Create(permCtx, &GameCreateRequest{Name: "http_404_game"})
	require.NoError(t, err)
	idStr := strconv.FormatUint(uint64(resp.Game.ID), 10)
	require.NoError(t, svc.Delete(permCtx, &GameDeleteRequest{ID: idStr}))

	c, rec := newGameTestContext("GET", "/games/"+idStr, "")
	c.Params = gin.Params{{Key: "id", Value: idStr}}
	c.Request = c.Request.WithContext(permCtx)
	handler.Detail(c)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error":"not_found"`)
}
