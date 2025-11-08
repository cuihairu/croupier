package httpserver

import (
	gamesmeta "github.com/cuihairu/croupier/internal/server/gamesmeta"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gormGames struct{ db *gorm.DB }

func (s *gormGames) Upsert(g gamesmeta.Game) error {
	return s.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&g).Error
}
func (s *gormGames) Get(id string) (*gamesmeta.Game, error) {
	var g gamesmeta.Game
	if err := s.db.First(&g, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}
func (s *gormGames) List() ([]*gamesmeta.Game, error) {
	var arr []*gamesmeta.Game
	if err := s.db.Order("created_at DESC").Find(&arr).Error; err != nil {
		return nil, err
	}
	return arr, nil
}
func (s *gormGames) Delete(id string) error {
	return s.db.Delete(&gamesmeta.Game{}, "id = ?", id).Error
}
