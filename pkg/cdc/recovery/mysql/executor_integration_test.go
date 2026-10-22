//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

func executorEvent() cdc.ChangeEvent {
	return cdc.ChangeEvent{
		ID: "event-1", SourceID: "source-1", SourceType: "mysql", Database: "recovery_executor", Table: "recovery_rows",
		Operation: cdc.OpInsert, Key: []string{"id"}, ColumnsMapped: true,
		Position: cdc.Position{Kind: "gtid", Value: "00000000-0000-0000-0000-000000000001:1"},
		Tx:       cdc.TxInfo{ID: "tx-1", Seq: 1, End: true}, TsUnixMs: time.Now().UnixMilli(),
		After: map[string]json.RawMessage{
			"id":   json.RawMessage(`1`),
			"name": json.RawMessage(`"one"`),
		},
	}
}

func recoveryPlan(table string) recovery.Plan {
	return recovery.Plan{
		RequestID: "request-1", Principal: "operator-1", SourceID: "source-1", TargetID: "mysql-target",
		FromSequence: 1, ToSequence: 1,
		FromPosition: cdc.Position{Kind: "gtid", Value: "00000000-0000-0000-0000-000000000001:1"},
		ToPosition:   cdc.Position{Kind: "gtid", Value: "00000000-0000-0000-0000-000000000001:1"},
		Tables:       []string{table}, Mode: recovery.ModeDryRun,
		Snapshot: recovery.Snapshot{
			ID: "snapshot-1", SourceID: "source-1",
			Position: cdc.Position{Kind: "gtid", Value: "00000000-0000-0000-0000-000000000001:0"},
			Tables:   []string{table}, Consistent: true,
		},
	}
}

func assertExecutorRows(ctx context.Context, t *testing.T, db *sql.DB, expected map[int64]string) {
	t.Helper()
	rows, err := db.QueryContext(ctx, "SELECT id, name FROM recovery_executor.recovery_rows")
	if err != nil {
		t.Fatalf("query rows: %v", err)
	}
	defer rows.Close()
	actual := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		actual[id] = name
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(actual) != len(expected) {
		t.Fatalf("row count = %d, want %d (%v)", len(actual), len(expected), actual)
	}
	for id, name := range expected {
		if actual[id] != name {
			t.Fatalf("row %d = %q, want %q", id, actual[id], name)
		}
	}
}

type allowAll struct{}

func (allowAll) Authorize(context.Context, string, recovery.Plan) error { return nil }

type allowSnapshot struct{}

func (allowSnapshot) VerifySnapshot(context.Context, recovery.Snapshot, recovery.Plan) error {
	return nil
}
