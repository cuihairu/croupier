package approvals

import (
    "errors"
    "sort"
    "strings"
    "sync"
    "time"
)

// Approval represents a two-person rule approval record.
type Approval struct {
    ID         string
    State      string // pending|approved|rejected
    FunctionID string
    GameID     string
    Env        string
    Actor      string
    Mode       string
    // Optional fields used by HTTP views
    IdempotencyKey string
    Route          string
    TargetServiceID string
    HashKey        string
    Payload        []byte
    Reason     string
    CreatedAt  time.Time
    UpdatedAt  time.Time
}

type Filter struct {
    State      string
    FunctionID string
    GameID     string
    Env        string
    Actor      string
    Mode       string
}

type Page struct {
    Page int
    Size int
    Sort string // created_at|updated_at asc|desc (simplified)
}

type Store interface {
    List(f Filter, p Page) ([]*Approval, int, error)
    Get(id string) (*Approval, error)
    Approve(id string) (*Approval, error)
    Reject(id, reason string) (*Approval, error)
}

// MemStore is an in-memory approval store for tests/dev.
type MemStore struct {
    mu   sync.RWMutex
    data map[string]*Approval
}

func NewMemStore() *MemStore { return &MemStore{data: map[string]*Approval{}} }

func (s *MemStore) List(f Filter, p Page) ([]*Approval, int, error) {
    s.mu.RLock(); defer s.mu.RUnlock()
    out := make([]*Approval, 0, len(s.data))
    for _, a := range s.data {
        if f.State != "" && !strings.EqualFold(a.State, f.State) { continue }
        if f.FunctionID != "" && a.FunctionID != f.FunctionID { continue }
        if f.GameID != "" && a.GameID != f.GameID { continue }
        if f.Env != "" && a.Env != f.Env { continue }
        if f.Actor != "" && a.Actor != f.Actor { continue }
        if f.Mode != "" && a.Mode != f.Mode { continue }
        out = append(out, a)
    }
    // simple sort by updated_at desc by default
    sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
    total := len(out)
    if p.Size <= 0 { p.Size = 50 }
    if p.Page <= 0 { p.Page = 1 }
    start := (p.Page - 1) * p.Size
    if start > total { return []*Approval{}, total, nil }
    end := start + p.Size
    if end > total { end = total }
    return out[start:end], total, nil
}

func (s *MemStore) Get(id string) (*Approval, error) {
    s.mu.RLock(); defer s.mu.RUnlock()
    if a := s.data[id]; a != nil { return a, nil }
    return nil, errors.New("not found")
}

func (s *MemStore) Approve(id string) (*Approval, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    a := s.data[id]
    if a == nil { return nil, errors.New("not found") }
    a.State = "approved"
    a.UpdatedAt = time.Now()
    return a, nil
}

func (s *MemStore) Reject(id, reason string) (*Approval, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    a := s.data[id]
    if a == nil { return nil, errors.New("not found") }
    a.State = "rejected"
    a.Reason = reason
    a.UpdatedAt = time.Now()
    return a, nil
}

// NewPGStore / NewSQLiteStore placeholders: not implemented in this repo snapshot.
func NewPGStore(_ string) (*MemStore, error)     { return nil, errors.New("pg store not available") }
func NewSQLiteStore(_ string) (*MemStore, error) { return nil, errors.New("sqlite store not available") }
