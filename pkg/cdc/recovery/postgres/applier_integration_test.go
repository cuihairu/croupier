//go:build integration

package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/pkg/cdc"

	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

func TestPostgresApplierRealTarget(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_RECOVERY_DSN")
	if dsn == "" {
		dsn = os.Getenv("TEST_POSTGRES_DSN")
	}
	if dsn == "" {
		t.Skip("TEST_POSTGRES_RECOVERY_DSN or TEST_POSTGRES_DSN not configured")
	}
	const dsnEnv = "TEST_POSTGRES_RECOVERY_DSN_ACTIVE"
	t.Setenv(dsnEnv, dsn)

	ctx := context.Background()
	setup, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = setup.Close() })
	_, err = setup.ExecContext(ctx, `DROP SCHEMA IF EXISTS croupier_recovery CASCADE; DROP TABLE IF EXISTS public.recovery_rows`)
	require.NoError(t, err)
	_, err = setup.ExecContext(ctx, `CREATE TABLE public.recovery_rows (id bigint PRIMARY KEY, name text NOT NULL, payload jsonb NOT NULL)`)
	require.NoError(t, err)

	applier, err := Open(ctx, dsnEnv)
	require.NoError(t, err)
	t.Cleanup(func() { _ = applier.Close() })

	insertOne := event("event-insert-1", cdc.OpInsert, 1, "one")
	insertTwo := event("event-insert-2", cdc.OpInsert, 2, "two")
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-1", []cdc.ChangeEvent{insertOne, insertTwo}))
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-1", []cdc.ChangeEvent{insertOne, insertTwo}))
	assertRows(t, applier.db, []row{{1, "one"}, {2, "two"}})

	update := event("event-update-1", cdc.OpUpdate, 1, "updated")
	update.Before = map[string]json.RawMessage{"id": []byte(`1`), "name": []byte(`"one"`), "payload": []byte(`{"kind":"demo"}`)}
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-2", []cdc.ChangeEvent{update}))
	delete := event("event-delete-2", cdc.OpDelete, 2, "two")
	delete.Before = map[string]json.RawMessage{"id": []byte(`2`), "name": []byte(`"two"`), "payload": []byte(`{"kind":"demo"}`)}
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-3", []cdc.ChangeEvent{delete}))
	assertRows(t, applier.db, []row{{1, "updated"}})

	insertThree := event("event-insert-3", cdc.OpInsert, 3, "three")
	conflict := event("event-insert-conflict", cdc.OpInsert, 1, "conflict")
	err = applier.ApplyTransaction(ctx, "request-1", "transaction-rollback", []cdc.ChangeEvent{insertThree, conflict})
	require.ErrorContains(t, err, "duplicate key")
	assertRows(t, applier.db, []row{{1, "updated"}})
	var rollbackLedgerRows int
	require.NoError(t, applier.db.QueryRowContext(ctx, `SELECT count(*) FROM croupier_recovery.applied_transactions WHERE transaction_id=$1`, "transaction-rollback").Scan(&rollbackLedgerRows))
	require.Zero(t, rollbackLedgerRows)

	conflictContent := event("event-update-1", cdc.OpUpdate, 1, "changed")
	err = applier.ApplyTransaction(ctx, "request-1", "transaction-2", []cdc.ChangeEvent{conflictContent})
	require.ErrorContains(t, err, "different content")
	assertRows(t, applier.db, []row{{1, "updated"}})

	require.NoError(t, applier.ValidateTransaction(ctx, recoveryPlan(), []cdc.ChangeEvent{insertOne}))
}

func event(id string, operation cdc.Operation, rowID int64, name string) cdc.ChangeEvent {
	return cdc.ChangeEvent{
		ID: id, SourceID: "source-1", SourceType: "postgres", Database: "public", Table: "recovery_rows",
		Operation: operation, Key: []string{"id"}, ColumnsMapped: true,
		Position: cdc.Position{Kind: "lsn", Value: "1"}, Tx: cdc.TxInfo{ID: "transaction-" + id, Seq: 1, End: true},
		After: map[string]json.RawMessage{"id": []byte(string(jsonInt(rowID))), "name": []byte(`"` + name + `"`), "payload": []byte(`{"kind":"demo"}`)},
	}
}

func jsonInt(value int64) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

type row struct {
	id   int64
	name string
}

func assertRows(t *testing.T, db *sql.DB, expected []row) {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), `SELECT id, name FROM public.recovery_rows ORDER BY id`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	actual := make([]row, 0, len(expected))
	for rows.Next() {
		var item row
		require.NoError(t, rows.Scan(&item.id, &item.name))
		actual = append(actual, item)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, expected, actual)
}

func recoveryPlan() recovery.Plan { return recovery.Plan{} }
