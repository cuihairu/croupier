package gamesmeta

import (
    "errors"
    "time"
)

// Game holds metadata for a game title.
type Game struct {
    ID          string    `json:"game_id" gorm:"primaryKey;column:id;type:text"`
    Name        string    `json:"name"`
    Icon        string    `json:"icon"`
    Description string    `json:"description"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type Store interface {
    Upsert(g Game) error
    Get(id string) (*Game, error)
    List() ([]*Game, error)
    Delete(id string) error
}

// onConflictByID is implemented in clause.go using gorm/clause.

// ErrDriverUnavailable is returned when a requested driver is not built in.
func ErrDriverUnavailable(driver string) error { return errors.New("driver unavailable: " + driver) }
