//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/journal"
	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

func TestMySQLApplierRealTarget(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_RECOVERY_DSN")
	if dsn == "" {
		dsn = os.Getenv("TEST_MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("TEST_MYSQL_RECOVERY_DSN or TEST_MYSQL_DSN not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(ctx, "DROP DATABASE IF EXISTS croupier_recovery")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "DROP DATABASE IF EXISTS recovery_app")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE DATABASE recovery_app")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE TABLE recovery_app.recovery_rows (id bigint NOT NULL PRIMARY KEY, name varchar(64) NOT NULL, payload json NOT NULL)")
	require.NoError(t, err)

	applier, err := NewApplier(ctx, db)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelCleanup()
		_, _ = db.ExecContext(cleanupCtx, "DROP DATABASE IF EXISTS recovery_app")
		_, _ = db.ExecContext(cleanupCtx, "DROP DATABASE IF EXISTS croupier_recovery")
	})

	payload := json.RawMessage(`{"kind":"demo"}`)
	insertOne := event("event-insert-1", cdc.OpInsert, 1, "one", payload)
	insertTwo := event("event-insert-2", cdc.OpInsert, 2, "two", payload)
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-1", []cdc.ChangeEvent{insertOne, insertTwo}))
	// Replay of the same request+transaction must be absorbed idempotently.
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-1", []cdc.ChangeEvent{insertOne, insertTwo}))
	assertRows(ctx, t, db, []row{{1, "one"}, {2, "two"}})

	update := event("event-update-1", cdc.OpUpdate, 1, "updated", payload)
	update.Before = beforeImage(1, "one", payload)
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-2", []cdc.ChangeEvent{update}))
	delete := event("event-delete-2", cdc.OpDelete, 2, "two", payload)
	delete.Before = beforeImage(2, "two", payload)
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-3", []cdc.ChangeEvent{delete}))
	assertRows(ctx, t, db, []row{{1, "updated"}})

	// Fault injection: the second statement of this transaction hits a primary
	// key conflict, so the whole transaction (including insert of id=3) must
	// roll back and the ledger must keep no reservation.
	insertThree := event("event-insert-3", cdc.OpInsert, 3, "three", payload)
	conflict := event("event-insert-conflict", cdc.OpInsert, 1, "conflict", payload)
	err = applier.ApplyTransaction(ctx, "request-1", "transaction-rollback", []cdc.ChangeEvent{insertThree, conflict})
	require.Error(t, err)
	require.ErrorContains(t, err, "Duplicate entry")
	assertRows(ctx, t, db, []row{{1, "updated"}})
	var rollbackLedgerRows int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM croupier_recovery.applied_transactions WHERE transaction_id = ?", "transaction-rollback").Scan(&rollbackLedgerRows))
	require.Zero(t, rollbackLedgerRows)

	// Same transaction ID with different content must be rejected, not applied.
	changed := event("event-update-1", cdc.OpUpdate, 1, "changed", payload)
	err = applier.ApplyTransaction(ctx, "request-1", "transaction-2", []cdc.ChangeEvent{changed})
	require.ErrorContains(t, err, "different content")
	assertRows(ctx, t, db, []row{{1, "updated"}})

	require.NoError(t, applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{insertOne}))

	// Schema mismatch must fail validation before any write.
	missing := event("event-missing-column", cdc.OpInsert, 4, "four", payload)
	missing.After["nonexistent"] = json.RawMessage(`"x"`)
	err = applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{missing})
	require.ErrorContains(t, err, "do not all exist")
}

