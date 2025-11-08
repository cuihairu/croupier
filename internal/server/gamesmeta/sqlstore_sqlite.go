//go:build sqlite

package gamesmeta

import (
	gsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type SQLiteStore struct{ db *gorm.DB }

func NewSQLiteStore(dsn string) (Store, error) {
	db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Game{}); err != nil {
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Upsert(g Game) error { return s.db.Clauses(onConflictByID()).Create(&g).Error }
func (s *SQLiteStore) GetByGameID(id string) (*Game, error) {
	var g Game
	if err := s.db.Where("game_id = ?", id).First(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}
func (s *SQLiteStore) List() ([]*Game, error) {
	var arr []*Game
	if err := s.db.Order("updated_at DESC").Find(&arr).Error; err != nil {
		return nil, err
	}
	return arr, nil
}
func (s *SQLiteStore) DeleteByGameID(id string) error {
	return s.db.Where("game_id = ?", id).Delete(&Game{}).Error
}
