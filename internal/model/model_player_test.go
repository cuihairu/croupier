package model

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// Split from model_test.go; test bodies moved verbatim.

func TestNewPlayerModel(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	assert.NotNil(t, model)
}

func TestPlayerModel_Create(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "testplayer",
		Nickname: "Test Player",
		GameID:   "game1",
		Status:   1,
		Balance:  1000,
	}

	err := model.Create(ctx, player, "password123")
	require.NoError(t, err)
	assert.NotZero(t, player.ID)
	assert.NotEmpty(t, player.Password)
}

func TestPlayerModel_Create_NoPassword(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "nopwdplayer",
		GameID:   "game1",
		Status:   1,
	}

	err := model.Create(ctx, player, "")
	require.NoError(t, err)
	assert.Empty(t, player.Password)
}

func TestPlayerModel_FindOne(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "findoneplayer",
		GameID:   "game1",
		Status:   1,
	}
	err := model.Create(ctx, player, "password")
	require.NoError(t, err)

	found, err := model.FindOne(ctx, player.ID)
	require.NoError(t, err)
	assert.Equal(t, player.Username, found.Username)

	_, err = model.FindOne(ctx, 99999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "player not found")
}

func TestPlayerModel_FindByUsername(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "usernameplayer",
		GameID:   "game1",
		Status:   1,
	}
	err := model.Create(ctx, player, "password")
	require.NoError(t, err)

	found, err := model.FindByUsername(ctx, "usernameplayer", "game1")
	require.NoError(t, err)
	assert.Equal(t, player.ID, found.ID)

	// Test with no game ID
	found2, err := model.FindByUsername(ctx, "usernameplayer", "")
	require.NoError(t, err)
	assert.Equal(t, player.ID, found2.ID)

	_, err = model.FindByUsername(ctx, "nonexistent", "game1")
	assert.Error(t, err)
}

func TestPlayerModel_Update(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "updateplayer",
		GameID:   "game1",
		Status:   1,
	}
	err := model.Create(ctx, player, "password")
	require.NoError(t, err)

	updates := map[string]interface{}{
		"nickname": "Updated Nickname",
		"balance":  int64(2000),
	}
	err = model.Update(ctx, player.ID, updates)
	require.NoError(t, err)

	found, err := model.FindOne(ctx, player.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Nickname", found.Nickname)
	assert.Equal(t, int64(2000), found.Balance)
}

func TestPlayerModel_Delete(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "deleteplayer",
		GameID:   "game1",
		Status:   1,
	}
	err := model.Create(ctx, player, "password")
	require.NoError(t, err)

	err = model.Delete(ctx, player.ID)
	require.NoError(t, err)

	_, err = model.FindOne(ctx, player.ID)
	assert.Error(t, err)
}

func TestPlayerModel_List(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		player := &Player{
			Username: "listplayer" + string(rune('a'+i)),
			GameID:   "game1",
			Status:   1,
		}
		err := model.Create(ctx, player, "password")
		require.NoError(t, err)
	}

	players, total, err := model.List(ctx, ListPlayersOptions{
		Page:     1,
		PageSize: 10,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(5))
	assert.GreaterOrEqual(t, len(players), 5)
}

