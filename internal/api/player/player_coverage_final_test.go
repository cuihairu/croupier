// 覆盖目标（coverage final）：
//  1. handler.Detail / handler.Delete 的成功分支（HTTP 层）。
//  2. service.Update 的 PlayerModel.Update 失败（update callback 注错）。
//  3. service.Update 的回读 FindOne 失败（query callback 按次注错）。
//  4. service.Balance 的 UpdateBalance 失败（负余额余额不足）。
package player

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/cache"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPlayerHandler_DetailAndDelete_Success(t *testing.T) {
	r := newPlayerHTTPRouter(t)

	w := doJSON(r, http.MethodPost, "/players", `{"username":"final_user","password":"pw","gameId":"g1"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var created struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	target := "/players/" + strconv.FormatInt(created.ID, 10)

	// Detail 成功 → 200
	assert.Equal(t, http.StatusOK, doJSON(r, http.MethodGet, target, "").Code)
	// Delete 成功 → 200
	assert.Equal(t, http.StatusOK, doJSON(r, http.MethodDelete, target, "").Code)
}

func newPlayerFailDB(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file::memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	nullCache := cache.NewNullCache()
	svcCtx := &svc.ServiceContext{
		DB:          db,
		PlayerModel: model.NewPlayerModel(db),
		Cache:       nullCache,
		CacheHelper: cache.NewCacheHelper(nullCache),
	}
	return NewService(svcCtx), db
}

func TestPlayerService_Update_ModelFailure(t *testing.T) {
	ctx := context.Background()
	s, db := newPlayerFailDB(t)

	created, err := s.Create(ctx, &PlayerCreateRequest{Username: "u1", Password: "pw", GameId: "g"})
	require.NoError(t, err)
	id := strconv.FormatInt(created.Player.Id, 10)

	require.NoError(t, db.Callback().Update().Before("gorm:update").
		Register("test:player_fail_update", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("update boom"))
		}))

	_, err = s.Update(ctx, &PlayerUpdateRequest{ID: id, Nickname: "n"})
	require.ErrorContains(t, err, "update boom")
}

func TestPlayerService_Update_ReloadFailure(t *testing.T) {
	ctx := context.Background()
	s, db := newPlayerFailDB(t)

	created, err := s.Create(ctx, &PlayerCreateRequest{Username: "u2", Password: "pw", GameId: "g"})
	require.NoError(t, err)
	id := strconv.FormatInt(created.Player.Id, 10)

	// Update 流程 SELECT：FindOne → (UPDATE) → 回读 FindOne；令第 2 次 SELECT 失败。
	var selects int32
	require.NoError(t, db.Callback().Query().Before("gorm.query").
		Register("test:player_fail_reload", func(tx *gorm.DB) {
			if atomic.AddInt32(&selects, 1) == 2 {
				_ = tx.AddError(errors.New("reload boom"))
			}
		}))

	_, err = s.Update(ctx, &PlayerUpdateRequest{ID: id, Nickname: "n"})
	require.ErrorContains(t, err, "reload boom")
}

func TestPlayerService_Balance_InsufficientBalance(t *testing.T) {
	ctx := context.Background()
	s, _ := newPlayerFailDB(t)

	created, err := s.Create(ctx, &PlayerCreateRequest{Username: "u3", Password: "pw", GameId: "g"})
	require.NoError(t, err)

	_, err = s.Balance(ctx, &PlayerBalanceRequest{
		ID: strconv.FormatInt(created.Player.Id, 10), Amount: -100, Reason: "deduct",
	})
	require.ErrorContains(t, err, "insufficient balance")
}

func TestPlayerService_Update_StatusInvalid(t *testing.T) {
	ctx := context.Background()
	s, _ := newPlayerFailDB(t)

	created, err := s.Create(ctx, &PlayerCreateRequest{Username: "u4", Password: "pw", GameId: "g"})
	require.NoError(t, err)

	_, err = s.Update(ctx, &PlayerUpdateRequest{
		ID: strconv.FormatInt(created.Player.Id, 10), Status: 99,
	})
	require.ErrorContains(t, err, "状态值无效")
}
