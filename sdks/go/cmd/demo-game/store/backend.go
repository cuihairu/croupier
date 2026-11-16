package store

// BeginDecision is the outcome of opening an operation slot in the backend.
type BeginDecision int

const (
	// BeginProceed: no prior row (or a stale pending row was released); the
	// caller executes the mutation.
	BeginProceed BeginDecision = iota
	// BeginReplay: the operation already committed; Receipt holds the original
	// result to replay without re-executing side effects.
	BeginReplay
	// BeginConflict: same operationId with a different request fingerprint —
	// rejected instead of silently executing different work under one key.
	BeginConflict
)

// BeginResult carries the decision plus the backend row key used by
// CommitOp/ReleaseOp.
type BeginResult struct {
	Decision BeginDecision
	Receipt  []byte
}

// IdemKey identifies one idempotency row.
type IdemKey struct {
	GameID      string
	Env         string
	Op          string
	OperationID string
}

// Backend is the durable substrate behind the Store. Two implementations:
// sqlBackend (gorm: postgres or pure-Go sqlite, chosen by DSN prefix) and
// memoryBackend (process-lifetime only; restart wipes state — the documented
// behavior of running without -dsn).
//
// All methods must be safe for use while the Store holds its write lock
// (single-writer discipline; SQL latency under the lock is acceptable at
// demo scale).
type Backend interface {
	// LoadAll restores committed state. Called once at construction before
	// any concurrent access.
	LoadAll() (players []Player, tasks []Task, bans []Ban, orders []Order,
		activities []Activity, audit []OpRecord, meta MetaState, err error)
	// BeginOp inserts a pending idempotency row, or arbitrates an existing
	// one (replay committed receipts, reject fingerprint conflicts, release
	// stale pending rows from crashed attempts).
	BeginOp(key IdemKey, fingerprint string) (BeginResult, error)
	// CommitOp applies the write set and marks the row committed with its
	// receipt, atomically. Audit rows are insert-missing (append-only).
	CommitOp(key IdemKey, receipt []byte, set WriteSet) error
	// ReleaseOp deletes a pending row so the operation can be retried
	// (business failure or crash before commit). Committed rows are never
	// released.
	ReleaseOp(key IdemKey) error
	// UpdateMeta persists the meta row (report run counter). Used by the
	// report.generate task path outside write transactions.
	UpdateMeta(meta MetaState) error
	// WipeAll clears every demo table (business rows, audit, idempotency
	// history, meta) for an explicit reseed.
	WipeAll() error
	Close() error
}
