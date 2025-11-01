package approvals

import (
    "errors"
    "sort"
    "strings"
    "sync"
)

// MemStore is an in-memory approvals store (dev/testing fallback).
type MemStore struct {
    mu   sync.RWMutex
    data map[string]*Approval
}

func NewMemStore() *MemStore { return &MemStore{data: map[string]*Approval{}} }

func (m *MemStore) Create(a *Approval) error {
    m.mu.Lock(); defer m.mu.Unlock()
    if _, ok := m.data[a.ID]; ok { return errors.New("duplicate id") }
    cp := *a
    m.data[a.ID] = &cp
    return nil
}

func (m *MemStore) Get(id string) (*Approval, error) {
    m.mu.RLock(); defer m.mu.RUnlock()
    a := m.data[id]
    if a == nil { return nil, errors.New("not found") }
    cp := *a
    return &cp, nil
}

func (m *MemStore) Approve(id string) (*Approval, error) {
    m.mu.Lock(); defer m.mu.Unlock()
    a := m.data[id]
    if a == nil { return nil, errors.New("not found") }
    if a.State != "pending" { return nil, errors.New("not pending") }
    a.State = "approved"
    cp := *a
    return &cp, nil
}

func (m *MemStore) Reject(id string, reason string) (*Approval, error) {
    m.mu.Lock(); defer m.mu.Unlock()
    a := m.data[id]
    if a == nil { return nil, errors.New("not found") }
    if a.State != "pending" { return nil, errors.New("not pending") }
    a.State = "rejected"
    a.Reason = reason
    cp := *a
    return &cp, nil
}

func (m *MemStore) List(f Filter, p Page) ([]*Approval, int, error) {
    m.mu.RLock(); defer m.mu.RUnlock()
    var arr []*Approval
    for _, a := range m.data {
        if f.State != "" && a.State != f.State { continue }
        if f.FunctionID != "" && a.FunctionID != f.FunctionID { continue }
        if f.GameID != "" && a.GameID != f.GameID { continue }
        if f.Env != "" && a.Env != f.Env { continue }
        if f.Actor != "" && a.Actor != f.Actor { continue }
        if f.Mode != "" && a.Mode != f.Mode { continue }
        cp := *a
        arr = append(arr, &cp)
    }
    // sort
    desc := true
    if strings.ToLower(p.Sort) == "created_at_asc" { desc = false }
    sort.Slice(arr, func(i, j int) bool {
        if desc { return arr[i].CreatedAt.After(arr[j].CreatedAt) }
        return arr[i].CreatedAt.Before(arr[j].CreatedAt)
    })
    total := len(arr)
    if p.Size <= 0 { p.Size = 20 }
    if p.Page <= 0 { p.Page = 1 }
    start := (p.Page-1)*p.Size
    if start >= total { return []*Approval{}, total, nil }
    end := start + p.Size
    if end > total { end = total }
    return arr[start:end], total, nil
}

