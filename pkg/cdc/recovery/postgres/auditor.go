package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

// Auditor durably records recovery metadata without row images or credentials.
type Auditor struct {
	db *sql.DB
}

// NewAuditor accepts an existing PostgreSQL pool and prepares its schema.
func NewAuditor(ctx context.Context, db *sql.DB) (*Auditor, error) {
	if db == nil {
		return nil, errors.New("postgres recovery auditor: db is required")
	}
	const statement = `CREATE SCHEMA IF NOT EXISTS croupier_recovery; CREATE TABLE IF NOT EXISTS croupier_recovery.audit_records (
id bigserial PRIMARY KEY,
request_id text NOT NULL,
principal text NOT NULL,
source_id text NOT NULL,
target_id text NOT NULL,
snapshot_id text NOT NULL,
stage text NOT NULL,
mode text NOT NULL,
sequence bigint NOT NULL DEFAULT 0,
event_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
tables jsonb NOT NULL DEFAULT '[]'::jsonb,
error_class text NOT NULL DEFAULT '',
recorded_at timestamptz NOT NULL DEFAULT now()
)`
	if _, err := db.ExecContext(ctx, statement); err != nil {
		return nil, fmt.Errorf("postgres recovery auditor: ensure schema: %w", err)
	}
	return &Auditor{db: db}, nil
}

// Append durably records one audit event.
func (a *Auditor) Append(ctx context.Context, record recovery.AuditRecord) error {
	if a == nil || a.db == nil {
		return errors.New("postgres recovery auditor: not open")
	}
	if record.RequestID == "" || record.Principal == "" || record.SourceID == "" ||
		record.TargetID == "" || record.Stage == "" || record.Mode == "" {
		return errors.New("postgres recovery auditor: record identity fields are required")
	}
	eventIDs, err := json.Marshal(record.EventIDs)
	if err != nil {
		return fmt.Errorf("postgres recovery auditor: encode event ids: %w", err)
	}
	tables, err := json.Marshal(record.TableNames)
	if err != nil {
		return fmt.Errorf("postgres recovery auditor: encode tables: %w", err)
	}
	_, err = a.db.ExecContext(ctx, `INSERT INTO croupier_recovery.audit_records (
request_id, principal, source_id, target_id, snapshot_id, stage, mode, sequence, event_ids, tables, error_class, recorded_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		record.RequestID, record.Principal, record.SourceID, record.TargetID, record.SnapshotID,
		record.Stage, string(record.Mode), int64(record.Sequence), eventIDs, tables,
		record.ErrorClass, record.At,
	)
	if err != nil {
		return fmt.Errorf("postgres recovery auditor: append: %w", err)
	}
	return nil
}
