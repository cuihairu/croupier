package ports

import (
    "context"
    "time"
)

// Game is the domain DTO used by services/handlers. It mirrors the DB model but avoids GORM tags.
type Game struct {
    ID          uint
    Name        string
    Icon        string
    Description string
    Enabled     bool
    AliasName   string
    Homepage    string
    Status      string
    GameType    string
    GenreCode   string
    Envs        []string // list of env names (unique, case-insensitive)
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// GameEnvDef is the global environment definition (deduped by Env across the system).
type GameEnvDef struct {
    Env         string
    Description string
    Color       string
}

// GamesRepository defines persistence for games and their env lists.
type GamesRepository interface {
    // CRUD
    Create(ctx context.Context, g *Game) error
    Update(ctx context.Context, g *Game) error
    Delete(ctx context.Context, id uint) error
    Get(ctx context.Context, id uint) (*Game, error)
    List(ctx context.Context) ([]*Game, error)

    // Envs (per-game list) and EnvDefs (global dictionary)
    ListEnvs(ctx context.Context, gameID uint) ([]string, error)
    ListEnvRecords(ctx context.Context, gameID uint) ([]*GameEnvDef, error)
    AddEnv(ctx context.Context, gameID uint, env string) error
    AddEnvWithMeta(ctx context.Context, gameID uint, env, desc, color string) error
    UpdateEnv(ctx context.Context, gameID uint, oldEnv, newEnv, desc, color string) error
    RemoveEnv(ctx context.Context, gameID uint, env string) error
}

// UnitOfWork is reserved for future transactional boundaries.
type UnitOfWork interface {
    Do(ctx context.Context, fn func(ctx context.Context) error) error
}

