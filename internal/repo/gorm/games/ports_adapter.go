package games

import (
    "context"

    dom "github.com/cuihairu/croupier/internal/ports"
    "gorm.io/gorm"
)

// PortRepo adapts *Repo to the ports.GamesRepository interface.
type PortRepo struct{ r *Repo }

func NewPortRepo(r *Repo) *PortRepo { return &PortRepo{r: r} }

var _ dom.GamesRepository = (*PortRepo)(nil)

func (p *PortRepo) Create(ctx context.Context, g *dom.Game) error {
    if g == nil { return nil }
    m := &Game{ Name: g.Name, Icon: g.Icon, Description: g.Description, Enabled: g.Enabled, AliasName: g.AliasName, Homepage: g.Homepage, Status: g.Status, GameType: g.GameType, GenreCode: g.GenreCode }
    if len(g.Envs) > 0 { m.SetEnvList(g.Envs) }
    if err := p.r.Create(ctx, m); err != nil { return err }
    g.ID = m.ID
    return nil
}
func (p *PortRepo) Update(ctx context.Context, g *dom.Game) error {
    if g == nil { return nil }
    m := &Game{ Model: gorm.Model{ID: g.ID}, Name: g.Name, Icon: g.Icon, Description: g.Description, Enabled: g.Enabled, AliasName: g.AliasName, Homepage: g.Homepage, Status: g.Status, GameType: g.GameType, GenreCode: g.GenreCode }
    if len(g.Envs) > 0 { m.SetEnvList(g.Envs) }
    return p.r.Update(ctx, m)
}
func (p *PortRepo) Delete(ctx context.Context, id uint) error { return p.r.Delete(ctx, id) }
func (p *PortRepo) Get(ctx context.Context, id uint) (*dom.Game, error) {
    mg, err := p.r.Get(ctx, id)
    if err != nil { return nil, err }
    return toDomain(mg), nil
}
func (p *PortRepo) List(ctx context.Context) ([]*dom.Game, error) {
    arr, err := p.r.List(ctx)
    if err != nil { return nil, err }
    out := make([]*dom.Game, 0, len(arr))
    for _, g := range arr { out = append(out, toDomain(g)) }
    return out, nil
}
func (p *PortRepo) ListEnvs(ctx context.Context, gameID uint) ([]string, error) { return p.r.ListEnvs(ctx, gameID) }
func (p *PortRepo) ListEnvRecords(ctx context.Context, gameID uint) ([]*dom.GameEnvDef, error) {
    recs, err := p.r.ListEnvRecords(ctx, gameID)
    if err != nil { return nil, err }
    out := make([]*dom.GameEnvDef, 0, len(recs))
    for _, e := range recs { out = append(out, &dom.GameEnvDef{Env: e.Env, Description: e.Description, Color: e.Color}) }
    return out, nil
}
func (p *PortRepo) AddEnv(ctx context.Context, gameID uint, env string) error { return p.r.AddEnv(ctx, gameID, env) }
func (p *PortRepo) AddEnvWithMeta(ctx context.Context, gameID uint, env, desc, color string) error {
    return p.r.AddEnvWithMeta(ctx, gameID, env, desc, color)
}
func (p *PortRepo) UpdateEnv(ctx context.Context, gameID uint, oldEnv, newEnv, desc, color string) error {
    return p.r.UpdateEnv(ctx, gameID, oldEnv, newEnv, desc, color)
}
func (p *PortRepo) RemoveEnv(ctx context.Context, gameID uint, env string) error { return p.r.RemoveEnv(ctx, gameID, env) }

// Helpers
func toDomain(g *Game) *dom.Game {
    if g == nil { return nil }
    return &dom.Game{
        ID:          g.ID,
        Name:        g.Name,
        Icon:        g.Icon,
        Description: g.Description,
        Enabled:     g.Enabled,
        AliasName:   g.AliasName,
        Homepage:    g.Homepage,
        Status:      g.Status,
        GameType:    g.GameType,
        GenreCode:   g.GenreCode,
        Envs:        g.GetEnvList(),
        CreatedAt:   g.CreatedAt,
        UpdatedAt:   g.UpdatedAt,
    }
}

// no-op helper kept to minimize patch churn in future


