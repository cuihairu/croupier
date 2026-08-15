package player

import (
	"context"
	"strconv"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- currentPlayerScope tests (covers 0% → 100%) ---

func TestCurrentPlayerScope_NoScopeInContext(t *testing.T) {
	t.Parallel()
	_, err := currentPlayerScope(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "scope 缺失")
}

func TestCurrentPlayerScope_WithScope(t *testing.T) {
	t.Parallel()
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{
		GameID: "game1",
		Env:    "prod",
	})
	scope, err := currentPlayerScope(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "game1", scope.GameID)
	assert.Equal(t, "prod", scope.Env)
}

// --- requirePlayerScope tests (covers 75% → 100%) ---

func TestRequirePlayerScope_NilPlayer(t *testing.T) {
	t.Parallel()
	err := requirePlayerScope(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该玩家")
}

func TestRequirePlayerScope_ScopeMismatch(t *testing.T) {
	t.Parallel()
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{
		GameID: "game1",
		Env:    "prod",
	})
	player := &model.Player{GameID: "game2"}
	err := requirePlayerScope(ctx, player)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该玩家")
}

func TestRequirePlayerScope_ScopeMatch(t *testing.T) {
	t.Parallel()
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{
		GameID: "game1",
		Env:    "prod",
	})
	player := &model.Player{GameID: "game1"}
	err := requirePlayerScope(ctx, player)
	assert.NoError(t, err)
}

func TestRequirePlayerScope_EmptyScope(t *testing.T) {
	t.Parallel()
	ctx := svc.WithGameScope(context.Background(), svc.GameScope{})
	player := &model.Player{GameID: "game1"}
	err := requirePlayerScope(ctx, player)
	assert.NoError(t, err)
}

// --- buildPlayer ---

func TestBuildPlayer_AllFields(t *testing.T) {
	t.Parallel()
	player := &model.Player{
		Username: "user1",
		Nickname: "User One",
		Email:    "user1@example.com",
		Phone:    "1234567890",
		GameID:   "game1",
		Status:   model.PlayerStatusActive,
		Balance:  1000,
		Level:    5,
		VIP:      3,
	}
	result := buildPlayer(player)
	assert.Equal(t, "user1", result.Username)
	assert.Equal(t, "User One", result.Nickname)
	assert.Equal(t, "user1@example.com", result.Email)
	assert.Equal(t, "1234567890", result.Phone)
	assert.Equal(t, "game1", result.GameId)
	assert.Equal(t, model.PlayerStatusActive, result.Status)
	assert.Equal(t, int64(1000), result.Balance)
	assert.Equal(t, 5, result.Level)
	assert.Equal(t, 3, result.Vip)
}

// --- Service.List with Level filter ---

