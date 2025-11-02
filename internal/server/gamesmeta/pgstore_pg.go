//go:build pg

package gamesmeta

import (
    gpostgres "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

type PGStore struct{ db *gorm.DB }

func NewPGStore(dsn string) (Store, error) {
    db, err := gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
    if err != nil { return nil, err }
    if err := db.AutoMigrate(&Game{}); err != nil { return nil, err }
    return &PGStore{db: db}, nil
}

func (s *PGStore) Upsert(g Game) error { return s.db.Clauses(onConflictByID()).Create(&g).Error }
func (s *PGStore) Get(id string) (*Game, error) { var g Game; if err := s.db.First(&g, "id = ?", id).Error; err != nil { return nil, err }; return &g, nil }
func (s *PGStore) List() ([]*Game, error) { var arr []*Game; if err := s.db.Order("created_at DESC").Find(&arr).Error; err != nil { return nil, err }; return arr, nil }
func (s *PGStore) Delete(id string) error { return s.db.Delete(&Game{}, "id = ?", id).Error }

