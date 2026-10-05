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

// 游戏不存在（含删除后）时，详情与环境端点必须回 404 not_found，
// 不得把模型层普通 error 透传成 500 internal_error。
// 线上走查回归：创建 → 删除 → GET 详情曾返回 500 "game not found"。
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

	// 模型层：可 errors.Is 识别的哨兵，文案保留 "game not found"。
	_, err = svcCtx.GameModel.FindOne(context.Background(), 999999)
	require.ErrorIs(t, err, model.ErrGameNotFound)
	assert.Contains(t, err.Error(), "game not found")

	// 删除后详情（走查回归主路径：此前 500）。
	_, err = svc.Detail(ctx, &GameDetailRequest{ID: gameID})
	assertNotFound(t, err)

	// 数字主键寻址不预检存在性，载入失败同样 404。
	_, err = svc.Detail(ctx, &GameDetailRequest{ID: "999999"})
	assertNotFound(t, err)

	// 业务串寻址不存在 → 404（resolveGameID 路径）。
	_, err = svc.Detail(ctx, &GameDetailRequest{ID: "ghost_game"})
	assertNotFound(t, err)

	// 更新不存在的游戏：写 0 行后回读同样 404。
	_, err = svc.Update(ctx, &GameUpdateRequest{ID: gameID, AliasName: "不该成功"})
	assertNotFound(t, err)

	// 环境四端点。
	_, err = svc.EnvsList(ctx, &GameEnvsListRequest{ID: gameID})
	assertNotFound(t, err)

	_, err = svc.EnvAdd(ctx, &GameEnvAddRequest{ID: gameID, Name: "ghostenv", Type: "dev"})
	assertNotFound(t, err)

	_, err = svc.EnvUpdate(ctx, &GameEnvUpdateRequest{ID: gameID, EnvID: "prod", Name: "prod2"})
	assertNotFound(t, err)

	_, err = svc.EnvDelete(ctx, &GameEnvDeleteRequest{ID: gameID, EnvID: "prod"})
	assertNotFound(t, err)
}

// HTTP 契约层：handler + response.Error 必须产出 404 {"error":"not_found"}，
// 而不是 500 internal_error。
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
