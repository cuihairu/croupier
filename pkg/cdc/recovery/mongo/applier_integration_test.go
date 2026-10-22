//go:build integration

package mongo

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/journal"
	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

func TestMongoApplierRealTarget(t *testing.T) {
	dsn := clientRecoveryDSN(t, os.Getenv("TEST_MONGO_RECOVERY_DSN"))
	if dsn == "" {
		t.Skip("TEST_MONGO_RECOVERY_DSN not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	t.Setenv("TEST_MONGO_RECOVERY_DSN_ACTIVE", dsn)

	client, err := mongo.Connect(options.Client().ApplyURI(dsn))
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.Disconnect(cleanupCtx)
	})
	require.NoError(t, client.Ping(ctx, nil))

	err = client.Database("recovery_app").Drop(ctx)
	require.NoError(t, err)
	err = client.Database(ledgerDatabase).Drop(ctx)
	require.NoError(t, err)
	require.NoError(t, client.Database("recovery_app").CreateCollection(ctx, "rows"))
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.Database("recovery_app").Drop(cleanup)
		_ = client.Database(ledgerDatabase).Drop(cleanup)
		_ = client.Disconnect(cleanup)
	})

	applier, err := Open(ctx, "TEST_MONGO_RECOVERY_DSN_ACTIVE")
	require.NoError(t, err)

	insertOne := event("event-insert-1", 1, "one")
	insertTwo := event("event-insert-2", 2, "two")
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-1", []cdc.ChangeEvent{insertOne, insertTwo}))
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-1", []cdc.ChangeEvent{insertOne, insertTwo}))
	assertRows(ctx, t, applier.client, map[int64]string{1: "one", 2: "two"})

	update := event("event-update-1", 1, "updated")
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-2", []cdc.ChangeEvent{update}))
	delete := event("event-delete-2", 2, "two")
	require.NoError(t, applier.ApplyTransaction(ctx, "request-1", "transaction-3", []cdc.ChangeEvent{delete}))
	assertRows(ctx, t, applier.client, map[int64]string{1: "updated"})

	insertThree := event("event-insert-3", 3, "three")
	conflict := event("event-insert-conflict", 1, "conflict")
	err = applier.ApplyTransaction(ctx, "request-1", "transaction-rollback", []cdc.ChangeEvent{insertThree, conflict})
	require.Error(t, err)
	assertRows(ctx, t, applier.client, map[int64]string{1: "updated"})
	count, err := applier.ledger.CountDocuments(ctx, bson.M{"transaction_id": "transaction-rollback"})
	require.NoError(t, err)
	require.Zero(t, count)

	changed := event("event-update-1", 1, "changed")
	err = applier.ApplyTransaction(ctx, "request-1", "transaction-2", []cdc.ChangeEvent{changed})
	require.ErrorContains(t, err, "different content")
	assertRows(ctx, t, applier.client, map[int64]string{1: "updated"})
}

func TestMongoRecoveryExecutorDryRunApplyAndRetry(t *testing.T) {
	dsn := clientRecoveryDSN(t, os.Getenv("TEST_MONGO_RECOVERY_DSN"))
	if dsn == "" {
		t.Skip("TEST_MONGO_RECOVERY_DSN not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	t.Setenv("TEST_MONGO_RECOVERY_DSN_ACTIVE", dsn)

	client, err := mongo.Connect(options.Client().ApplyURI(dsn))
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.Database("recovery_executor").Drop(cleanupCtx)
		_ = client.Database(ledgerDatabase).Drop(cleanupCtx)
		_ = client.Disconnect(cleanupCtx)
	})
	require.NoError(t, client.Ping(ctx, nil))
	require.NoError(t, client.Database("recovery_executor").Drop(ctx))
	require.NoError(t, client.Database("recovery_executor").CreateCollection(ctx, "rows"))

	applier, err := Open(ctx, "TEST_MONGO_RECOVERY_DSN_ACTIVE")
	require.NoError(t, err)
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(func() { _ = applier.Close(cleanupCtx); cleanupCancel() })
	auditor, err := NewAuditor(ctx, client)
	require.NoError(t, err)

	event := executorEvent()
	dir := t.TempDir()
	journal, err := journal.Open(filepath.Join(dir, "cdc.jsonl"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = journal.Close() })
	_, _, err = journal.Append(event)
	require.NoError(t, err)

	executor := &recovery.Executor{
		Journal: journal,
		TargetConfig: recovery.TargetConfig{
			ID: "mongo-target", Adapter: "mongo", DSNEnv: "TEST_MONGO_RECOVERY_DSN_ACTIVE",
		},
		Authorizer: allowAll{},
		Snapshots:  allowSnapshot{},
		Target:     applier,
		Audit:      auditor,
		Source: cdc.Capabilities{
			Snapshot: true, TransactionMetadata: true, AfterImage: true,
		},
	}
	plan := recoveryPlan(event.QualifiedTable())
	dryRun, err := executor.Execute(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, uint64(0), dryRun.LastSequence)
	require.Equal(t, 1, dryRun.Transactions)
	assertExecutorRows(ctx, t, client, map[string]string{})

	plan.Mode = recovery.ModeApply
	applied, err := executor.Execute(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, uint64(1), applied.LastSequence)
	require.Equal(t, 1, applied.Transactions)
	retried, err := executor.Execute(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, 1, retried.Transactions)
	assertExecutorRows(ctx, t, client, map[string]string{"one": "one"})

	var stages []string
	cursor, err := client.Database(ledgerDatabase).Collection("audit_records").Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	require.NoError(t, err)
	for cursor.Next(ctx) {
		var record struct {
			Stage string `bson:"stage"`
		}
		require.NoError(t, cursor.Decode(&record))
		stages = append(stages, record.Stage)
	}
	require.NoError(t, cursor.Err())
	require.NoError(t, cursor.Close(ctx))
	require.Equal(t, []string{
		"requested", "started", "transaction_validated", "transaction_applied", "completed",
		"requested", "started", "transaction_validated", "completed",
		"requested", "started", "transaction_validated", "completed",
		"requested", "started", "transaction_validated", "completed",
	}, stages)
}

