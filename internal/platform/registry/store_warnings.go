package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// DeleteRegistrationWarning 删除单条注册警告（key 为去重主键）。
func (s *Store) DeleteRegistrationWarning(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.registrationWarnings[key]; !ok {
		return false
	}
	delete(s.registrationWarnings, key)
	return true
}

// DeleteAllRegistrationWarnings 清空注册警告（「全部清除」UI）。
// 返回清除条数。
func (s *Store) DeleteAllRegistrationWarnings() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.registrationWarnings)
	s.registrationWarnings = map[string]*FunctionRegistrationWarning{}
	return n
}

// MarkRegistrationWarningRead 标记单条警告已读。
func (s *Store) MarkRegistrationWarningRead(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.registrationWarnings[key]
	if !ok || item == nil {
		return false
	}
	item.Read = true
	return true
}

// MarkAllRegistrationWarningsRead 全部标记已读，返回条数。
func (s *Store) MarkAllRegistrationWarningsRead() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, item := range s.registrationWarnings {
		if item != nil && !item.Read {
			item.Read = true
			n++
		}
	}
	return n
}

// 设计债清理：原签名声明 error 但纯内存实现恒返 nil，已删除 error 返回值。
func (s *Store) UpsertRegistrationWarning(ctx context.Context, item FunctionRegistrationWarning) {
	if item.Message == "" {
		return
	}
	key := item.Key
	if key == "" {
		// Use SHA-256 hash for fixed-length key, include game_id and env for isolation
		h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%s|%s", item.GameID, item.Env, item.AgentID, item.FunctionID, item.Code, item.Message)))
		key = hex.EncodeToString(h[:16]) // Use first 16 bytes (32 hex chars) for brevity
	}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.registrationWarnings == nil {
		s.registrationWarnings = make(map[string]*FunctionRegistrationWarning)
	}
	existing, ok := s.registrationWarnings[key]
	if !ok || existing == nil {
		cp := item
		cp.Key = key
		if cp.FirstSeen.IsZero() {
			cp.FirstSeen = now
		}
		if cp.LastSeen.IsZero() {
			cp.LastSeen = cp.FirstSeen
		}
		if cp.Count <= 0 {
			cp.Count = 1
		}
		s.registrationWarnings[key] = &cp
		return
	}
	existing.Count++
	existing.LastSeen = now
	if existing.Version == "" && item.Version != "" {
		existing.Version = item.Version
	}
	if existing.AgentID == "" && item.AgentID != "" {
		existing.AgentID = item.AgentID
	}
	if existing.FunctionID == "" && item.FunctionID != "" {
		existing.FunctionID = item.FunctionID
	}
}

func (s *Store) ListRegistrationWarnings(filter RegistrationWarningFilter) []FunctionRegistrationWarning {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]FunctionRegistrationWarning, 0, len(s.registrationWarnings))
	for _, item := range s.registrationWarnings {
		if item == nil {
			continue
		}
		if filter.GameID != "" && item.GameID != filter.GameID {
			continue
		}
		if filter.Env != "" && item.Env != filter.Env {
			continue
		}
		if filter.AgentID != "" && item.AgentID != filter.AgentID {
			continue
		}
		if filter.FunctionID != "" && item.FunctionID != filter.FunctionID {
			continue
		}
		if filter.Code != "" && item.Code != filter.Code {
			continue
		}
		out = append(out, *item)
	}
	// Sort by most recent warnings first.
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastSeen.After(out[j].LastSeen)
	})
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out
}

// RemoveRegistrationWarnings 删除匹配的注册警告（修复闭环：如 provider
// scope 修正后再次注册一致时，清除历史 mismatch 警告——业务层警告的
// 生命周期跟随注册行为，不借运维告警状态机）。
func (s *Store) RemoveRegistrationWarnings(filter RegistrationWarningFilter) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for key, item := range s.registrationWarnings {
		if item == nil {
			continue
		}
		if filter.GameID != "" && item.GameID != filter.GameID {
			continue
		}
		if filter.Env != "" && item.Env != filter.Env {
			continue
		}
		if filter.AgentID != "" && item.AgentID != filter.AgentID {
			continue
		}
		if filter.FunctionID != "" && item.FunctionID != filter.FunctionID {
			continue
		}
		if filter.Code != "" && item.Code != filter.Code {
			continue
		}
		delete(s.registrationWarnings, key)
		removed++
	}
	return removed
}