func TestService_List_LevelFilterV4(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	pm := model.NewPlayerModel(db)

	require.NoError(t, pm.Create(context.Background(), &model.Player{
		Username: "lv_player_v4", Nickname: "Lv Player", GameID: "game_lv", Status: 1, Level: 10,
	}, "password"))

	s := NewService(svcCtx)
	resp, err := s.List(context.Background(), &PlayersListRequest{
		Page: 1, PageSize: 10, Level: 10,
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.GreaterOrEqual(t, resp.Total, int64(1))
}

// --- Service.Update with various branches ---

func TestService_Update_LevelV4(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	pm := model.NewPlayerModel(db)

	p := &model.Player{Username: "updlv", Nickname: "Upd Lv", GameID: "g1", Status: 1}
	require.NoError(t, pm.Create(context.Background(), p, "pw"))

	s := NewService(svcCtx)
	resp, err := s.Update(context.Background(), &PlayerUpdateRequest{
		ID:       strconv.FormatUint(uint64(p.ID), 10),
		Level:    99,
		Nickname: "Updated Lv",
	})
	assert.NoError(t, err)
	assert.Equal(t, 99, resp.Player.Level)
}

func TestService_Update_PhoneV4(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	pm := model.NewPlayerModel(db)

	p := &model.Player{Username: "updphone", Nickname: "Upd Phone", GameID: "g1", Status: 1}
	require.NoError(t, pm.Create(context.Background(), p, "pw"))

	s := NewService(svcCtx)
	resp, err := s.Update(context.Background(), &PlayerUpdateRequest{
		ID:    strconv.FormatUint(uint64(p.ID), 10),
		Phone: "9999999",
	})
	assert.NoError(t, err)
	assert.Equal(t, "9999999", resp.Player.Phone)
}

func TestService_Update_AllFieldsV4(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	pm := model.NewPlayerModel(db)

	p := &model.Player{Username: "updall", Nickname: "Old", GameID: "g1", Status: 1}
	require.NoError(t, pm.Create(context.Background(), p, "pw"))

	s := NewService(svcCtx)
	resp, err := s.Update(context.Background(), &PlayerUpdateRequest{
		ID:       strconv.FormatUint(uint64(p.ID), 10),
		Nickname: "New",
		Email:    "new@example.com",
		Phone:    "111",
		Status:   model.PlayerStatusSuspended,
		Level:    10,
		Vip:      5,
	})
	assert.NoError(t, err)
	assert.Equal(t, "New", resp.Player.Nickname)
	assert.Equal(t, "new@example.com", resp.Player.Email)
	assert.Equal(t, "111", resp.Player.Phone)
	assert.Equal(t, model.PlayerStatusSuspended, resp.Player.Status)
	assert.Equal(t, 10, resp.Player.Level)
	assert.Equal(t, 5, resp.Player.Vip)
}

func TestService_Update_NilRequestV4(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	s := NewService(svcCtx)

	resp, err := s.Balance(context.Background(), nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "请求体不能为空")
}

// --- Service.Detail with invalid ID ---

func TestService_Detail_InvalidIDV4(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	s := NewService(svcCtx)

	resp, err := s.Detail(context.Background(), &PlayerDetailRequest{ID: "notanumber"})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

// --- Service.Delete with invalid ID ---

func TestService_Delete_InvalidIDV4(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	s := NewService(svcCtx)

	err := s.Delete(context.Background(), &PlayerDeleteRequest{ID: "abc"})
	assert.Error(t, err)
}

// --- Service.Create with full data ---

func TestService_Create_FullDataV4(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	s := NewService(svcCtx)

	resp, err := s.Create(context.Background(), &PlayerCreateRequest{
		Username: "fulluser",
		Password: "pass123",
		Nickname: "Full User",
		Email:    "full@example.com",
		Phone:    "5555555",
		GameId:   "game_full",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "fulluser", resp.Player.Username)
	assert.Equal(t, "Full User", resp.Player.Nickname)
	assert.Equal(t, "full@example.com", resp.Player.Email)
	assert.Equal(t, "5555555", resp.Player.Phone)
}

// --- Service.List with whitespace search ---

func TestService_List_WithWhitespaceSearchV4(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	pm := model.NewPlayerModel(db)

	require.NoError(t, pm.Create(context.Background(), &model.Player{
		Username: "wsuser_v4", Nickname: "WS", GameID: "g1", Status: 1,
	}, "pw"))

	s := NewService(svcCtx)
	resp, err := s.List(context.Background(), &PlayersListRequest{
		Page: 1, PageSize: 10, Search: "  wsuser_v4  ",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// --- Service.Balance with negative amount ---

func TestService_Balance_NegativeAmountV4(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	pm := model.NewPlayerModel(db)

	p := &model.Player{Username: "balneg", Nickname: "Bal Neg", GameID: "g1", Status: 1, Balance: 100}
	require.NoError(t, pm.Create(context.Background(), p, "pw"))

	s := NewService(svcCtx)
	resp, err := s.Balance(context.Background(), &PlayerBalanceRequest{
		ID:     strconv.FormatUint(uint64(p.ID), 10),
		Amount: -50,
		Reason: "Purchase",
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(50), resp.Player.Balance)
}

// --- Service.Balance invalid ID ---

func TestService_Balance_InvalidIDV4(t *testing.T) {
	db := setupTestDB(t)
	svcCtx := setupTestServiceContext(t, db)
	s := NewService(svcCtx)

	resp, err := s.Balance(context.Background(), &PlayerBalanceRequest{
		ID:     "notanumber",
		Amount: 100,
		Reason: "test",
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
}