func TestPlayerModel_List_GameIDFilter(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player1 := &Player{Username: "game1player", GameID: "game1", Status: 1}
	player2 := &Player{Username: "game2player", GameID: "game2", Status: 1}
	err := model.Create(ctx, player1, "password")
	require.NoError(t, err)
	err = model.Create(ctx, player2, "password")
	require.NoError(t, err)

	players, total, err := model.List(ctx, ListPlayersOptions{
		Page:     1,
		PageSize: 10,
		GameID:   "game1",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	for _, p := range players {
		assert.Equal(t, "game1", p.GameID)
	}
}

func TestPlayerModel_List_Search(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player1 := &Player{Username: "searchplayer", Nickname: "Searchable Player", GameID: "game1", Status: 1}
	player2 := &Player{Username: "otherplayer", Nickname: "Other Player", GameID: "game1", Status: 1}
	err := model.Create(ctx, player1, "password")
	require.NoError(t, err)
	err = model.Create(ctx, player2, "password")
	require.NoError(t, err)

	players, total, err := model.List(ctx, ListPlayersOptions{
		Page:     1,
		PageSize: 10,
		Search:   "searchable",
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	found := false
	for _, p := range players {
		if p.Username == "searchplayer" {
			found = true
			break
		}
	}
	assert.True(t, found)
}

func TestPlayerModel_List_StatusFilter(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	// Use unique usernames to avoid conflicts
	timestamp := time.Now().Format("20060102150405.999")
	active := 1
	suspended := 2
	player1 := &Player{Username: "activeplayer" + timestamp, GameID: "game1", Status: 1}
	player2 := &Player{Username: "suspendedplayer" + timestamp, GameID: "game1", Status: 2}
	err := model.Create(ctx, player1, "password")
	require.NoError(t, err)
	err = model.Create(ctx, player2, "password")
	require.NoError(t, err)

	// First verify players were created with correct statuses
	found1, err := model.FindByUsername(ctx, player1.Username, "")
	require.NoError(t, err)
	assert.Equal(t, 1, found1.Status, "Active player should have status 1")

	found2, err := model.FindByUsername(ctx, player2.Username, "")
	require.NoError(t, err)
	assert.Equal(t, 2, found2.Status, "Suspended player should have status 2")

	// Now test filtering
	players, total, err := model.List(ctx, ListPlayersOptions{
		Page:     1,
		PageSize: 10,
		Status:   &active,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1), "Should find at least 1 active player")
	for _, p := range players {
		assert.Equal(t, 1, p.Status)
	}

	players, total, err = model.List(ctx, ListPlayersOptions{
		Page:     1,
		PageSize: 10,
		Status:   &suspended,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1), "Should find at least 1 suspended player")
	for _, p := range players {
		assert.Equal(t, 2, p.Status)
	}
}

func TestPlayerModel_ValidatePassword(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "pwdplayer",
		GameID:   "game1",
		Status:   1,
	}
	err := model.Create(ctx, player, "correctpassword")
	require.NoError(t, err)

	// Test correct password
	found, err := model.ValidatePassword(ctx, "pwdplayer", "correctpassword", "game1")
	require.NoError(t, err)
	assert.Equal(t, player.ID, found.ID)

	// Test incorrect password
	_, err = model.ValidatePassword(ctx, "pwdplayer", "wrongpassword", "game1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid password")

	// Test non-existent user
	_, err = model.ValidatePassword(ctx, "nonexistent", "password", "game1")
	assert.Error(t, err)
}

func TestPlayerModel_UpdatePassword(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "updatepwdplayer",
		GameID:   "game1",
		Status:   1,
	}
	err := model.Create(ctx, player, "oldpassword")
	require.NoError(t, err)

	err = model.UpdatePassword(ctx, player.ID, "newpassword")
	require.NoError(t, err)

	// Verify new password works
	_, err = model.ValidatePassword(ctx, "updatepwdplayer", "newpassword", "game1")
	require.NoError(t, err)

	// Verify old password doesn't work
	_, err = model.ValidatePassword(ctx, "updatepwdplayer", "oldpassword", "game1")
	assert.Error(t, err)
}

func TestPlayerModel_UpdateBalance(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "balanceplayer",
		GameID:   "game1",
		Status:   1,
		Balance:  1000,
	}
	err := model.Create(ctx, player, "password")
	require.NoError(t, err)

	// Add balance
	updated, err := model.UpdateBalance(ctx, player.ID, 500, "test")
	require.NoError(t, err)
	assert.Equal(t, int64(1500), updated.Balance)

	// Deduct balance
	updated, err = model.UpdateBalance(ctx, player.ID, -200, "test")
	require.NoError(t, err)
	assert.Equal(t, int64(1300), updated.Balance)
}

func TestPlayerModel_UpdateBalance_Insufficient(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "insufficientplayer",
		GameID:   "game1",
		Status:   1,
		Balance:  100,
	}
	err := model.Create(ctx, player, "password")
	require.NoError(t, err)

	// Try to deduct more than available
	_, err = model.UpdateBalance(ctx, player.ID, -200, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient balance")
}

func TestPlayerModel_BanPlayer(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "banplayer",
		GameID:   "game1",
		Status:   1,
	}
	err := model.Create(ctx, player, "password")
	require.NoError(t, err)

	err = model.BanPlayer(ctx, player.ID)
	require.NoError(t, err)

	found, err := model.FindOne(ctx, player.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, found.Status)
}

func TestPlayerModel_SuspendPlayer(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "suspendplayer",
		GameID:   "game1",
		Status:   1,
	}
	err := model.Create(ctx, player, "password")
	require.NoError(t, err)

	err = model.SuspendPlayer(ctx, player.ID)
	require.NoError(t, err)

	found, err := model.FindOne(ctx, player.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, found.Status)
}

func TestPlayerModel_ActivatePlayer(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{
		Username: "activateplayer",
		GameID:   "game1",
		Status:   0,
	}
	err := model.Create(ctx, player, "password")
	require.NoError(t, err)

	err = model.ActivatePlayer(ctx, player.ID)
	require.NoError(t, err)

	found, err := model.FindOne(ctx, player.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, found.Status)
}

func TestPlayerModel_CountNewPlayers(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player1 := &Player{Username: "newplayer1", GameID: "game1", Status: 1}
	player2 := &Player{Username: "newplayer2", GameID: "game1", Status: 1}
	err := model.Create(ctx, player1, "password")
	require.NoError(t, err)
	err = model.Create(ctx, player2, "password")
	require.NoError(t, err)

	count, err := model.CountNewPlayers(ctx, "game1", time.Time{}, time.Now())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(2))
}

func TestPlayerModel_DailyNewPlayers(t *testing.T) {
	db := setupTestDB(t)
	model := NewPlayerModel(db)
	ctx := context.Background()

	player := &Player{Username: "dailyplayer", GameID: "game1", Status: 1}
	err := model.Create(ctx, player, "password")
	require.NoError(t, err)

	stats, err := model.DailyNewPlayers(ctx, "game1", time.Now().Add(-24*time.Hour), time.Now())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(stats), 1)
}

// ===== Player Helper Methods Tests =====

func TestPlayer_IsActive(t *testing.T) {
	player := &Player{Status: 1}
	assert.True(t, player.IsActive())

	player.Status = 0
	assert.False(t, player.IsActive())
}

func TestPlayer_IsBanned(t *testing.T) {
	player := &Player{Status: 0}
	assert.True(t, player.IsBanned())

	player.Status = 1
	assert.False(t, player.IsBanned())
}

func TestPlayer_IsSuspended(t *testing.T) {
	player := &Player{Status: 2}
	assert.True(t, player.IsSuspended())

	player.Status = 1
	assert.False(t, player.IsSuspended())
}

func TestPlayer_Ban(t *testing.T) {
	player := &Player{Status: 1}
	player.Ban()
	assert.Equal(t, 0, player.Status)
}

func TestPlayer_Suspend(t *testing.T) {
	player := &Player{Status: 1}
	player.Suspend()
	assert.Equal(t, 2, player.Status)
}

func TestPlayer_Activate(t *testing.T) {
	player := &Player{Status: 0}
	player.Activate()
	assert.Equal(t, 1, player.Status)
}

// ===== FunctionModel Tests =====
