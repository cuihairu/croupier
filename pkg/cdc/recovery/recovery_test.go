package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/journal"
	"github.com/cuihairu/croupier/pkg/cdc/schema"
)

func TestPlanRequiresExactScopeAndCoveredConsistentSnapshot(t *testing.T) {
	p := validPlan(ModeDryRun)
	p.Tables = []string{"db.*"}
	if err := p.Validate(); err == nil {
		t.Fatal("wildcard table should be rejected")
	}
	p = validPlan(ModeDryRun)
	p.Snapshot.Tables = nil
	if err := p.Validate(); err == nil {
		t.Fatal("snapshot without table coverage should be rejected")
	}
	p = validPlan(ModeDryRun)
	p.Snapshot.Consistent = false
	if err := p.Validate(); err == nil {
		t.Fatal("inconsistent snapshot should be rejected")
	}
	p = validPlan(ModeDryRun)
	p.RequireSchemaHistory = true
	if err := p.Validate(); err == nil {
		t.Fatal("missing base schema version should be rejected when schema history is required")
	}
}

func TestTargetConfigUsesSecretReferenceAndRejectsPlaintextOptions(t *testing.T) {
	cfg := TargetConfig{ID: "target-1", Adapter: "postgres", DSNEnv: "RESTORE_DATABASE_DSN", Options: map[string]string{"sslmode": "verify-full"}}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Options["password"] = "plaintext"
	if err := cfg.Validate(); err == nil {
		t.Fatal("sensitive option should be rejected")
	}
}

func TestExecuteDryRunAuthorizesAuditsAndNeverWrites(t *testing.T) {
	x, target, audit := setupExecutor(t, nil)
	result, err := x.Execute(context.Background(), validPlan(ModeDryRun))
	if err != nil {
		t.Fatal(err)
	}
	if result.LastSequence != 1 || result.Transactions != 1 || target.validated != 1 || target.applied != 0 {
		t.Fatalf("result=%+v target=%+v", result, target)
	}
	if audit.stages[0] != "requested" || audit.stages[len(audit.stages)-1] != "completed" {
		t.Fatalf("audit stages = %v", audit.stages)
	}
}

func TestExecuteApplyDelegatesAtomicRollbackAndAuditsFailure(t *testing.T) {
	applyErr := errors.New("target write failed")
	x, target, audit := setupExecutor(t, applyErr)
	result, err := x.Execute(context.Background(), validPlan(ModeApply))
	if !errors.Is(err, applyErr) || target.applied != 1 || !target.rollbackOnError || result.LastSequence != 0 {
		t.Fatalf("result=%+v err=%v target=%+v", result, err, target)
	}
	if audit.stages[len(audit.stages)-1] != "failed" {
		t.Fatalf("audit stages = %v", audit.stages)
	}
}

func TestExecuteAuthorizationDenialPreventsDestinationAccess(t *testing.T) {
	denied := errors.New("scope denied")
	x, target, audit := setupExecutor(t, nil)
	x.Authorizer = allowFunc(func(context.Context, string, Plan) error { return denied })
	if _, err := x.Execute(context.Background(), validPlan(ModeApply)); !errors.Is(err, denied) {
		t.Fatalf("Execute error = %v", err)
	}
	if target.capabilityCalls != 0 || target.applied != 0 || audit.stages[len(audit.stages)-1] != "denied" {
		t.Fatalf("target=%+v audit=%v", target, audit.stages)
	}
}

func TestExecuteRejectsUnavailableSourceCapabilities(t *testing.T) {
	x, target, _ := setupExecutor(t, nil)
	x.Source.TransactionMetadata = false
	if _, err := x.Execute(context.Background(), validPlan(ModeApply)); err == nil {
		t.Fatal("expected source capability rejection")
	}
	if target.capabilityCalls != 0 || target.applied != 0 {
		t.Fatalf("target touched before capability rejection: %+v", target)
	}
}

