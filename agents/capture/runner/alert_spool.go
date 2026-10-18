package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/cuihairu/croupier/agents/capture/gate"
)

// alertSpool stores unacknowledged gate hits on disk so a failed upload or
// process restart can resume delivery without losing the alert.
type alertSpool struct {
	path string

	mu      sync.Mutex
	pending map[string]gate.Hit
	order   []string
}

func newAlertSpool(path string) *alertSpool {
	s := &alertSpool{path: path, pending: make(map[string]gate.Hit)}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	var loaded map[string]gate.Hit
	if json.Unmarshal(data, &loaded) == nil && loaded != nil {
		s.pending = loaded
		s.order = make([]string, 0, len(loaded))
		for key := range loaded {
			s.order = append(s.order, key)
		}
	}
	return s
}

func (s *alertSpool) key(h gate.Hit) string {
	return fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s\x1f%d", h.Rule, h.Table, h.Key, h.Event.Position.Value, h.Event.TsUnixMs)
}

func (s *alertSpool) enqueue(hit gate.Hit) error {
	key := s.key(hit)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.pending[key]; exists {
		return nil
	}
	s.pending[key] = hit
	s.order = append(s.order, key)
	return s.saveLocked()
}

func (s *alertSpool) ack(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, key)
	for index, pendingKey := range s.order {
		if pendingKey == key {
			s.order = append(s.order[:index], s.order[index+1:]...)
			break
		}
	}
	return s.saveLocked()
}

func (s *alertSpool) next() (gate.Hit, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range s.order {
		if hit, exists := s.pending[key]; exists {
			return hit, true
		}
	}
	return gate.Hit{}, false
}

func (s *alertSpool) saveLocked() error {
	if len(s.pending) == 0 {
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	data, err := json.Marshal(s.pending)
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path, data)
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".alerts-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
