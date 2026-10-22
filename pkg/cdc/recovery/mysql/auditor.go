package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

const auditTable = "`croupier_recovery`.`audit_records`"

// Auditor durably records recovery metadata without row images or credentials.
type Auditor struct {
	db *sql.DB
}

// NewAuditor accepts an existing MySQL pool and prepares its schema.
func NewAuditor(ctx context.Context, db *sql.DB) (*Auditor, error) {
	if db == nil {
		return nil, errors.New("mysql recovery auditor: db is required")
	}
	if _, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS croupier_recovery CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		return nil, fmt.Errorf("mysql recovery auditor: ensure schema database: %w", err)
	}
	const statement = "CREATE TABLE IF NOT EXISTS " + auditTable + ` (
id bigint NOT NULL AUTO_INCREMENT PRIMARY KEY,
request_id varchar(191) NOT NULL,
principal varchar(191) NOT NULL,
source_id varchar(191) NOT NULL,
target_id varchar(191) NOT NULL,
snapshot_id varchar(191) NOT NULL,
stage varchar(64) NOT NULL,
mode varchar(16) NOT NULL,
sequence bigint NOT NULL DEFAULT 0,
event_ids json NOT NULL,
tables json NOT NULL,
error_class varchar(255) NOT NULL DEFAULT '',
recorded_at timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB`
	if _, err := db.ExecContext(ctx, statement); err != nil {
		return nil, fmt.Errorf("mysql recovery auditor: ensure schema: %w", err)
	}
	return &Auditor{db: db}, nil
}

// Append durably records one audit event.
func (a *Auditor) Append(ctx context.Context, record recovery.AuditRecord) error {
	if a == nil || a.db == nil {
		return errors.New("mysql recovery auditor: not open")
	}
	if record.RequestID == "" || record.Principal == "" || record.SourceID == "" ||
		record.TargetID == "" || record.Stage == "" || record.Mode == "" {
		return errors.New("mysql recovery auditor: record identity fields are required")
	}
	eventIDs, err := json.Marshal(record.EventIDs)
	if err != nil {
		return fmt.Errorf("mysql recovery auditor: encode event ids: %w", err)
	}
	tables, err := json.Marshal(record.TableNames)
	if err != nil {
		return fmt.Errorf("mysql recovery auditor: encode tables: %w", err)
	}
	if _, err := a.db.ExecContext(ctx, "INSERT INTO "+auditTable+` (
request_id, principal, source_id, target_id, snapshot_id, stage, mode, sequence, event_ids, tables, error_class, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS json), CAST(? AS json), ?, ?)`,
		record.RequestID, record.Principal, record.SourceID, record.TargetID, record.SnapshotID,
		record.Stage, string(record.Mode), int64(record.Sequence), string(eventIDs), string(tables),
		record.ErrorClass, record.At,
	); err != nil {
		return fmt.Errorf("mysql recovery auditor: append: %w", err)
	}
	return nil
}
