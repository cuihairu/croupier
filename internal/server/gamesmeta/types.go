package gamesmeta

import (
	"errors"
	"gorm.io/gorm"
	"time"
)

// Game holds metadata for a game title.
// Use gorm.Model's ID (uint) as primary key per user's preference.
// Keep a business key GameID as unique for external references.
type Game struct {
	gorm.Model
	GameID      string    `json:"game_id" gorm:"column:game_id;size:64;uniqueIndex;not null"`
	Name        string    `json:"name"`
	Icon        string    `json:"icon"`
	Description string    `json:"description" gorm:"type:text"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Store interface {
	Upsert(g Game) error
	GetByGameID(id string) (*Game, error)
	List() ([]*Game, error)
	DeleteByGameID(id string) error
}

// ErrDriverUnavailable is returned when a requested driver is not built in.
func ErrDriverUnavailable(driver string) error { return errors.New("driver unavailable: " + driver) }