func TestMySQLRecoveryExecutorDryRunApplyAndRetry(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_RECOVERY_DSN")
	if dsn == "" {
		dsn = os.Getenv("TEST_MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("TEST_MYSQL_RECOVERY_DSN or TEST_MYSQL_DSN not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(ctx, "DROP DATABASE IF EXISTS croupier_recovery")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "DROP DATABASE IF EXISTS recovery_executor")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE DATABASE recovery_executor")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE TABLE recovery_executor.recovery_rows (id bigint NOT NULL PRIMARY KEY, name varchar(64) NOT NULL)")
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelCleanup()
		_, _ = db.ExecContext(cleanupCtx, "DROP DATABASE IF EXISTS recovery_executor")
		_, _ = db.ExecContext(cleanupCtx, "DROP DATABASE IF EXISTS croupier_recovery")
	})

	applier, err := NewApplier(ctx, db)
	require.NoError(t, err)
	auditor, err := NewAuditor(ctx, db)
	require.NoError(t, err)

	log, err := journal.Open(filepath.Join(t.TempDir(), "events.jsonl"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })
	event := executorEvent()
	_, _, err = log.Append(event)
	require.NoError(t, err)

	const dsnEnv = "TEST_MYSQL_RECOVERY_DSN_ACTIVE"
	t.Setenv(dsnEnv, dsn)
	executor := &recovery.Executor{
		Journal:      log,
		TargetConfig: recovery.TargetConfig{ID: "mysql-target", Adapter: "mysql", DSNEnv: dsnEnv},
		Authorizer:   allowAll{},
		Snapshots:    allowSnapshot{},
		Target:       applier,
		Audit:        auditor,
		Source:       cdc.Capabilities{Snapshot: true, TransactionMetadata: true, AfterImage: true},
	}
	plan := recoveryPlan("recovery_executor.recovery_rows")

	dryRun, err := executor.Execute(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, uint64(1), dryRun.LastSequence)
	require.Equal(t, 1, dryRun.Transactions)
	assertExecutorRows(ctx, t, db, map[int64]string{})

	plan.Mode = recovery.ModeApply
	applied, err := executor.Execute(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, uint64(1), applied.LastSequence)
	require.Equal(t, 1, applied.Transactions)
	// Replaying the same plan must stay idempotent end to end.
	retried, err := executor.Execute(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, 1, retried.Transactions)
	assertExecutorRows(ctx, t, db, map[int64]string{1: "one"})

	var stages []string
	rows, err := db.QueryContext(ctx, "SELECT stage FROM croupier_recovery.audit_records ORDER BY id")
	require.NoError(t, err)
	for rows.Next() {
		var stage string
		require.NoError(t, rows.Scan(&stage))
		stages = append(stages, stage)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	require.Equal(t, []string{
		"requested", "started", "transaction_validated", "completed",
		"requested", "started", "transaction_validated", "transaction_applied", "completed",
		"requested", "started", "transaction_validated", "transaction_applied", "completed",
	}, stages)
}

func event(id string, operation cdc.Operation, rowID int64, name string, payload json.RawMessage) cdc.ChangeEvent {
	return cdc.ChangeEvent{
		ID: id, SourceID: "source-1", SourceType: "mysql", Database: "recovery_app", Table: "recovery_rows",
		Operation: operation, Key: []string{"id"}, ColumnsMapped: true,
		Position: cdc.Position{Kind: "gtid", Value: "00000000-0000-0000-0000-000000000001:1"},
		Tx:       cdc.TxInfo{ID: "transaction-" + id, Seq: 1, End: true}, TsUnixMs: time.Now().UnixMilli(),
		After: map[string]json.RawMessage{
			"id":      json.RawMessage(jsonInt(rowID)),
			"name":    json.RawMessage(`"` + name + `"`),
			"payload": payload,
		},
	}
}

func beforeImage(rowID int64, name string, payload json.RawMessage) map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"id":      json.RawMessage(jsonInt(rowID)),
		"name":    json.RawMessage(`"` + name + `"`),
		"payload": payload,
	}
}

func jsonInt(value int64) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

type row struct {
	id   int64
	name string
}

func assertRows(ctx context.Context, t *testing.T, db *sql.DB, expected []row) {
	t.Helper()
	rows, err := db.QueryContext(ctx, "SELECT id, name FROM recovery_app.recovery_rows ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()
	actual := make([]row, 0, len(expected))
	for rows.Next() {
		var item row
		require.NoError(t, rows.Scan(&item.id, &item.name))
		actual = append(actual, item)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	require.Equal(t, expected, actual)
}
