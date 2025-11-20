package svc

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/ports"
)

type memoryGamesRepo struct {
	mu    sync.Mutex
	next  uint
	games map[uint]*ports.Game
	envs  map[uint]map[string]ports.GameEnvDef
}

func newMemoryGamesRepo() *memoryGamesRepo {
	return &memoryGamesRepo{
		next:  1,
		games: make(map[uint]*ports.Game),
		envs:  make(map[uint]map[string]ports.GameEnvDef),
	}
}

func (m *memoryGamesRepo) Create(ctx context.Context, g *ports.Game) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.next
	m.next++
	now := time.Now()
	cp := clonePortsGame(g)
	cp.ID = id
	cp.CreatedAt = now
	cp.UpdatedAt = now
	m.games[id] = cp
	if g != nil {
		g.ID = cp.ID
		g.CreatedAt = cp.CreatedAt
		g.UpdatedAt = cp.UpdatedAt
	}
	return nil
}

func (m *memoryGamesRepo) Update(ctx context.Context, g *ports.Game) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.games[g.ID]; ok {
		cp := clonePortsGame(g)
		cp.UpdatedAt = time.Now()
		cp.CreatedAt = existing.CreatedAt
		m.games[g.ID] = cp
		return nil
	}
	return ErrGameNotFound
}

func (m *memoryGamesRepo) Delete(ctx context.Context, id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.games, id)
	delete(m.envs, id)
	return nil
}

func (m *memoryGamesRepo) Get(ctx context.Context, id uint) (*ports.Game, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if g, ok := m.games[id]; ok {
		cp := clonePortsGame(g)
		if envs := m.envs[id]; envs != nil {
			cp.Envs = collectEnvNames(envs)
		}
		return cp, nil
	}
	return nil, ErrGameNotFound
}

func (m *memoryGamesRepo) List(ctx context.Context) ([]*ports.Game, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*ports.Game, 0, len(m.games))
	for _, g := range m.games {
		cp := clonePortsGame(g)
		if envs := m.envs[g.ID]; envs != nil {
			cp.Envs = collectEnvNames(envs)
		}
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (m *memoryGamesRepo) ListEnvs(ctx context.Context, gameID uint) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if envs := m.envs[gameID]; envs != nil {
		return collectEnvNames(envs), nil
	}
	return []string{}, nil
}

func (m *memoryGamesRepo) ListEnvRecords(ctx context.Context, gameID uint) ([]*ports.GameEnvDef, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	envs := m.envs[gameID]
	out := make([]*ports.GameEnvDef, 0, len(envs))
	for _, def := range envs {
		cp := def
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Env < out[j].Env })
	return out, nil
}

func (m *memoryGamesRepo) AddEnv(ctx context.Context, gameID uint, env string) error {
	return m.AddEnvWithMeta(ctx, gameID, env, "", "")
}

func (m *memoryGamesRepo) AddEnvWithMeta(ctx context.Context, gameID uint, env, desc, color string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	env = strings.TrimSpace(env)
	if env == "" {
		return ErrGameInvalidEnv
	}
	if _, ok := m.games[gameID]; !ok {
		return ErrGameNotFound
	}
	if m.envs[gameID] == nil {
		m.envs[gameID] = map[string]ports.GameEnvDef{}
	}
	m.envs[gameID][env] = ports.GameEnvDef{Env: env, Description: desc, Color: color}
	return nil
}

func (m *memoryGamesRepo) UpdateEnv(ctx context.Context, gameID uint, oldEnv, newEnv, desc, color string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if envs := m.envs[gameID]; envs != nil {
		delete(envs, oldEnv)
		if strings.TrimSpace(newEnv) != "" {
			envs[strings.TrimSpace(newEnv)] = ports.GameEnvDef{
				Env:         strings.TrimSpace(newEnv),
				Description: strings.TrimSpace(desc),
				Color:       strings.TrimSpace(color),
			}
		}
	}
	return nil
}

func (m *memoryGamesRepo) RemoveEnv(ctx context.Context, gameID uint, env string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if envs := m.envs[gameID]; envs != nil {
		delete(envs, env)
	}
	return nil
}

func clonePortsGame(g *ports.Game) *ports.Game {
	if g == nil {
		return nil
	}
	cp := *g
	if len(g.Envs) > 0 {
		cp.Envs = append([]string{}, g.Envs...)
	}
	return &cp
}

func collectEnvNames(envs map[string]ports.GameEnvDef) []string {
	out := make([]string, 0, len(envs))
	for env := range envs {
		out = append(out, env)
	}
	sort.Strings(out)
	return out
}

var (
	ErrGameNotFound   = errors.New("game not found")
	ErrGameInvalidEnv = errors.New("invalid env")
)
