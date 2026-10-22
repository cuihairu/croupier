//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/journal"
	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

func TestPostgresRecoveryExecutorDryRunApplyAndRetry(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_RECOVERY_DSN")
	if dsn == "" {
		t.Skip("TEST_POSTGRES_RECOVERY_DSN not configured")
	}
	ctx := context.Background()
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(ctx, `DROP SCHEMA IF EXISTS croupier_recovery CASCADE; DROP TABLE IF EXISTS public.recovery_rows`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `CREATE TABLE public.recovery_rows (id bigint PRIMARY KEY, name text NOT NULL, payload jsonb NOT NULL)`)
	require.NoError(t, err)

	applier, err := NewApplier(ctx, db)
	require.NoError(t, err)
	auditor, err := NewAuditor(ctx, db)
	require.NoError(t, err)

	log, err := journal.Open(filepath.Join(t.TempDir(), "events.jsonl"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = log.Close() })
	event := targetEvent("event-1", cdc.OpInsert, 1, "one")
	_, _, err = log.Append(event)
	require.NoError(t, err)

	plan := recovery.Plan{
		RequestID: "request-1", Principal: "operator-1", SourceID: "source-1", TargetID: "target-1",
		FromSequence: 1, ToSequence: 1, Tables: []string{"public.recovery_rows"}, Mode: recovery.ModeApply,
		FromPosition: cdc.Position{Kind: "lsn", Value: "1"}, ToPosition: cdc.Position{Kind: "lsn", Value: "1"},
		Snapshot: recovery.Snapshot{ID: "snapshot-1", SourceID: "source-1", Position: cdc.Position{Kind: "lsn", Value: "0"}, Tables: []string{"public.recovery_rows"}, Consistent: true},
	}
	executor := &recovery.Executor{
		Journal: log, TargetConfig: recovery.TargetConfig{ID: "target-1", Adapter: "postgres", DSNEnv: "RESTORE_DSN"},
		Authorizer: allowAll{}, Snapshots: allowSnapshot{}, Target: applier, Audit: auditor,
		Source: cdc.Capabilities{Snapshot: true, TransactionMetadata: true, AfterImage: true},
	}

	plan.Mode = recovery.ModeDryRun
	dryRun, err := executor.Execute(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, uint64(1), dryRun.LastSequence)
	require.Equal(t, 1, dryRun.Transactions)
	var rowCount int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM public.recovery_rows`).Scan(&rowCount))
	require.Zero(t, rowCount)

	plan.Mode = recovery.ModeApply
	applied, err := executor.Execute(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, uint64(1), applied.LastSequence)
	require.Equal(t, 1, applied.Transactions)
	retried, err := executor.Execute(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, 1, retried.Transactions)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM public.recovery_rows`).Scan(&rowCount))
	require.Equal(t, 1, rowCount)

	var stages []string
	rows, err := db.QueryContext(ctx, `SELECT stage FROM croupier_recovery.audit_records ORDER BY id`)
	require.NoError(t, err)
	for rows.Next() {
		var stage string
		require.NoError(t, rows.Scan(&stage))
		stages = append(stages, stage)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{
		"requested", "started", "transaction_validated", "completed",
		"requested", "started", "transaction_validated", "transaction_applied", "completed",
		"requested", "started", "transaction_validated", "transaction_applied", "completed",
	}, stages)
}

func targetEvent(id string, operation cdc.Operation, rowID int64, name string) cdc.ChangeEvent {
	return cdc.ChangeEvent{
		ID: id, SourceID: "source-1", SourceType: "postgres", Database: "public", Table: "recovery_rows",
		Operation: operation, Key: []string{"id"}, ColumnsMapped: true,
		Position: cdc.Position{Kind: "lsn", Value: "1"}, Tx: cdc.TxInfo{ID: "tx-1", Seq: 1, End: true}, TsUnixMs: time.Now().UnixMilli(),
		After: map[string]json.RawMessage{"id": []byte(string(mustJSON(rowID))), "name": mustJSON(name), "payload": []byte(`{"kind":"demo"}`)},
	}
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

type allowAll struct{}

func (allowAll) Authorize(context.Context, string, recovery.Plan) error { return nil }

type allowSnapshot struct{}

func (allowSnapshot) VerifySnapshot(context.Context, recovery.Snapshot, recovery.Plan) error {
	return nil
}
