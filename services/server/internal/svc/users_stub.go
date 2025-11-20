package svc

import (
	"context"
	"errors"
	"strings"
	"sync"
)

type UserRecord struct {
	ID          uint
	Username    string
	DisplayName string
	Email       string
	Phone       string
	Active      bool
}

type UserRepository interface {
	Verify(ctx context.Context, username, password string) (*UserRecord, error)
	ListUserRoles(ctx context.Context, userID uint) ([]string, error)
	GetUserByUsername(ctx context.Context, username string) (*UserRecord, error)
	ListUserGameIDs(ctx context.Context, userID uint) ([]uint, error)
	ListUserGameEnvs(ctx context.Context, userID, gameID uint) ([]string, error)
	UpdateUser(ctx context.Context, user *UserRecord) error
	SetPassword(ctx context.Context, userID uint, password string) error
}

type userEntry struct {
	record   UserRecord
	password string
	roles    []string
	games    map[uint][]string
}

type memoryUserRepo struct {
	mu    sync.Mutex
	users map[string]*userEntry
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{
		users: map[string]*userEntry{
			"admin": {
				record: UserRecord{
					ID:          1,
					Username:    "admin",
					DisplayName: "Administrator",
					Email:       "admin@example.com",
					Phone:       "+1-555-0100",
					Active:      true,
				},
				password: "password",
				roles:    []string{"admin"},
				games:    map[uint][]string{},
			},
		},
	}
}

func (m *memoryUserRepo) lookup(username string) (*userEntry, bool) {
	key := strings.ToLower(strings.TrimSpace(username))
	acc, ok := m.users[key]
	return acc, ok
}

func cloneUserRecord(rec UserRecord) *UserRecord {
	cp := rec
	return &cp
}

func (m *memoryUserRepo) Verify(ctx context.Context, username, password string) (*UserRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	acc, ok := m.lookup(username)
	if !ok || acc.password != password {
		return nil, errors.New("invalid credentials")
	}
	return cloneUserRecord(acc.record), nil
}

func (m *memoryUserRepo) ListUserRoles(ctx context.Context, userID uint) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, acc := range m.users {
		if acc.record.ID == userID {
			return append([]string(nil), acc.roles...), nil
		}
	}
	return []string{}, nil
}

func (m *memoryUserRepo) GetUserByUsername(ctx context.Context, username string) (*UserRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	acc, ok := m.lookup(username)
	if !ok {
		return nil, errors.New("user not found")
	}
	return cloneUserRecord(acc.record), nil
}

func (m *memoryUserRepo) ListUserGameIDs(ctx context.Context, userID uint) ([]uint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, acc := range m.users {
		if acc.record.ID == userID {
			out := make([]uint, 0, len(acc.games))
			for gid := range acc.games {
				out = append(out, gid)
			}
			return out, nil
		}
	}
	return []uint{}, nil
}

func (m *memoryUserRepo) ListUserGameEnvs(ctx context.Context, userID, gameID uint) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, acc := range m.users {
		if acc.record.ID == userID {
			envs := acc.games[gameID]
			return append([]string(nil), envs...), nil
		}
	}
	return []string{}, nil
}

func (m *memoryUserRepo) UpdateUser(ctx context.Context, user *UserRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, acc := range m.users {
		if acc.record.ID == user.ID {
			if strings.TrimSpace(user.DisplayName) != "" {
				acc.record.DisplayName = user.DisplayName
			}
			acc.record.Email = strings.TrimSpace(user.Email)
			acc.record.Phone = strings.TrimSpace(user.Phone)
			acc.record.Active = user.Active
			return nil
		}
	}
	return errors.New("user not found")
}

func (m *memoryUserRepo) SetPassword(ctx context.Context, userID uint, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, acc := range m.users {
		if acc.record.ID == userID {
			acc.password = password
			return nil
		}
	}
	return errors.New("user not found")
}
