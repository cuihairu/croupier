package games

import (
	"encoding/json"
	"os"
	"sync"
)

type Entry struct {
	GameID string `json:"game_id"`
	Env    string `json:"env,omitempty"`
}

type Store struct {
	mu   sync.RWMutex
	set  map[string]map[string]struct{} // gameID -> env -> exists (empty env means any)
	path string
}

func NewStore(path string) *Store {
	return &Store{set: map[string]map[string]struct{}{}, path: path}
}

func (s *Store) Load() error {
	if s.path == "" {
		return nil
	}
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var data struct {
		Games []Entry `json:"games"`
	}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	for _, e := range data.Games {
		s.Add(e.GameID, e.Env)
	}
	return nil
}

func (s *Store) Save() error {
	if s.path == "" {
		return nil
	}
	s.mu.RLock()
	var arr []Entry
	for g, m := range s.set {
		if len(m) == 0 {
			arr = append(arr, Entry{GameID: g})
		} else {
			for env := range m {
				arr = append(arr, Entry{GameID: g, Env: env})
			}
		}
	}
	s.mu.RUnlock()
	payload, _ := json.MarshalIndent(struct {
		Games []Entry `json:"games"`
	}{Games: arr}, "", "  ")
	if err := os.MkdirAll(dirOf(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, payload, 0o644)
}

func (s *Store) Add(gameID, env string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.set[gameID]
	if m == nil {
		m = map[string]struct{}{}
		s.set[gameID] = m
	}
	if env != "" {
		m[env] = struct{}{}
	}
}

func (s *Store) IsAllowed(gameID, env string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.set[gameID]
	if m == nil {
		return false
	}
	if len(m) == 0 {
		return true
	}
	if env == "" {
		return true
	} // any env allowed
	_, ok := m[env]
	return ok
}

func (s *Store) List() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Entry
	for g, m := range s.set {
		if len(m) == 0 {
			out = append(out, Entry{GameID: g})
		} else {
			for env := range m {
				out = append(out, Entry{GameID: g, Env: env})
			}
		}
	}
	return out
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
