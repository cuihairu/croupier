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
	// Approver 是批准/拒绝该申请的操作人（"system" 表示超时/取消等系统动作）；
	// 大多数调用无需审批，仅有审批记录的执行才携带审批人与审批时间。
	Approver   string
	ReviewedAt *time.Time
	Mode       string
	// Optional fields used by HTTP views
	IdempotencyKey  string
	Route           string
	TargetServiceID string
	HashKey         string
	Payload         []byte
	Metadata        map[string]string
	Reason          string
	ResultKind      string
	TaskID          string
	Result          []byte
	// ARCH-002 D7：持久执行领取（意图先行）。ClaimStatus ∈ {empty=pending,
	// claimed, dispatched}；ClaimToken 单调递增；DispatchedAt 为结果回写时间。
	ClaimStatus                string
	ClaimToken                 int64
	ClaimedAt                  *time.Time
	DispatchPayloadFingerprint string
	DispatchedAt               *time.Time
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
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
	Approve(id, operator string) (*Approval, error)
	Reject(id, reason, operator string) (*Approval, error)
	Create(approval *Approval) (*Approval, error)
	Update(approval *Approval) (*Approval, error)
	// ClaimDispatch 是 ARCH-002 D7 的意图先行：CAS 胜出者、调用续跑前，在同一
	// 记录上原子写 claim_status=claimed + 单调递增 claim_token + claimed_at +
	// dispatch_payload_fingerprint。只对 RowsAffected==1 的写入者开放续跑入口，
	// 其余（已领取/已回写/未知）返回 ErrAlreadyClaimed 携带当前记录。
	ClaimDispatch(id, fingerprint string) (*Approval, error)
	// CompleteDispatch 在续跑返回后按 claim_token 条件回写结果与
	// claim_status=dispatched；token 已前进则拒绝（fencing）。
	CompleteDispatch(id string, token int64, resultKind, taskID string, result []byte) (*Approval, error)
}

// ErrAlreadyClaimed 标记 D7 领取竞争失败：意图已由他人写入（或结果已回写）。
var ErrAlreadyClaimed = errors.New("approval dispatch already claimed")

// ErrAlreadyReviewed 标记一次「pending → 终态」的 CAS 转换失败：记录已不在
// pending（被其他操作员批准/拒绝，或本请求重复点击）。携带当前记录，
// 供调用方映射为 409 approval_already_reviewed，且绝不再触发续跑/副作用。
// 这是 ARCH-001 D1（原子转换）与 D4（重复请求规则）的存储层契约。
var ErrAlreadyReviewed = errors.New("approval already reviewed")

type alreadyReviewedError struct {
	record *Approval
	err    error
}

func (e *alreadyReviewedError) Error() string { return e.err.Error() }
func (e *alreadyReviewedError) Unwrap() error { return e.err }

// AsAlreadyReviewed 从 err 中提取 ErrAlreadyReviewed 携带的当前记录。
// 返回 (record, true) 表示 err 是「已处理」冲突；否则 (nil, false)。
func AsAlreadyReviewed(err error) (*Approval, bool) {
	if err == nil {
		return nil, false
	}
	var ae *alreadyReviewedError
	if errors.As(err, &ae) {
		return ae.record, true
	}
	return nil, false
}

func wrapAlreadyReviewed(record *Approval) error {
	// Unwrap 指向 ErrAlreadyReviewed 哨兵：errors.Is(err, ErrAlreadyReviewed)
	// 与 AsAlreadyReviewed 都能识别。
	return &alreadyReviewedError{record: record, err: ErrAlreadyReviewed}
}

func wrapAlreadyClaimed(record *Approval) error {
	return &alreadyClaimedError{record: record, err: ErrAlreadyClaimed}
}

type alreadyClaimedError struct {
	record *Approval
	err    error
}

func (e *alreadyClaimedError) Error() string { return e.err.Error() }
func (e *alreadyClaimedError) Unwrap() error { return e.err }

// AsAlreadyClaimed 从 err 中提取 ErrAlreadyClaimed 携带的当前记录。
func AsAlreadyClaimed(err error) (*Approval, bool) {
	if err == nil {
		return nil, false
	}
	var ae *alreadyClaimedError
	if errors.As(err, &ae) {
		return ae.record, true
	}
	return nil, false
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

func (s *MemStore) Approve(id, operator string) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.data[id]
	if a == nil {
		return nil, errors.New("not found")
	}
	if a.State != "pending" {
		// D1：只有 pending → 终态的转换胜出；重复/竞争请求原样返回当前记录。
		return a, wrapAlreadyReviewed(a)
	}
	a.State = "approved"
	a.Approver = strings.TrimSpace(operator)
	now := time.Now()
	a.ReviewedAt = &now
	a.UpdatedAt = now
	return a, nil
}

func (s *MemStore) Reject(id, reason, operator string) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.data[id]
	if a == nil {
		return nil, errors.New("not found")
	}
	if a.State != "pending" {
		return a, wrapAlreadyReviewed(a)
	}
	a.State = "rejected"
	a.Reason = reason
	a.Approver = strings.TrimSpace(operator)
	now := time.Now()
	a.ReviewedAt = &now
	a.UpdatedAt = now
	return a, nil
}

// ClaimDispatch implements D7 intent-first claim on MemStore under the existing
// lock. Only an untouched record (claim_status empty) can be claimed; the token
// is monotonic (previous+1) so a stale claimant is fenced by token.
func (s *MemStore) ClaimDispatch(id, fingerprint string) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.data[id]
	if a == nil {
		return nil, errors.New("not found")
	}
	if a.ClaimStatus != "" {
		// 已领取/已回写：非本请求写入的意图，返回当前记录 + ErrAlreadyClaimed。
		return a, wrapAlreadyClaimed(a)
	}
	now := time.Now()
	a.ClaimStatus = "claimed"
	a.ClaimToken = a.ClaimToken + 1
	a.ClaimedAt = &now
	a.DispatchPayloadFingerprint = strings.TrimSpace(fingerprint)
	a.UpdatedAt = now
	return a, nil
}

// CompleteDispatch implements result write-back with token fencing.
func (s *MemStore) CompleteDispatch(id string, token int64, resultKind, taskID string, result []byte) (*Approval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.data[id]
	if a == nil {
		return nil, errors.New("not found")
	}
	if a.ClaimToken != token {
		// 令牌已前进：本请求的续跑结果属陈旧派发，不覆盖当前所有权（fencing）。
		return a, wrapAlreadyClaimed(a)
	}
	now := time.Now()
	a.ResultKind = resultKind
	a.TaskID = taskID
	a.Result = result
	a.ClaimStatus = "dispatched"
	a.DispatchedAt = &now
	a.UpdatedAt = now
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
	// Create a copy. Caller-supplied non-zero timestamps are preserved so the
	// in-memory store matches SQLStore semantics (gorm keeps non-zero
	// CreatedAt/UpdatedAt); zero values fall back to now.
	newApproval := *approval
	if newApproval.CreatedAt.IsZero() {
		newApproval.CreatedAt = time.Now()
	}
	if newApproval.UpdatedAt.IsZero() {
		newApproval.UpdatedAt = newApproval.CreatedAt
	}
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
