package profile

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestGetUserGamesReturnsAllGamesForAdmin(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	roleModel := model.NewRoleModel(db)
	svc := NewService(adminModel, gameModel)

	admin := &model.Admin{Username: "admin", Status: 1}
	if err := adminModel.Create(context.Background(), admin, "admin123"); err != nil {
		t.Fatalf("create admin failed: %v", err)
	}
	role := &model.Role{Name: "admin"}
	if err := roleModel.Create(context.Background(), role); err != nil {
		t.Fatalf("create role failed: %v", err)
	}
	if err := adminModel.AssignRole(context.Background(), admin.ID, role.ID); err != nil {
		t.Fatalf("assign role failed: %v", err)
	}

	game := &model.Game{Name: "default", AliasName: "Default Game", Color: "#8c8c8c"}
	if err := game.SetEnvs([]model.GameEnv{{Env: "prod"}, {Env: "dev"}}); err != nil {
		t.Fatalf("set envs failed: %v", err)
	}
	if err := gameModel.Create(context.Background(), game); err != nil {
		t.Fatalf("create game failed: %v", err)
	}

	resp, err := svc.GetUserGames(context.Background(), "admin")
	if err != nil {
		t.Fatalf("GetUserGames failed: %v", err)
	}
	if len(resp.Games) != 1 {
		t.Fatalf("expected 1 game, got %d", len(resp.Games))
	}
	if resp.Games[0].GameId != "default" {
		t.Fatalf("expected gameId=default, got %q", resp.Games[0].GameId)
	}
	if resp.Games[0].GameName != "Default Game" {
		t.Fatalf("expected gameName=Default Game, got %q", resp.Games[0].GameName)
	}
	if len(resp.Games[0].Envs) != 2 {
		t.Fatalf("expected envs to be populated, got %#v", resp.Games[0].Envs)
	}
}

func TestGetUserGamesReturnsAllGamesWhenScopeIsEmpty(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	adminModel := model.NewAdminModel(db)
	gameModel := model.NewGameModel(db)
	svc := NewService(adminModel, gameModel)

	admin := &model.Admin{Username: "viewer", Status: 1}
	if err := adminModel.Create(context.Background(), admin, "viewer123"); err != nil {
		t.Fatalf("create admin failed: %v", err)
	}

	game := &model.Game{Name: "default", AliasName: "Default Game"}
	if err := game.SetEnvs([]model.GameEnv{{Env: "prod"}}); err != nil {
		t.Fatalf("set envs failed: %v", err)
	}
	if err := gameModel.Create(context.Background(), game); err != nil {
		t.Fatalf("create game failed: %v", err)
	}

	resp, err := svc.GetUserGames(context.Background(), "viewer")
	if err != nil {
		t.Fatalf("GetUserGames failed: %v", err)
	}
	if len(resp.Games) != 1 {
		t.Fatalf("expected 1 fallback game, got %d", len(resp.Games))
	}
	if resp.Games[0].GameId != "default" {
		t.Fatalf("expected fallback gameId=default, got %q", resp.Games[0].GameId)
	}
}
