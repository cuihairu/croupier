package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// opKey names one mutation kind; together with (gameID, env, operationId) it
// forms the idempotency key.
type opKey string

const (
	opPlayerCreate        opKey = "player.create"
	opPlayerUpdate        opKey = "player.update"
	opPlayerDelete        opKey = "player.delete"
	opPlayerBan           opKey = "player.ban"
	opPlayerRecharge      opKey = "player.recharge"
	opPlayerBatchRecharge opKey = "player.batchrecharge"
	opPlayerMail          opKey = "player.mail"
	opPlayerBatchBan      opKey = "player.batchban"
	opPlayerBatchUnban    opKey = "player.batchunban"
	opMailTaskCreate      opKey = "mail_task.create"
	opMailTaskUpdate      opKey = "mail_task.update"
	opMailTaskRetryFailed opKey = "mail_task.retry_failed"
	opMailTaskCancel      opKey = "mail_task.cancel"
	opBanCreate           opKey = "ban.create"
	opBanUnban            opKey = "ban.unban"
	opBanExtend           opKey = "ban.extend"
	opBanBatchUnban       opKey = "ban.batch_unban"
	opOrderRecredit       opKey = "order.recredit"
	opActivityCreate      opKey = "activity.create"
	opActivityPublish     opKey = "activity.publish_version"
	opActivityActivate    opKey = "activity.activate"
	opActivityEnd         opKey = "activity.end"
	opActivityRollback    opKey = "activity.rollback"
)

// ErrOperationConflict is returned when an operationId is reused with a
// different request fingerprint. Clients must pick a new operationId instead
// of silently rerouting old work under one key.
var ErrOperationConflict = errors.New("operation_fingerprint_mismatch")

// begunTx is one in-flight write transaction: the backend row key plus the
// pre-tx snapshot used for rollback and delete detection.
type begunTx struct {
	key      IdemKey
	snapshot snapshot
}

// write runs one exactly-once mutation.
//
// Protocol (single writer — the store lock is held from Begin to Commit):
//
//  1. decode the payload; operationId comes from the payload's operationId
//     field (declared extra input, not part of the InputSchema contract —
//     schemas accept additionalProperties). Writes without one get an
//     auto-generated id: the transactional path is uniform, but only
//     client-supplied ids are stable across retries.
//  2. BeginOp arbitrates the idempotency row: replay a committed receipt
//     verbatim (the core restart-replay guarantee), reject fingerprint
//     conflicts, release stale pending rows from crashed attempts.
//  3. fn mutates in-memory state and returns the receipt value.
//  4. commitWrite persists business rows + audit + meta and marks the row
//     committed with the receipt — atomically in the SQL backend.
//  5. on any business or commit failure: restore the snapshot and release
//     the pending row, so the caller can retry cleanly. Convergence for a
//     true unknown result (crash after game-side commit, before the caller
//     learns it) stays a platform contract — GAME-010.
func (s *Store) write(op opKey, payload []byte, fn func(m map[string]any) (any, error)) ([]byte, error) {
	m, err := decodePayload(payload)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	replay, begun, err := s.beginWriteLocked(op, m)
	if err != nil {
		return nil, err
	}
	if replay != nil {
		return replay, nil
	}
	out, err := fn(m)
	if err == nil {
		var receipt []byte
		receipt, err = encodeJSON(out)
		if err == nil {
			err = s.commitWriteLocked(begun, receipt)
			if err == nil {
				return receipt, nil
			}
		}
	}
	s.restoreLocked(begun.snapshot)
	_ = s.backend.ReleaseOp(begun.key)
	return nil, err
}

func (s *Store) beginWriteLocked(op opKey, m map[string]any) (replay []byte, begun *begunTx, err error) {
	operationID := valueString(m, "operationId")
	if operationID == "" {
		operationID = autoOperationID()
	}
	fp, err := payloadFingerprint(m)
	if err != nil {
		return nil, nil, err
	}
	key := IdemKey{GameID: s.gameID, Env: s.env, Op: string(op), OperationID: operationID}
	res, err := s.backend.BeginOp(key, fp)
	if err != nil {
		return nil, nil, err
	}
	switch res.Decision {
	case BeginReplay:
		return res.Receipt, nil, nil
	case BeginConflict:
		return nil, nil, fmt.Errorf("%w: operationId %s was used with a different payload", ErrOperationConflict, operationID)
	default:
		return nil, &begunTx{key: key, snapshot: s.snapshotLocked()}, nil
	}
}

func (s *Store) commitWriteLocked(begun *begunTx, receipt []byte) error {
	cur := s.snapshotLocked()
	set := WriteSet{
		Players:    cur.players,
		Tasks:      cur.tasks,
		Bans:       cur.bans,
		Orders:     cur.orders,
		Activities: cur.activities,
		Audit:      cur.opRecords,
		Meta:       s.metaLocked(),
	}
	return s.backend.CommitOp(begun.key, receipt, set)
}

func (s *Store) metaLocked() MetaState {
	return MetaState{
		SeedN:        s.meta.SeedN,
		Seeded:       s.meta.Seeded,
		ReportRunSeq: s.reportRunSeq,
	}
}

// payloadFingerprint is the sha256 of the canonical JSON re-encoding of the
// decoded payload (map keys are sorted by encoding/json), so byte-level
// key-order or whitespace differences do not manufacture conflicts while any
// semantic difference does.
func payloadFingerprint(m map[string]any) (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("canonicalize payload: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
