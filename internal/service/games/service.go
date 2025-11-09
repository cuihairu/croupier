package games

import (
    "context"
    "encoding/json"
    "os"
    "strings"

    dom "github.com/cuihairu/croupier/internal/ports"
)

type Service struct {
    repo        dom.GamesRepository
    defaultEnvs []dom.GameEnvDef
}

func NewService(repo dom.GamesRepository, defaults []dom.GameEnvDef) *Service {
    return &Service{repo: repo, defaultEnvs: defaults}
}

// LoadDefaultsFromFile parses configs/games.json for default_envs.
func LoadDefaultsFromFile(path string) ([]dom.GameEnvDef, error) {
    b, err := os.ReadFile(path)
    if err != nil { return nil, err }
    var cfg struct {
        DefaultEnvs []struct {
            Env         string `json:"env"`
            Description string `json:"description"`
            Color       string `json:"color"`
        } `json:"default_envs"`
    }
    if err := json.Unmarshal(b, &cfg); err != nil { return nil, err }
    out := make([]dom.GameEnvDef, 0, len(cfg.DefaultEnvs))
    for _, e := range cfg.DefaultEnvs {
        if strings.TrimSpace(e.Env) == "" { continue }
        out = append(out, dom.GameEnvDef{Env: e.Env, Description: e.Description, Color: e.Color})
    }
    return out, nil
}

// CreateGame persists the game and ensures default envs exist and are added.
func (s *Service) CreateGame(ctx context.Context, g *dom.Game) error {
    if err := s.repo.Create(ctx, g); err != nil { return err }
    for _, d := range s.defaultEnvs {
        // Create/update env definition and add to game list
        _ = s.repo.AddEnvWithMeta(ctx, g.ID, d.Env, d.Description, d.Color)
    }
    return nil
}

// UpdateGame forwards to repo.
func (s *Service) UpdateGame(ctx context.Context, g *dom.Game) error { return s.repo.Update(ctx, g) }
// DeleteGame removes a game by id.
func (s *Service) DeleteGame(ctx context.Context, id uint) error { return s.repo.Delete(ctx, id) }

// GetGame returns a single game.
func (s *Service) GetGame(ctx context.Context, id uint) (*dom.Game, error) { return s.repo.Get(ctx, id) }

// ListGames returns all games (ordered by updated_at desc via repo).
func (s *Service) ListGames(ctx context.Context) ([]*dom.Game, error) { return s.repo.List(ctx) }

// Envs operations
func (s *Service) AddEnv(ctx context.Context, gameID uint, env dom.GameEnvDef) error {
    return s.repo.AddEnvWithMeta(ctx, gameID, env.Env, env.Description, env.Color)
}
func (s *Service) UpdateEnv(ctx context.Context, gameID uint, oldEnv string, next dom.GameEnvDef) error {
    return s.repo.UpdateEnv(ctx, gameID, oldEnv, next.Env, next.Description, next.Color)
}
func (s *Service) RemoveEnv(ctx context.Context, gameID uint, env string) error {
    return s.repo.RemoveEnv(ctx, gameID, env)
}

// ListEnvRecords returns env definitions for a game's env list (deduped by env).
func (s *Service) ListEnvRecords(ctx context.Context, gameID uint) ([]dom.GameEnvDef, error) {
    recs, err := s.repo.ListEnvRecords(ctx, gameID)
    if err != nil { return nil, err }
    out := make([]dom.GameEnvDef, 0, len(recs))
    for _, e := range recs {
        if e == nil { continue }
        out = append(out, dom.GameEnvDef{Env: e.Env, Description: e.Description, Color: e.Color})
    }
    return out, nil
}
