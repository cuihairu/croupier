package mysql

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

type anyArg struct{}

func (anyArg) Match(driver.Value) bool { return true }

func TestOpenRequiresEnvironmentDSN(t *testing.T) {
	_, err := Open(context.Background(), "")
	require.ErrorContains(t, err, "dsnEnv is required")
	_, err = Open(context.Background(), "CROUPIER_MYSQL_RECOVERY_DSN_UNSET")
	require.ErrorContains(t, err, "environment variable")
}

func TestValidateTransactionRejectsUnsafeEvents(t *testing.T) {
	db, _, _ := sqlmock.New()
	applier := &Applier{db: db}
	base := cdc.ChangeEvent{
		ID: "event-1", SourceID: "source", SourceType: "mysql", Database: "app", Table: "rows",
		Operation: cdc.OpInsert, ColumnsMapped: true, Position: cdc.Position{Kind: "gtid", Value: "1"},
		Tx:    cdc.TxInfo{End: true},
		After: map[string]json.RawMessage{"id": []byte(`1`), "name": []byte(`"hello"`)},
	}

	positional := base
	positional.ColumnsMapped = false
	require.ErrorContains(t, applier.ValidateTransaction(context.Background(), recovery.Plan{}, []cdc.ChangeEvent{positional}), "columns are not mapped")

	noAfter := base
	noAfter.After = nil
	require.ErrorContains(t, applier.ValidateTransaction(context.Background(), recovery.Plan{}, []cdc.ChangeEvent{noAfter}), "no after image")
}
