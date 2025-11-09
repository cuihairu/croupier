package games

import (
    "context"
    "sort"
    "strings"

    "gorm.io/gorm"
)

// Repo provides GORM-based persistence for games and their env lists/defs.
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
    g, err := r.Get(ctx, gameID)
    if err != nil { return nil, err }
    envs := g.GetEnvList()
    // normalize unique lower-case preserve original order
    seen := map[string]struct{}{}
    out := make([]string, 0, len(envs))
    for _, e := range envs {
        t := strings.TrimSpace(e)
        if t == "" { continue }
        k := strings.ToLower(t)
        if _, ok := seen[k]; ok { continue }
        seen[k] = struct{}{}
        out = append(out, t)
    }
    return out, nil
}

// List env records with env/description/color (deduped by env)
func (r *Repo) ListEnvRecords(ctx context.Context, gameID uint) ([]*GameEnv, error) {
    envs, err := r.ListEnvs(ctx, gameID)
    if err != nil { return nil, err }
    if len(envs) == 0 { return []*GameEnv{}, nil }
    var arr []*GameEnv
    if err := r.db.WithContext(ctx).Where("env IN ?", envs).Find(&arr).Error; err != nil {
        return nil, err
    }
    // sort by env name order in envs
    order := map[string]int{}
    for i, e := range envs { order[strings.ToLower(e)] = i }
    sort.Slice(arr, func(i, j int) bool { return order[strings.ToLower(arr[i].Env)] < order[strings.ToLower(arr[j].Env)] })
    return arr, nil
}

func (r *Repo) ensureEnvDef(ctx context.Context, env, desc, color string) error {
    var ge GameEnv
    if err := r.db.WithContext(ctx).Where("env=?", env).First(&ge).Error; err != nil {
        ge = GameEnv{Env: env, Description: strings.TrimSpace(desc), Color: strings.TrimSpace(color)}
        return r.db.WithContext(ctx).Create(&ge).Error
    }
    changed := false
    if d := strings.TrimSpace(desc); d != "" && strings.TrimSpace(ge.Description) != d { ge.Description = d; changed = true }
    if c := strings.TrimSpace(color); c != "" && strings.TrimSpace(ge.Color) != c { ge.Color = c; changed = true }
    if changed { return r.db.WithContext(ctx).Save(&ge).Error }
    return nil
}

func (r *Repo) AddEnv(ctx context.Context, gameID uint, env string) error { return r.AddEnvWithMeta(ctx, gameID, env, "", "") }
func (r *Repo) AddEnvWithDesc(ctx context.Context, gameID uint, env, desc string) error { return r.AddEnvWithMeta(ctx, gameID, env, desc, "") }
func (r *Repo) AddEnvWithMeta(ctx context.Context, gameID uint, env, desc, color string) error {
    env = strings.TrimSpace(env)
    if env == "" { return nil }
    // ensure env def exists/updated
    _ = r.ensureEnvDef(ctx, env, desc, color)
    // append to game's env list
    g, err := r.Get(ctx, gameID)
    if err != nil { return err }
    lst := g.GetEnvList()
    seen := map[string]struct{}{}
    for _, e := range lst { seen[strings.ToLower(strings.TrimSpace(e))] = struct{}{} }
    if _, ok := seen[strings.ToLower(env)]; !ok {
        lst = append(lst, env)
        g.SetEnvList(lst)
        return r.Update(ctx, g)
    }
    return nil
}

func (r *Repo) UpdateEnv(ctx context.Context, gameID uint, oldEnv, newEnv, desc, color string) error {
    oldEnv = strings.TrimSpace(oldEnv)
    newEnv = strings.TrimSpace(newEnv)
    g, err := r.Get(ctx, gameID)
    if err != nil { return err }
    lst := g.GetEnvList()
    changed := false
    if newEnv != "" && !strings.EqualFold(oldEnv, newEnv) {
        for i, e := range lst {
            if strings.EqualFold(strings.TrimSpace(e), oldEnv) { lst[i] = newEnv; changed = true }
        }
        if changed { g.SetEnvList(lst); if err := r.Update(ctx, g); err != nil { return err } }
        _ = r.ensureEnvDef(ctx, newEnv, desc, color)
    } else if strings.TrimSpace(desc) != "" || strings.TrimSpace(color) != "" {
        _ = r.ensureEnvDef(ctx, oldEnv, desc, color)
    }
    return nil
}

func (r *Repo) RemoveEnv(ctx context.Context, gameID uint, env string) error {
    env = strings.TrimSpace(env)
    g, err := r.Get(ctx, gameID)
    if err != nil { return err }
    lst := g.GetEnvList()
    next := make([]string, 0, len(lst))
    for _, e := range lst { if !strings.EqualFold(strings.TrimSpace(e), env) { next = append(next, e) } }
    g.SetEnvList(next)
    return r.Update(ctx, g)
}

// RemoveEnvByID kept for API compatibility but is a no-op for global env defs.
func (r *Repo) RemoveEnvByID(ctx context.Context, id uint) error { return nil }

