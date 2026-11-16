package store

import (
	"strconv"
	"strings"
	"sync"
)

// Store holds the demo game state in memory and writes every mutation
// through to the Backend inside an exactly-once transaction (see
// idempotency.go). Reads scan the in-memory state under RLock — the demo
// dataset is tiny by construction, so filter/sort/pagination run in Go for
// both backends and the executor logic has a single implementation.
//
// Mutations append/prepend slices and reassign leaf pointer fields; records
// are never mutated in place through shared pointer or slice backings. That
// invariant is what makes the shallow snapshot copies in snapshotLocked
// safe rollback sources.
type Store struct {
	mu      sync.RWMutex
	backend Backend
	gameID  string
	env     string

	servers      []Server
	players      []Player
	tasks        []Task
	bans         []Ban
	orders       []Order
	activities   []Activity
	opRecords    []OpRecord
	opSeq        int
	reportRunSeq int
	meta         MetaState
}

// seedServers is static (no mutation contract); re-set on load and seed.
var seedServers = []Server{
	{ID: "s1", Name: "电信一区", Status: "healthy"},
	{ID: "s2", Name: "电信二区", Status: "busy"},
	{ID: "s3", Name: "联通一区", Status: "maintenance"},
}

// NewStore builds a store over backend and restores committed state.
func NewStore(gameID, env string, backend Backend) (*Store, error) {
	s := &Store{
		backend: backend,
		gameID:  gameID,
		env:     env,
		servers: append([]Server(nil), seedServers...),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	players, tasks, bans, orders, activities, audit, meta, err := s.backend.LoadAll()
	if err != nil {
		return err
	}
	s.players = players
	s.tasks = tasks
	s.bans = bans
	s.orders = orders
	s.activities = activities
	// Reload order mirrors the in-memory prepend order (newest first).
	for i, j := 0, len(audit)-1; i < j; i, j = i+1, j-1 {
		audit[i], audit[j] = audit[j], audit[i]
	}
	s.opRecords = audit
	// opSeq invariant: every pushed record op-demo-N is committed (append-only
	// audit), so len == the highest N; the scan below is cheap insurance for
	// hand-edited databases.
	s.opSeq = len(s.opRecords)
	for _, r := range s.opRecords {
		if n, ok := parseOpDemoSeq(r.ID); ok && n > s.opSeq {
			s.opSeq = n
		}
	}
	s.reportRunSeq = meta.ReportRunSeq
	s.meta = meta
	return nil
}

func parseOpDemoSeq(id string) (int, bool) {
	rest, ok := strings.CutPrefix(id, "op-demo-")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false
	}
	return n, true
}

// Close releases the backend.
func (s *Store) Close() error { return s.backend.Close() }

// GameID returns the scope this store serves (idempotency keys include it).
func (s *Store) GameID() string { return s.gameID }

// Env returns the scope environment.
func (s *Store) Env() string { return s.env }

// Seeded reports whether a seed run has committed (used by healthz detail).
func (s *Store) Seeded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.meta.Seeded
}

// ---------------------------------------------------------------------------
// Lookup helpers (Locked suffix = caller holds s.mu; ported from
// gameops-demo executors.go).
// ---------------------------------------------------------------------------

func (s *Store) playerByIDLocked(id string) *Player {
	for i := range s.players {
		if s.players[i].ID == id {
			return &s.players[i]
		}
	}
	return nil
}

func (s *Store) taskByIDLocked(id string) *Task {
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			return &s.tasks[i]
		}
	}
	return nil
}

func (s *Store) banByIDLocked(id string) *Ban {
	for i := range s.bans {
		if s.bans[i].ID == id {
			return &s.bans[i]
		}
	}
	return nil
}

func (s *Store) orderByNoLocked(no string) *Order {
	for i := range s.orders {
		if s.orders[i].OrderNo == no {
			return &s.orders[i]
		}
	}
	return nil
}

func (s *Store) activityByIDLocked(id string) *Activity {
	for i := range s.activities {
		if s.activities[i].ID == id {
			return &s.activities[i]
		}
	}
	return nil
}

// pushOpLocked prepends an audit record and returns its operation id.
func (s *Store) pushOpLocked(operator, action, target, result string, approvalID *string) string {
	s.opSeq++
	id := "op-demo-" + intStr(s.opSeq)
	s.opRecords = append([]OpRecord{{
		ID: id, At: millisISO(nowMillis()), Operator: operator, Action: action,
		Target: target, Result: result, OperationID: id, ApprovalID: approvalID,
	}}, s.opRecords...)
	return id
}

// ---------------------------------------------------------------------------
// Snapshot / restore (rollback sources for write transactions).
// ---------------------------------------------------------------------------

func (s *Store) snapshotLocked() snapshot {
	return snapshot{
		players:    cloneSlice(s.players),
		tasks:      cloneSlice(s.tasks),
		bans:       cloneSlice(s.bans),
		orders:     cloneSlice(s.orders),
		activities: cloneSlice(s.activities),
		opRecords:  cloneSlice(s.opRecords),
		opSeq:      s.opSeq,
	}
}

func (s *Store) restoreLocked(snap snapshot) {
	s.players = snap.players
	s.tasks = snap.tasks
	s.bans = snap.bans
	s.orders = snap.orders
	s.activities = snap.activities
	s.opRecords = snap.opRecords
	s.opSeq = snap.opSeq
}

// cloneSlice is a shallow copy: records carry value leaves plus pointer/
// slice fields that mutation sites only ever reassign or append (never write
// through) — see the Store type comment for the invariant.
func cloneSlice[T any](in []T) []T {
	if in == nil {
		return nil
	}
	out := make([]T, len(in))
	copy(out, in)
	return out
}