func setupExecutor(t *testing.T, applyErr error) (*Executor, *fakeApplier, *memoryAudit) {
	t.Helper()
	log, err := journal.Open(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	event := cdc.ChangeEvent{
		SourceID: "source-1", SourceType: "postgres", Database: "db", Table: "items",
		Operation: cdc.OpInsert, ColumnsMapped: true,
		After:    map[string]json.RawMessage{"id": json.RawMessage(`1`)},
		Tx:       cdc.TxInfo{ID: "tx-1", Seq: 1, End: true},
		Position: cdc.Position{Kind: "lsn", Value: "10"}, TsUnixMs: 1,
	}
	if _, _, err := log.Append(event); err != nil {
		t.Fatal(err)
	}
	target := &fakeApplier{applyErr: applyErr}
	audit := &memoryAudit{}
	return &Executor{
		Journal:      log,
		TargetConfig: TargetConfig{ID: "restore-db", Adapter: "postgres", DSNEnv: "RESTORE_DATABASE_DSN"},
		Authorizer:   allowFunc(func(context.Context, string, Plan) error { return nil }),
		Snapshots: snapshotFunc(func(_ context.Context, snapshot Snapshot, plan Plan) error {
			if snapshot.ID != plan.Snapshot.ID || !snapshot.Consistent {
				return errors.New("snapshot mismatch")
			}
			return nil
		}),
		Target: target, Audit: audit,
		Source: cdc.Capabilities{Snapshot: true, TransactionMetadata: true, BeforeImage: true},
	}, target, audit
}

func validPlan(mode Mode) Plan {
	return Plan{
		RequestID: "req-1", Principal: "operator-1", SourceID: "source-1", TargetID: "restore-db",
		FromSequence: 1, ToSequence: 1, Tables: []string{"db.items"}, Mode: mode,
		FromPosition: cdc.Position{Kind: "lsn", Value: "10"}, ToPosition: cdc.Position{Kind: "lsn", Value: "10"},
		Snapshot: Snapshot{ID: "snapshot-1", SourceID: "source-1", Position: cdc.Position{Kind: "lsn", Value: "9"}, Tables: []string{"db.items"}, Consistent: true},
	}
}

type allowFunc func(context.Context, string, Plan) error

func (f allowFunc) Authorize(ctx context.Context, principal string, plan Plan) error {
	return f(ctx, principal, plan)
}

type snapshotFunc func(context.Context, Snapshot, Plan) error

func (f snapshotFunc) VerifySnapshot(ctx context.Context, snapshot Snapshot, plan Plan) error {
	return f(ctx, snapshot, plan)
}

type fakeApplier struct {
	capabilityCalls int
	validated       int
	applied         int
	rollbackOnError bool
	applyErr        error
}

func (a *fakeApplier) Capabilities(context.Context) (TargetCapabilities, error) {
	a.capabilityCalls++
	return TargetCapabilities{AtomicTransactions: true, IdempotentTransactions: true, SupportedOperations: []cdc.Operation{cdc.OpInsert, cdc.OpUpdate, cdc.OpDelete}}, nil
}

func (a *fakeApplier) ValidateTransaction(context.Context, Plan, []cdc.ChangeEvent) error {
	a.validated++
	return nil
}

func (a *fakeApplier) ApplyTransaction(context.Context, string, string, []cdc.ChangeEvent) error {
	a.applied++
	if a.applyErr != nil {
		a.rollbackOnError = true // Adapter contract: failed ApplyTransaction rolls back atomically.
	}
	return a.applyErr
}

type memoryAudit struct{ stages []string }

func (a *memoryAudit) Append(_ context.Context, record AuditRecord) error {
	a.stages = append(a.stages, record.Stage)
	return nil
}

// 恢复时按事务时点校验 schema（RequireSchemaHistory）：旧事件列集按其源位点
// 当时的 schema 版本核验——引用时点外列→拒绝（旧事件恢复拒绝路径）。
func TestExecuteSchemaHistoryTimePointValidation(t *testing.T) {
	histSchema := func(columns ...string) schema.TableSchema {
		ts := schema.TableSchema{Database: "db", Table: "items"}
		for _, c := range columns {
			ts.Columns = append(ts.Columns, schema.Column{Name: c, Type: "23"})
		}
		return ts
	}

	t.Run("pass when event columns exist at time point", func(t *testing.T) {
		x, target, _ := setupExecutor(t, nil)
		x.Source.SchemaHistory = true
		x.SchemaHistory = func(_ context.Context, table string, pos cdc.Position) (schema.TableSchema, error) {
			return histSchema("id"), nil // 事件携带 id → 存在
		}
		plan := validPlan(ModeDryRun)
		plan.RequireSchemaHistory = true
		plan.Snapshot.SchemaVersions = map[string]int64{"db.items": 1}
		if _, err := x.Execute(context.Background(), plan); err != nil {
			t.Fatalf("execute: %v", err)
		}
		if target.validated != 1 {
			t.Fatalf("target validated = %d, want 1", target.validated)
		}
	})

	t.Run("reject old event referencing column outside its time-point schema", func(t *testing.T) {
		x, target, _ := setupExecutor(t, nil)
		x.Source.SchemaHistory = true
		// 事件携带 id；时点 schema 无 id（该列 DDL 后已不存在）→ 拒绝。
		x.SchemaHistory = func(_ context.Context, table string, pos cdc.Position) (schema.TableSchema, error) {
			return histSchema("name"), nil
		}
		plan := validPlan(ModeApply)
		plan.RequireSchemaHistory = true
		plan.Snapshot.SchemaVersions = map[string]int64{"db.items": 1}
		_, err := x.Execute(context.Background(), plan)
		if err == nil || !strings.Contains(err.Error(), "violates schema at its time point") {
			t.Fatalf("execute err = %v, want schema time-point violation", err)
		}
		if target.validated != 0 || target.applied != 0 {
			t.Fatalf("target touched before rejection: %+v", target)
		}
	})

	t.Run("fail closed when executor has no schema history source", func(t *testing.T) {
		x, target, _ := setupExecutor(t, nil)
		x.Source.SchemaHistory = true
		x.SchemaHistory = nil // 接线缺失
		plan := validPlan(ModeApply)
		plan.RequireSchemaHistory = true
		plan.Snapshot.SchemaVersions = map[string]int64{"db.items": 1}
		_, err := x.Execute(context.Background(), plan)
		if err == nil || !strings.Contains(err.Error(), "no schema history source") {
			t.Fatalf("execute err = %v, want missing source", err)
		}
		if target.applied != 0 {
			t.Fatalf("target applied before fail-closed: %+v", target)
		}
	})

	t.Run("capability gate rejects when source lacks schema history", func(t *testing.T) {
		x, target, _ := setupExecutor(t, nil)
		plan := validPlan(ModeApply)
		plan.RequireSchemaHistory = true
		plan.Snapshot.SchemaVersions = map[string]int64{"db.items": 1}
		_, err := x.Execute(context.Background(), plan) // x.Source.SchemaHistory 默认 false
		if err == nil || !strings.Contains(err.Error(), "does not provide schema history") {
			t.Fatalf("execute err = %v, want capability rejection", err)
		}
		if target.applied != 0 {
			t.Fatalf("target applied before gate: %+v", target)
		}
	})
}

// 位置下标像（ColumnsMapped=false）不可按列名 schema 校验：fail-closed。
func TestExecuteSchemaHistoryRejectsPositionalImages(t *testing.T) {
	log, err := journal.Open(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	event := cdc.ChangeEvent{
		SourceID: "source-1", SourceType: "postgres", Database: "db", Table: "items",
		Operation: cdc.OpInsert, ColumnsMapped: false,
		After:    map[string]json.RawMessage{"0": json.RawMessage(`1`)},
		Tx:       cdc.TxInfo{ID: "tx-1", Seq: 1, End: true},
		Position: cdc.Position{Kind: "lsn", Value: "10"}, TsUnixMs: 1,
	}
	if _, _, err := log.Append(event); err != nil {
		t.Fatal(err)
	}
	target := &fakeApplier{}
	x := &Executor{
		Journal:      log,
		TargetConfig: TargetConfig{ID: "restore-db", Adapter: "postgres", DSNEnv: "RESTORE_DATABASE_DSN"},
		Authorizer:   allowFunc(func(context.Context, string, Plan) error { return nil }),
		Snapshots:    snapshotFunc(func(_ context.Context, snapshot Snapshot, plan Plan) error { return nil }),
		Target:       target, Audit: &memoryAudit{},
		Source: cdc.Capabilities{Snapshot: true, TransactionMetadata: true, SchemaHistory: true},
		SchemaHistory: func(_ context.Context, table string, pos cdc.Position) (schema.TableSchema, error) {
			return schema.TableSchema{Database: "db", Table: "items", Columns: []schema.Column{{Name: "id", Type: "23"}}}, nil
		},
	}
	plan := validPlan(ModeApply)
	plan.RequireSchemaHistory = true
	plan.Snapshot.SchemaVersions = map[string]int64{"db.items": 1}
	_, err = x.Execute(context.Background(), plan)
	if err == nil || !strings.Contains(err.Error(), "positional") {
		t.Fatalf("execute err = %v, want positional rejection", err)
	}
	if target.applied != 0 {
		t.Fatalf("target applied: %+v", target)
	}
}
