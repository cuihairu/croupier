package games

import (
	"context"
	"gorm.io/gorm"
)

type Repo struct{ db *gorm.DB }

func AutoMigrate(db *gorm.DB) error { return db.AutoMigrate(&Game{}, &GameEnv{}) }
func NewRepo(db *gorm.DB) *Repo     { return &Repo{db: db} }

// Games CRUD
func (r *Repo) Create(ctx context.Context, g *Game) error {
	return r.db.WithContext(ctx).Create(g).Error
}
func (r *Repo) Update(ctx context.Context, g *Game) error { return r.db.WithContext(ctx).Save(g).Error }
func (r *Repo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&Game{}, id).Error
}
func (r *Repo) Get(ctx context.Context, id uint) (*Game, error) {
	var g Game
	if err := r.db.WithContext(ctx).First(&g, id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}
func (r *Repo) List(ctx context.Context) ([]*Game, error) {
	var arr []*Game
	if err := r.db.WithContext(ctx).Order("updated_at DESC").Find(&arr).Error; err != nil {
		return nil, err
	}
	return arr, nil
}

// Env scopes
func (r *Repo) ListEnvs(ctx context.Context, gameID uint) ([]string, error) {
	var out []string
	if err := r.db.WithContext(ctx).Model(&GameEnv{}).Where("game_id=?", gameID).Pluck("env", &out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// List env records with id/env/description
func (r *Repo) ListEnvRecords(ctx context.Context, gameID uint) ([]*GameEnv, error) {
	var arr []*GameEnv
	if err := r.db.WithContext(ctx).Where("game_id=?", gameID).Order("env ASC").Find(&arr).Error; err != nil {
		return nil, err
	}
	return arr, nil
}

func (r *Repo) AddEnv(ctx context.Context, gameID uint, env string) error {
	return r.AddEnvWithDesc(ctx, gameID, env, "")
}
func (r *Repo) AddEnvWithDesc(ctx context.Context, gameID uint, env, desc string) error {
	ge := &GameEnv{GameID: gameID, Env: env, Description: desc}
	return r.db.WithContext(ctx).Create(ge).Error
}
func (r *Repo) UpdateEnv(ctx context.Context, gameID uint, oldEnv, newEnv, desc string) error {
	var ge GameEnv
	if err := r.db.WithContext(ctx).Where("game_id=? AND env=?", gameID, oldEnv).First(&ge).Error; err != nil {
		return err
	}
	if newEnv != "" {
		ge.Env = newEnv
	}
	ge.Description = desc
	return r.db.WithContext(ctx).Save(&ge).Error
}
func (r *Repo) RemoveEnv(ctx context.Context, gameID uint, env string) error {
	return r.db.WithContext(ctx).Where("game_id=? AND env=?", gameID, env).Delete(&GameEnv{}).Error
}
func (r *Repo) RemoveEnvByID(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&GameEnv{}, id).Error
}
