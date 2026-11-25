package resourcecatalog

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func brokenTableDBV10(t *testing.T, table string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE "+table+" (id INTEGER PRIMARY KEY)").Error)
	return db
}

// failingCreateAuditStore wraps the in-memory implementation and makes Create
// always fail, to cover the branch where a failed audit.Log write only logs to slog.
type failingCreateAuditStore struct {
	*audit.InMemoryAuditStore
}

func (f *failingCreateAuditStore) Create(record *audit.AuditRecord) error {
	return errors.New("audit store down")
}

// ---------------------------------------------------------------------------
// UpdateSemantics: updateId / deleteId binding validation branches
// ---------------------------------------------------------------------------

func TestUpdateSemanticsInvalidUpdateIDV10(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	seedV9Capability(t, db, "player")
	seedV9Contract(t, db, "player.update", dbenum.CapabilityUpdate,
		`{"type":"object","properties":{"id":{"type":"string"}}}`, `{"type":"object"}`)
	svc := NewService(db, nil)

	contract, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "g1", "e1", "player.update")
	require.NoError(t, err)

	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", UpdateID: contract.ID + 100,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid updateId")
}

func TestUpdateSemanticsDeleteIDBindsAndFailsV10(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	seedV9Capability(t, db, "player")
	seedV9Contract(t, db, "player.delete", dbenum.CapabilityDelete,
		`{"type":"object","properties":{"id":{"type":"string"}}}`, `{"type":"object"}`)
	svc := NewService(db, nil)

	contract, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "g1", "e1", "player.delete")
	require.NoError(t, err)

	// A valid deleteId: binding succeeds and semantics are written.
	resp, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", DeleteID: contract.ID, ChangeReason: "bind delete",
	})
	require.NoError(t, err)
	assert.Equal(t, "platform_review", resp.Source)

	sem, err := model.NewCapabilitySemanticsModel(db).FindByScopeAndResourceKey(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	assert.Equal(t, contract.ID, sem.DeleteID)

	// An invalid deleteId: errors and rolls back.
	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", DeleteID: contract.ID + 100,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid deleteId")
}

// ---------------------------------------------------------------------------
// validateSemanticFunctionRef: resource ownership / capability mismatch branches
// ---------------------------------------------------------------------------

func TestValidateSemanticFunctionRefMismatchesV10(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	seedV9Capability(t, db, "player")
	seedV9Contract(t, db, "player.report", dbenum.CapabilityReport,
		`{"type":"object","properties":{"date":{"type":"string"}}}`,
		`{"type":"object","properties":{"rows":{"type":"array","items":{"type":"object","properties":{"date":{"type":"string"},"count":{"type":"integer"}}}}}}`)
	svc := NewService(db, nil)

	// The function belongs to another resource.
	_, err := svc.validateSemanticFunctionRef(ctx, "g1", "e1", "guild",
		spec.FunctionRef{FunctionID: "player.report"}, spec.CapabilityReport, "reports[0].query")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to resource guild")

	// The capability does not match the requirement.
	_, err = svc.validateSemanticFunctionRef(ctx, "g1", "e1", "player",
		spec.FunctionRef{FunctionID: "player.report"}, spec.CapabilityTask, "tasks[0].start")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "capability must be task")
}

// ---------------------------------------------------------------------------
// arrayItemSchemaAtPointer: intermediate node without properties
// ---------------------------------------------------------------------------

func TestArrayItemSchemaAtPointerIntermediateNoPropertiesV10(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"wrapper":{"type":"object"}}}`)
	_, ok := arrayItemSchemaAtPointer(schema, "/wrapper/rows")
	assert.False(t, ok, "pointer through a node without properties must fail")
}

// ---------------------------------------------------------------------------
// buildAffectedPages: missing published/proposals tables fall back to nil
// ---------------------------------------------------------------------------

func TestBuildAffectedPagesPublishedMissingTableV10(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	createV9PageDraftsStub(t, db)
	// The whole published_page_specs table is missing -> isMissingTableErr -> (nil, nil).
	items, err := NewService(db, nil).buildAffectedPages(context.Background(), "g1", "e1", "player")
	require.NoError(t, err)
	assert.Nil(t, items)
}

func TestBuildAffectedPagesProposalMissingTableV10(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	createV9PageDraftsStub(t, db)
	require.NoError(t, db.Exec("CREATE TABLE published_page_specs (game_id TEXT, env TEXT, page_key TEXT, version INTEGER)").Error)
	require.NoError(t, db.Migrator().DropTable("page_proposals"))

	items, err := NewService(db, nil).buildAffectedPages(context.Background(), "g1", "e1", "player")
	require.NoError(t, err)
	assert.Nil(t, items)
}

// ---------------------------------------------------------------------------
// ResolveConflict: rebuildProposals failure / audit write failure
// ---------------------------------------------------------------------------

func TestResolveConflictRebuildProposalsErrorV10(t *testing.T) {
	db := setupTestDB(t)
	seedV9ConflictSemantics(t, db, []byte(`[{"field":"identityField","values":{"sdk_explicit":"id"}}]`))
	svc := NewService(db, nil)
	svc.contractService = contractsvc.NewContractService(brokenTableDBV10(t, "capability_semantics"))

	_, err := svc.ResolveConflict(context.Background(), &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "identityField", ChosenSource: "sdk_explicit",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rebuild proposals")
}

func TestUpdateSemanticsAuditLogFailureV10(t *testing.T) {
	db := setupTestDB(t)
	seedV9Capability(t, db, "player")
	auditSvc := audit.NewAuditService(&failingCreateAuditStore{InMemoryAuditStore: audit.NewInMemoryAuditStore()}, nil)
	svc := NewService(db, auditSvc)

	// A failed audit write only logs to slog; the business result is unaffected.
	resp, err := svc.UpdateSemantics(context.Background(), &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", IdentityField: "id",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "semantics updated")
}

func TestResolveConflictAuditLogFailureV10(t *testing.T) {
	db := setupTestDB(t)
	seedV9ConflictSemantics(t, db, []byte(`[{"field":"identityField","values":{"sdk_explicit":"id"}}]`))
	auditSvc := audit.NewAuditService(&failingCreateAuditStore{InMemoryAuditStore: audit.NewInMemoryAuditStore()}, nil)
	svc := NewService(db, auditSvc)

	resp, err := svc.ResolveConflict(context.Background(), &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "identityField", ChosenSource: "sdk_explicit",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "Conflict resolved")
}
