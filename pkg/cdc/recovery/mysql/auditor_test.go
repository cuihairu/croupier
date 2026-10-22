package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

func TestAuditorRejectsIncompleteRecord(t *testing.T) {
	db, _, _ := sqlmock.New()
	auditor := &Auditor{db: db}
	require.ErrorContains(t, auditor.Append(context.Background(), recovery.AuditRecord{}), "identity fields")
}

func TestAuditorRequiresOpenPool(t *testing.T) {
	require.ErrorContains(t, (*Auditor)(nil).Append(context.Background(), recovery.AuditRecord{}), "not open")
	_, err := NewAuditor(context.Background(), nil)
	require.ErrorContains(t, err, "db is required")
}

func TestAuditorAppendsMetadataOnlyRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	auditor := &Auditor{db: db}
	record := recovery.AuditRecord{
		RequestID: "req-1", Principal: "operator-1", SourceID: "source-1", TargetID: "target-1",
		SnapshotID: "snapshot-1", Stage: "transaction_applied", Mode: recovery.ModeApply, Sequence: 3,
		EventIDs: []string{"event-hash"}, TableNames: []string{"recovery_app.recovery_rows"},
		ErrorClass: "", At: time.Unix(1730000000, 0),
	}
	mock.ExpectExec("INSERT INTO .croupier_recovery...audit_records").
		WithArgs("req-1", "operator-1", "source-1", "target-1", "snapshot-1", "transaction_applied", "apply", int64(3), `["event-hash"]`, `["recovery_app.recovery_rows"]`, "", record.At).
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, auditor.Append(context.Background(), record))
	require.NoError(t, mock.ExpectationsWereMet())
}
