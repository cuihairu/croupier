package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/cuihairu/croupier/agents/capture/gate"
)

// alertSpool stores unacknowledged gate hits on disk so a failed upload or
// process restart can resume delivery without losing the alert.
type alertSpool struct {
	path string

	limit int

	mu      sync.Mutex
	pending map[string]gate.Hit
	order   []string

	writeFile func(path string, data []byte) error
}

type alertSpoolFile struct {
	Entries []alertSpoolEntry `json:"entries"`
}

type alertSpoolEntry struct {
	Key string   `json:"key"`
	Hit gate.Hit `json:"hit"`
}

func newAlertSpool(path string) *alertSpool {
	s := &alertSpool{
		path:      path,
		limit:     alertSpoolLimit,
		pending:   make(map[string]gate.Hit),
		writeFile: writeFileAtomic,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	var file alertSpoolFile
	if json.Unmarshal(data, &file) == nil && len(file.Entries) > 0 {
		for _, entry := range file.Entries {
			if entry.Key == "" {
				continue
			}
			if _, exists := s.pending[entry.Key]; exists {
				continue
			}
			s.pending[entry.Key] = entry.Hit
			s.order = append(s.order, entry.Key)
		}
		return s
	}
	var legacy map[string]gate.Hit
	if json.Unmarshal(data, &legacy) == nil && len(legacy) > 0 {
		s.pending = legacy
		s.order = make([]string, 0, len(legacy))
		for key := range legacy {
			s.order = append(s.order, key)
		}
		sort.Strings(s.order)
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
	if len(s.pending) >= s.limit {
		return fmt.Errorf("alert spool full: %d/%d", len(s.pending), s.limit)
	}
	s.pending[key] = hit
	s.order = append(s.order, key)
	if err := s.saveLocked(); err != nil {
		delete(s.pending, key)
		s.order = s.order[:len(s.order)-1]
		return err
	}
	return nil
}

func (s *alertSpool) ack(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	oldHit := s.pending[key]
	oldOrder := append([]string(nil), s.order...)
	delete(s.pending, key)
	for index, pendingKey := range s.order {
		if pendingKey == key {
			s.order = append(s.order[:index], s.order[index+1:]...)
			break
		}
	}
	if err := s.saveLocked(); err != nil {
		s.pending[key] = oldHit
		s.order = oldOrder
		return err
	}
	return nil
}

func (s *alertSpool) hitForKey(key string) gate.Hit {
	for _, pendingKey := range s.order {
		if pendingKey == key {
			return s.pending[key]
		}
	}
	return gate.Hit{}
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
	file := alertSpoolFile{Entries: make([]alertSpoolEntry, 0, len(s.order))}
	for _, key := range s.order {
		if hit, exists := s.pending[key]; exists {
			file.Entries = append(file.Entries, alertSpoolEntry{Key: key, Hit: hit})
		}
	}
	data, err := json.Marshal(file)
	if err != nil {
		return err
	}
	if s.writeFile == nil {
		return errors.New("alert spool writer unavailable")
	}
	return s.writeFile(s.path, data)
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
	if err := tmp.Sync(); err != nil {
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
