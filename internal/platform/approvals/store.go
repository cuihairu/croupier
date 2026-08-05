package approvals

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	gpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	IdempotencyKey  string
	Route           string
	TargetServiceID string
	HashKey         string
	Payload         []byte
	Reason          string
	ResultKind      string
	TaskID          string
	Result          []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
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
	Create(approval *Approval) (*Approval, error)
	Update(approval *Approval) (*Approval, error)
}

// MemStore is an in-memory approval store for tests/dev.
type MemStore struct {
	mu   sync.RWMutex
	data map[string]*Approval
}

func NewMemStore() *MemStore { return &MemStore{data: map[string]*Approval{}} }

func (s *MemStore) List(f Filter, p Page) ([]*Approval, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Approval, 0, len(s.data))
	for _, a := range s.data {
		if f.State != "" && !strings.EqualFold(a.State, f.State) {
			continue
		}
		if f.FunctionID != "" && a.FunctionID != f.FunctionID {
			continue
		}
		if f.GameID != "" && a.GameID != f.GameID {
			continue
		}
		if f.Env != "" && a.Env != f.Env {
			continue
		}
		if f.Actor != "" && a.Actor != f.Actor {
			continue
		}
		if f.Mode != "" && a.Mode != f.Mode {
			continue
		}
		out = append(out, a)
	}
	// simple sort by updated_at desc by default
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	total := len(out)
	if p.Size <= 0 {
		p.Size = 50
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	start := (p.Page - 1) * p.Size
	if start > total {
		return []*Approval{}, total, nil
	}
	end := start + p.Size
	if end > total {
		end = total
	}
	return out[start:end], total, nil
}

func (s *MemStore) Get(id string) (*Approval, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if a := s.data[id]; a != nil {
		return a, nil
	}
	return nil, errors.New("not found")
}

func (s *MemStore) Approve(id string) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.data[id]
	if a == nil {
		return nil, errors.New("not found")
	}
	a.State = "approved"
	a.UpdatedAt = time.Now()
	return a, nil
}

func (s *MemStore) Reject(id, reason string) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.data[id]
	if a == nil {
		return nil, errors.New("not found")
	}
	a.State = "rejected"
	a.Reason = reason
	a.UpdatedAt = time.Now()
	return a, nil
}

func (s *MemStore) Create(approval *Approval) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if approval == nil {
		return nil, errors.New("approval is required")
	}
	if approval.ID == "" {
		return nil, errors.New("approval ID is required")
	}
	// Check if already exists
	if _, exists := s.data[approval.ID]; exists {
		return nil, errors.New("approval already exists")
	}
	// Create a copy
	newApproval := *approval
	newApproval.CreatedAt = time.Now()
	newApproval.UpdatedAt = time.Now()
	s.data[approval.ID] = &newApproval
	return &newApproval, nil
}

func (s *MemStore) Update(approval *Approval) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if approval == nil {
		return nil, errors.New("approval is required")
	}
	if approval.ID == "" {
		return nil, errors.New("approval ID is required")
	}
	if _, exists := s.data[approval.ID]; !exists {
		return nil, errors.New("not found")
	}
	// Create a copy with updated timestamp
	newApproval := *approval
	newApproval.UpdatedAt = time.Now()
	s.data[approval.ID] = &newApproval
	return &newApproval, nil
}

// NewPGStore creates a PostgreSQL-backed approval store
func NewPGStore(dsn string) (Store, error) {
	if dsn == "" {
		return nil, errors.New("PostgreSQL DSN is required")
	}

	db, err := openDB(dsn)
	if err != nil {
		return nil, err
	}

	return NewSQLStore(db)
}

// NewSQLiteStore creates a SQLite-backed approval store
func NewSQLiteStore(dsn string) (Store, error) {
	if dsn == "" {
		dsn = "data/croupier.db"
	}

	db, err := openDB(dsn)
	if err != nil {
		return nil, err
	}

	return NewSQLStore(db)
}

// openDB opens a database connection using GORM
func openDB(dsn string) (*gorm.DB, error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		return gorm.Open(gpostgres.Open(dsn), &gorm.Config{})
	}

	// Default to SQLite - just use the DSN as is for gorm.io/driver/sqlite
	return gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
}
