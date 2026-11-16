package store

import "sync"

// memoryBackend is the no-DSN backend: idempotency arbitration and meta live
// in process memory; the Store's live state IS the durable copy. A restart
// wipes everything — the documented behavior of running without -dsn (the
// same durability gameops-demo had).
type memoryBackend struct {
	mu   sync.Mutex
	idem map[IdemKey]memoryIdemEntry
	meta MetaState
}

type memoryIdemEntry struct {
	fingerprint string
	status      string // pending | committed
	receipt     []byte
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{idem: map[IdemKey]memoryIdemEntry{}}
}

func (b *memoryBackend) LoadAll() ([]Player, []Task, []Ban, []Order, []Activity, []OpRecord, MetaState, error) {
	// Nothing durable to restore; an empty store starts from scratch.
	return []Player{}, []Task{}, []Ban{}, []Order{}, []Activity{}, []OpRecord{}, MetaState{}, nil
}

func (b *memoryBackend) BeginOp(key IdemKey, fingerprint string) (BeginResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	existing, ok := b.idem[key]
	if !ok {
		b.idem[key] = memoryIdemEntry{fingerprint: fingerprint, status: "pending"}
		return BeginResult{Decision: BeginProceed}, nil
	}
	switch {
	case existing.fingerprint != fingerprint:
		return BeginResult{Decision: BeginConflict}, nil
	case existing.status == "committed":
		return BeginResult{Decision: BeginReplay, Receipt: existing.receipt}, nil
	default:
		// Stale pending: release and re-arm (same protocol as the SQL backend).
		b.idem[key] = memoryIdemEntry{fingerprint: fingerprint, status: "pending"}
		return BeginResult{Decision: BeginProceed}, nil
	}
}

func (b *memoryBackend) CommitOp(key IdemKey, receipt []byte, _ WriteSet) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.idem[key]
	if !ok {
		return errIdemRowMissing
	}
	entry.status = "committed"
	entry.receipt = receipt
	b.idem[key] = entry
	return nil
}

func (b *memoryBackend) ReleaseOp(key IdemKey) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	entry, ok := b.idem[key]
	if !ok || entry.status != "pending" {
		return nil
	}
	delete(b.idem, key)
	return nil
}

func (b *memoryBackend) UpdateMeta(meta MetaState) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.meta = meta
	return nil
}

// WipeAll clears idempotency state for a reseed (see seed.go).
func (b *memoryBackend) WipeAll() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.idem = map[IdemKey]memoryIdemEntry{}
	b.meta = MetaState{}
	return nil
}

func (b *memoryBackend) Close() error { return nil }

var _ Backend = (*memoryBackend)(nil)
