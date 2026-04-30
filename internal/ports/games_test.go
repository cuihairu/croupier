// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package ports

import (
	"context"
	"testing"
	"time"
)

func TestGameZeroValue(t *testing.T) {
	var g Game
	if g.ID != 0 {
		t.Errorf("Game zero value ID should be 0, got %d", g.ID)
	}
	if g.Name != "" {
		t.Errorf("Game zero value Name should be empty, got %s", g.Name)
	}
	if g.Enabled {
		t.Error("Game zero value Enabled should be false")
	}
	if g.Envs != nil {
		t.Error("Game zero value Envs should be nil")
	}
}

func TestGameFields(t *testing.T) {
	now := time.Now()
	g := Game{
		ID:          123,
		Name:        "Test Game",
		Icon:        "test.png",
		Description: "A test game",
		Enabled:     true,
		AliasName:   "test-alias",
		Homepage:    "https://example.com",
		Status:      "active",
		GameType:    "rpg",
		GenreCode:   "fantasy",
		Color:       "#FF0000",
		Envs:        []string{"dev", "prod"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if g.ID != 123 {
		t.Errorf("ID = %d, want 123", g.ID)
	}
	if g.Name != "Test Game" {
		t.Errorf(`Name = %s, want "Test Game"`, g.Name)
	}
	if !g.Enabled {
		t.Error("Enabled should be true")
	}
	if len(g.Envs) != 2 {
		t.Errorf("Envs length = %d, want 2", len(g.Envs))
	}
}

func TestGameEnvDefZeroValue(t *testing.T) {
	var e GameEnvDef
	if e.Env != "" {
		t.Errorf("GameEnvDef zero value Env should be empty, got %s", e.Env)
	}
	if e.Description != "" {
		t.Errorf("GameEnvDef zero value Description should be empty, got %s", e.Description)
	}
	if e.Color != "" {
		t.Errorf("GameEnvDef zero value Color should be empty, got %s", e.Color)
	}
}

func TestGameEnvDefFields(t *testing.T) {
	e := GameEnvDef{
		Env:         "production",
		Description: "Production environment",
		Color:       "#00FF00",
	}

	if e.Env != "production" {
		t.Errorf(`Env = %s, want "production"`, e.Env)
	}
	if e.Description != "Production environment" {
		t.Errorf(`Description = %s, want "Production environment"`, e.Description)
	}
	if e.Color != "#00FF00" {
		t.Errorf(`Color = %s, want "#00FF00"`, e.Color)
	}
}

// mockGamesRepository implements GamesRepository for testing
type mockGamesRepository struct {
	createFunc   func(ctx context.Context, g *Game) error
	updateFunc   func(ctx context.Context, g *Game) error
	deleteFunc   func(ctx context.Context, id uint) error
	getFunc      func(ctx context.Context, id uint) (*Game, error)
	listFunc     func(ctx context.Context) ([]*Game, error)
	listEnvsFunc func(ctx context.Context, gameID uint) ([]string, error)
}

func (m *mockGamesRepository) Create(ctx context.Context, g *Game) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, g)
	}
	return nil
}

func (m *mockGamesRepository) Update(ctx context.Context, g *Game) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, g)
	}
	return nil
}

func (m *mockGamesRepository) Delete(ctx context.Context, id uint) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockGamesRepository) Get(ctx context.Context, id uint) (*Game, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockGamesRepository) List(ctx context.Context) ([]*Game, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx)
	}
	return nil, nil
}

func (m *mockGamesRepository) ListEnvs(ctx context.Context, gameID uint) ([]string, error) {
	if m.listEnvsFunc != nil {
		return m.listEnvsFunc(ctx, gameID)
	}
	return nil, nil
}

func (m *mockGamesRepository) ListEnvRecords(ctx context.Context, gameID uint) ([]*GameEnvDef, error) {
	return nil, nil
}

func (m *mockGamesRepository) AddEnv(ctx context.Context, gameID uint, env string) error {
	return nil
}

func (m *mockGamesRepository) AddEnvWithMeta(ctx context.Context, gameID uint, env, desc, color string) error {
	return nil
}

func (m *mockGamesRepository) UpdateEnv(ctx context.Context, gameID uint, oldEnv, newEnv, desc, color string) error {
	return nil
}

func (m *mockGamesRepository) RemoveEnv(ctx context.Context, gameID uint, env string) error {
	return nil
}

func TestGamesRepositoryInterface(t *testing.T) {
	// Verify mock implements the interface
	var _ GamesRepository = &mockGamesRepository{}

	ctx := context.Background()
	repo := &mockGamesRepository{}

	// Test Create doesn't panic
	if err := repo.Create(ctx, &Game{}); err != nil {
		t.Errorf("Create() error = %v", err)
	}

	// Test Get doesn't panic
	g, err := repo.Get(ctx, 1)
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if g != nil {
		t.Error("Get() returned non-nil game from mock")
	}
}

func TestGamesRepositoryMockWithCallbacks(t *testing.T) {
	ctx := context.Background()
	expectedGame := &Game{ID: 1, Name: "Test"}

	repo := &mockGamesRepository{
		getFunc: func(ctx context.Context, id uint) (*Game, error) {
			if id != 1 {
				t.Errorf("getFunc called with id = %d, want 1", id)
			}
			return expectedGame, nil
		},
		createFunc: func(ctx context.Context, g *Game) error {
			if g.Name != "New Game" {
				t.Errorf(`createFunc called with Name = %s, want "New Game"`, g.Name)
			}
			return nil
		},
	}

	// Test getFunc callback
	g, err := repo.Get(ctx, 1)
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if g != expectedGame {
		t.Error("Get() didn't return expected game")
	}

	// Test createFunc callback
	if err := repo.Create(ctx, &Game{Name: "New Game"}); err != nil {
		t.Errorf("Create() error = %v", err)
	}
}