func executorEvent() cdc.ChangeEvent {
	return cdc.ChangeEvent{
		SourceID: "source-1", SourceType: "mongodb", Database: "recovery_executor", Table: "rows",
		Operation: cdc.OpInsert, Key: []string{"_id"}, ColumnsMapped: true,
		Position: cdc.Position{Kind: "token", Value: "00"},
		Tx:       cdc.TxInfo{ID: "tx-1", Seq: 1, End: true}, TsUnixMs: time.Now().UnixMilli(),
		After: map[string]json.RawMessage{
			"_id":  json.RawMessage(`"one"`),
			"name": json.RawMessage(`"one"`),
		},
	}
}

func recoveryPlan(table string) recovery.Plan {
	return recovery.Plan{
		RequestID: "request-1", Principal: "tester", SourceID: "source-1", TargetID: "mongo-target",
		FromSequence: 1, ToSequence: 1,
		FromPosition: cdc.Position{Kind: "token", Value: "01"},
		ToPosition:   cdc.Position{Kind: "token", Value: "01"},
		Tables:       []string{table}, Mode: recovery.ModeDryRun,
		Snapshot: recovery.Snapshot{
			ID: "snapshot-1", SourceID: "source-1",
			Position: cdc.Position{Kind: "token", Value: "00"}, Tables: []string{table}, Consistent: true,
		},
	}
}

func assertExecutorRows(ctx context.Context, t *testing.T, client *mongo.Client, expected map[string]string) {
	t.Helper()
	rows, err := client.Database("recovery_executor").Collection("rows").Find(ctx, bson.D{})
	require.NoError(t, err)
	actual := map[string]string{}
	for rows.Next(ctx) {
		var row struct {
			ID   string `bson:"_id"`
			Name string `bson:"name"`
		}
		require.NoError(t, rows.Decode(&row))
		actual[row.ID] = row.Name
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close(ctx))
	require.Equal(t, expected, actual)
}

type allowAll struct{}

func (allowAll) Authorize(context.Context, string, recovery.Plan) error { return nil }

type allowSnapshot struct{}

func (allowSnapshot) VerifySnapshot(context.Context, recovery.Snapshot, recovery.Plan) error {
	return nil
}

func event(id string, rowID int64, name string) cdc.ChangeEvent {
	return cdc.ChangeEvent{
		ID: id, SourceID: "source-1", SourceType: "mongodb", Database: "recovery_app", Table: "rows",
		Operation: cdc.OpInsert, Key: []string{"_id"}, ColumnsMapped: true,
		Position: cdc.Position{Kind: "token", Value: "00"}, Tx: cdc.TxInfo{ID: "transaction-" + id, Seq: 1, End: true},
		TsUnixMs: time.Now().UnixMilli(),
		After: map[string]json.RawMessage{
			"_id":  json.RawMessage(string(mustJSON(rowID))),
			"name": mustJSON(name),
		},
	}
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func assertRows(ctx context.Context, t *testing.T, client *mongo.Client, expected map[int64]string) {
	t.Helper()
	rows, err := client.Database("recovery_app").Collection("rows").Find(ctx, bson.D{})
	require.NoError(t, err)
	defer rows.Close(ctx)
	actual := map[int64]string{}
	for rows.Next(ctx) {
		var row struct {
			ID   int64  `bson:"_id"`
			Name string `bson:"name"`
		}
		require.NoError(t, rows.Decode(&row))
		actual[row.ID] = row.Name
	}
	require.NoError(t, rows.Err())
	require.Equal(t, expected, actual)
}

func clientRecoveryDSN(t *testing.T, dsn string) string {
	t.Helper()
	if dsn == "" {
		return ""
	}
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("directConnection", "true")
	if query.Has("replicaSet") {
		parsed.RawQuery = query.Encode()
	}
	return parsed.String()
}
