package logic

import (
    "context"
    "strings"

    "github.com/cuihairu/croupier/internal/ports"
    "github.com/cuihairu/croupier/services/api/internal/svc"
    "github.com/cuihairu/croupier/services/api/internal/types"

    "github.com/zeromicro/go-zero/core/logx"
)

type MeLogic struct {
    logx.Logger
    ctx    context.Context
    svcCtx *svc.ServiceContext
}

func NewMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MeLogic {
    return &MeLogic{
        Logger: logx.WithContext(ctx),
        ctx:    ctx,
        svcCtx: svcCtx,
    }
}

func (l *MeLogic) Profile(username string) (*types.MeProfileResponse, error) {
    repo := l.svcCtx.UserRepository()
    if repo == nil {
        return nil, ErrUnavailable
    }
    user, err := repo.GetUserByUsername(l.ctx, username)
    if err != nil {
        return nil, ErrNotFound
    }
    roles, err := repo.ListUserRoles(l.ctx, user.ID)
    if err != nil {
        return nil, err
    }
    return &types.MeProfileResponse{
        Username:    user.Username,
        DisplayName: user.DisplayName,
        Email:       user.Email,
        Phone:       user.Phone,
        Active:      user.Active,
        Roles:       roles,
    }, nil
}

func (l *MeLogic) Games(username string) (*types.MeGamesResponse, error) {
    repo := l.svcCtx.UserRepository()
    gamesRepo := l.svcCtx.GamesRepository()
    if repo == nil || gamesRepo == nil {
        return nil, ErrUnavailable
    }
    user, err := repo.GetUserByUsername(l.ctx, username)
    if err != nil {
        return nil, ErrNotFound
    }
    ids, err := repo.ListUserGameIDs(l.ctx, user.ID)
    if err != nil {
        return nil, err
    }
    allowedAll := len(ids) == 0
    allowedGames := map[uint]struct{}{}
    for _, id := range ids {
        allowedGames[id] = struct{}{}
    }
    games, err := gamesRepo.List(l.ctx)
    if err != nil {
        return nil, err
    }
    out := make([]types.MeGame, 0, len(games))
    for _, g := range games {
        if !allowedAll {
            if _, ok := allowedGames[g.ID]; !ok {
                continue
            }
        }
        envs := append([]string{}, g.Envs...)
        envRecs, _ := gamesRepo.ListEnvRecords(l.ctx, g.ID)
        allowedEnvs, _ := repo.ListUserGameEnvs(l.ctx, user.ID, g.ID)
        if len(allowedEnvs) > 0 {
            envs = filterEnvs(envs, allowedEnvs)
            envRecs = filterEnvRecords(envRecs, allowedEnvs)
        }
        out = append(out, types.MeGame{
            Id:          uint(g.ID),
            Name:        g.Name,
            AliasName:   g.AliasName,
            Status:      g.Status,
            Enabled:     g.Enabled,
            Description: g.Description,
            Icon:        g.Icon,
            Homepage:    g.Homepage,
            Envs:        envs,
            GameEnvs:    convertEnvRecords(envRecs),
        })
    }
    return &types.MeGamesResponse{Games: out}, nil
}

func (l *MeLogic) UpdateProfile(username string, req *types.MeProfileUpdateRequest) error {
    if req == nil {
        return ErrInvalidRequest
    }
    repo := l.svcCtx.UserRepository()
    if repo == nil {
        return ErrUnavailable
    }
    user, err := repo.GetUserByUsername(l.ctx, username)
    if err != nil {
        return ErrNotFound
    }
    changed := false
    if v := strings.TrimSpace(req.DisplayName); v != "" && v != user.DisplayName {
        user.DisplayName = v
        changed = true
    }
    if req.Email != "" && req.Email != user.Email {
        user.Email = req.Email
        changed = true
    }
    if req.Phone != "" && req.Phone != user.Phone {
        user.Phone = req.Phone
        changed = true
    }
    if !changed {
        return nil
    }
    return repo.UpdateUser(l.ctx, user)
}

func (l *MeLogic) UpdatePassword(username string, req *types.MePasswordRequest) error {
    if req == nil || strings.TrimSpace(req.Password) == "" {
        return ErrInvalidRequest
    }
    repo := l.svcCtx.UserRepository()
    if repo == nil {
        return ErrUnavailable
    }
    if _, err := repo.Verify(l.ctx, username, strings.TrimSpace(req.Current)); err != nil {
        return ErrUnauthorized
    }
    user, err := repo.GetUserByUsername(l.ctx, username)
    if err != nil {
        return ErrNotFound
    }
    return repo.SetPassword(l.ctx, user.ID, strings.TrimSpace(req.Password))
}

func filterEnvs(envs []string, allowed []string) []string {
    if len(allowed) == 0 {
        return envs
    }
    set := make(map[string]struct{}, len(allowed))
    for _, e := range allowed {
        set[strings.ToLower(strings.TrimSpace(e))] = struct{}{}
    }
    kept := make([]string, 0, len(envs))
    for _, env := range envs {
        if _, ok := set[strings.ToLower(strings.TrimSpace(env))]; ok {
            kept = append(kept, env)
        }
    }
    return kept
}

func filterEnvRecords(envs []*ports.GameEnvDef, allowed []string) []*ports.GameEnvDef {
    if len(allowed) == 0 {
        return envs
    }
    set := make(map[string]struct{}, len(allowed))
    for _, e := range allowed {
        set[strings.ToLower(strings.TrimSpace(e))] = struct{}{}
    }
    kept := make([]*ports.GameEnvDef, 0, len(envs))
    for _, env := range envs {
        if env == nil {
            continue
        }
        if _, ok := set[strings.ToLower(strings.TrimSpace(env.Env))]; ok {
            kept = append(kept, env)
        }
    }
    return kept
}

func convertEnvRecords(envs []*ports.GameEnvDef) []types.MeGameEnv {
    out := make([]types.MeGameEnv, 0, len(envs))
    for _, env := range envs {
        if env == nil {
            continue
        }
        out = append(out, types.MeGameEnv{Env: env.Env, Description: env.Description, Color: env.Color})
    }
    return out
}
