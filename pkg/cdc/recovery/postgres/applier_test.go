package postgres

import (
	"context"
	"database/sql/driver"

	"database/sql"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

type anyArg struct{}

func (a anyArg) Match(value driver.Value) bool { return true }

type anyBytes []byte

func (anyBytes) Match(driver.Value) bool { return true }

func expectTargetColumns(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM information_schema.columns.*IN \(\$3, \$4\)`).
		WithArgs("public", "rows", "id", "name").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
}

var errNoRows = sql.ErrNoRows

func mustHash(events []cdc.ChangeEvent) []byte {
	hash, err := hashEvents(events)
	if err != nil {
		panic(err)
	}
	return hash
}

func TestOpenRequiresEnvironmentDSN(t *testing.T) {
	_, err := Open(context.Background(), "")
	require.ErrorContains(t, err, "dsnEnv is required")
	_, err = Open(context.Background(), "CROUPIER_PG_RECOVERY_DSN_UNSET")
	require.ErrorContains(t, err, "environment variable")
}

func TestValidateTransactionRejectsUnsafeEvents(t *testing.T) {
	db, _, _ := sqlmock.New()
	applier := &Applier{db: db}
	base := cdc.ChangeEvent{
		ID: "event-1", SourceID: "source", SourceType: "postgres", Database: "public", Table: "rows",
		Operation: cdc.OpInsert, ColumnsMapped: true, Position: cdc.Position{Kind: "lsn", Value: "1"}, Tx: cdc.TxInfo{End: true},
		After: map[string]json.RawMessage{"id": []byte(`1`), "name": []byte(`"hello"`)},
	}

	positional := base
	positional.ColumnsMapped = false
	require.ErrorContains(t, applier.ValidateTransaction(context.Background(), recovery.Plan{}, []cdc.ChangeEvent{positional}), "columns are not mapped")

	noAfter := base
	noAfter.After = nil
	require.ErrorContains(t, applier.ValidateTransaction(context.Background(), recovery.Plan{}, []cdc.ChangeEvent{noAfter}), "no after image")
}

func TestApplyTransactionRollsBackAndRetriesIdempotently(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	applier := &Applier{db: db}
	event := cdc.ChangeEvent{
		ID: "event-1", SourceID: "source", SourceType: "postgres", Database: "public", Table: "rows",
		Operation: cdc.OpInsert, ColumnsMapped: true, Key: []string{"id"},
		Position: cdc.Position{Kind: "lsn", Value: "1"}, Tx: cdc.TxInfo{ID: "tx-1", End: true},
		After: map[string]json.RawMessage{"id": []byte(`1`), "name": []byte(`"hello"`)},
	}

	expectTargetColumns(mock)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO croupier_recovery`).WithArgs("req-1", "tx-1", 1, anyArg{}).WillReturnRows(sqlmock.NewRows([]string{"request_id"}).AddRow("req-1"))
	mock.ExpectExec(`INSERT INTO "public"\."rows" \("id", "name"\) VALUES \(\$1, \$2\)`).WithArgs(1, "hello").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err = applier.ApplyTransaction(context.Background(), "req-1", "tx-1", []cdc.ChangeEvent{event})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	expectTargetColumns(mock)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO croupier_recovery`).WithArgs("req-1", "tx-1", 1, anyArg{}).WillReturnError(errNoRows)
	mock.ExpectQuery(`SELECT event_count, event_hash`).WithArgs("req-1", "tx-1").WillReturnRows(sqlmock.NewRows([]string{"event_count", "event_hash"}).AddRow(1, mustHash([]cdc.ChangeEvent{event})))
	mock.ExpectCommit()

	err = applier.ApplyTransaction(context.Background(), "req-1", "tx-1", []cdc.ChangeEvent{event})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyTransactionRejectsDifferentContent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	applier := &Applier{db: db}
	event := cdc.ChangeEvent{
		ID: "event-1", SourceID: "source", SourceType: "postgres", Database: "public", Table: "rows",
		Operation: cdc.OpInsert, ColumnsMapped: true, Key: []string{"id"},
		Position: cdc.Position{Kind: "lsn", Value: "1"}, Tx: cdc.TxInfo{ID: "tx-1", End: true},
		After: map[string]json.RawMessage{"id": []byte(`1`), "name": []byte(`"changed"`)},
	}
	expectTargetColumns(mock)
	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO croupier_recovery`).WithArgs("req-1", "tx-1", 1, anyArg{}).WillReturnError(errNoRows)
	mock.ExpectQuery(`SELECT event_count, event_hash`).WithArgs("req-1", "tx-1").WillReturnRows(sqlmock.NewRows([]string{"event_count", "event_hash"}).AddRow(1, []byte("different")))
	mock.ExpectRollback()

	err = applier.ApplyTransaction(context.Background(), "req-1", "tx-1", []cdc.ChangeEvent{event})
	require.ErrorContains(t, err, "different content")
	require.NoError(t, mock.ExpectationsWereMet())
}
