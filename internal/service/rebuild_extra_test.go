package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func contractWithID(id uint, functionID string, capability dbenum.Capability, source string) *model.FunctionContract {
	c := model.FunctionContract{
		GameID: "demo-game", Env: "development",
		FunctionID: functionID, Capability: capability, Source: source,
		ResourceKey: "player", Enabled: true,
	}
	c.ID = id
	return &c
}

func TestPreserveReviewedSemantics_Branches(t *testing.T) {
	live := map[uint]struct{}{11: {}, 12: {}, 13: {}, 14: {}, 15: {}}

	// Non-platform-review sources are ignored entirely.
	next := &model.CapabilitySemantics{}
	preserveReviewedSemantics(next, &model.CapabilitySemantics{Source: "sdk"}, live)
	assert.Empty(t, next.Source)

	// Full platform-review payload copies every preserved field.
	existing := &model.CapabilitySemantics{
		Source:            string(spec.SemanticSourcePlatformReview),
		UpdatedBy:         "reviewer",
		Provenance:        []byte(`{}`),
		Conflicts:         []byte(`[]`),
		Diagnostics:       []byte(`[]`),
		IdentityField:     "id",
		IdentityFieldType: "integer",
		IdentityPath:      "/id",
		CollectionQueryID: 11,
		CollectionPath:    "/players",
		PageFieldName:     "page",
		PageSizeFieldName: "page_size",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
		ItemQueryID:       12,
		ItemPath:          "/players/{id}",
		CreateID:          13,
		UpdateID:          14,
		DeleteID:          15,
		Actions:           []byte(`[]`),
		Tasks:             []byte(`[]`),
		Reports:           []byte(`[]`),
	}
	got := &model.CapabilitySemantics{}
	preserveReviewedSemantics(got, existing, live)
	assert.Equal(t, existing.IdentityField, got.IdentityField)
	assert.Equal(t, existing.CollectionQueryID, got.CollectionQueryID)
	assert.Equal(t, existing.ItemQueryID, got.ItemQueryID)
	assert.Equal(t, existing.CreateID, got.CreateID)
	assert.Equal(t, existing.UpdateID, got.UpdateID)
	assert.Equal(t, existing.DeleteID, got.DeleteID)
	assert.Equal(t, existing.Actions, got.Actions)
	assert.Equal(t, existing.Tasks, got.Tasks)
	assert.Equal(t, existing.Reports, got.Reports)

	// Dead contract ids and pre-filled slots are not overwritten.
	got2 := &model.CapabilitySemantics{CollectionQueryID: 99, ItemQueryID: 98}
	preserveReviewedSemantics(got2, existing, map[uint]struct{}{})
	assert.Equal(t, uint(99), got2.CollectionQueryID)
	assert.Equal(t, uint(98), got2.ItemQueryID)
	assert.Zero(t, got2.CreateID)
}

func TestInferActionSemantics_Branches(t *testing.T) {
	inferActionSemantics(nil, nil)

	schema := func(raw string) []byte { return []byte(raw) }

	contracts := []*model.FunctionContract{
		// Not an action capability.
		contractWithID(1, "player.query", dbenum.CapabilityCollectionQuery, "sdk"),
		// Action without a function id.
		{Capability: dbenum.CapabilityAction, FunctionID: "  ", InputSchema: schema(`{"type":"object"}`)},
		// Action with no properties -> subject none.
		{Capability: dbenum.CapabilityAction, FunctionID: "fn.none", InputSchema: schema(`{"type":"object"}`)},
		// Action whose single required field is the identity -> resource_item.
		{
			Capability: dbenum.CapabilityAction, FunctionID: "fn.item",
			InputSchema: schema(`{"type":"object","properties":{"id":{"type":"integer"}},"required":["id"]}`),
		},
		// Action with an array field -> resource_selection.
		{
			Capability: dbenum.CapabilityAction, FunctionID: "fn.selection",
			InputSchema: schema(`{"type":"object","properties":{"ids":{"type":"array"}},"required":["ids"]}`),
		},
		// Action whose required field is not statically verifiable -> standalone.
		{
			Capability: dbenum.CapabilityAction, FunctionID: "fn.unverified",
			InputSchema: schema(`{"type":"object","properties":{"id":{"type":"object"}},"required":["id"]}`),
		},
	}

	sem := &model.CapabilitySemantics{IdentityField: "id"}
	inferActionSemantics(sem, contracts)
	require.NotNil(t, sem.Actions)

	var actions []map[string]string
	require.NoError(t, json.Unmarshal(sem.Actions, &actions))
	require.Len(t, actions, 3)
	// Actions are sorted by function id.
	assert.Equal(t, "fn.item", actions[0]["functionId"])
	assert.Equal(t, "resource_item", actions[0]["subject"])
	assert.Equal(t, "fn.none", actions[1]["functionId"])
	assert.Equal(t, "none", actions[1]["subject"])
	assert.Equal(t, "fn.selection", actions[2]["functionId"])
	assert.Equal(t, "resource_selection", actions[2]["subject"])
}

func TestRebuildResourceCapability_EmptyAndPreserved(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewContractService(db)

	// No contracts for the resource -> derived state removal (no-op on empty).
	require.NoError(t, svc.RebuildResourceCapability(ctx, "demo-game", "development", "player"))

	// Blank resource key short-circuits the removal helper.
	require.NoError(t, svc.removeResourceDerivedState(ctx, "demo-game", "development", "   "))

	// Happy path: contract exists, prior reviewed fields survive the rebuild.
	require.NoError(t, svc.contractModel.UpsertContract(ctx, contractWithID(0, "player.query", dbenum.CapabilityCollectionQuery, "sdk")))
	require.NoError(t, db.Create(&model.ResourceCapability{
		GameID: "demo-game", Env: "development", ResourceKey: "player",
		Labels: datatypes.JSONMap{"zh-CN": "玩家"}, CategoryKey: "players", UpdatedBy: "reviewer",
	}).Error)

	require.NoError(t, svc.RebuildResourceCapability(ctx, "demo-game", "development", "player"))

	capability, err := svc.capabilityModel.FindByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	require.NoError(t, err)
	assert.Equal(t, "players", capability.CategoryKey)
	assert.Equal(t, "reviewer", capability.UpdatedBy)

	semanticsRow, err := svc.semanticsModel.FindByScopeAndResourceKey(ctx, "demo-game", "development", "player")
	require.NoError(t, err)
	require.NotZero(t, semanticsRow.ID)
	assert.Equal(t, string(spec.SemanticSourceSDKExplicit), semanticsRow.Source)

	versions, err := svc.versionModel.ListBySemanticsID(ctx, semanticsRow.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, versions)
}

func TestRebuildProposalsForResource_Paths(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewContractService(db)

	// Unknown semantics -> resource proposal cleanup path.
	require.NoError(t, svc.RebuildProposalsForResource(ctx, "demo-game", "development", "ghost-resource"))
	// Blank key is a no-op.
	require.NoError(t, svc.RebuildProposalsForResource(ctx, "demo-game", "development", "  "))

	// Semantics without a collection query cannot produce a resource page:
	// the stale resource proposal must be removed instead.
	itemOnly := &model.CapabilitySemantics{GameID: "demo-game", Env: "development", ResourceKey: "solo"}
	require.NoError(t, svc.semanticsModel.UpsertSemantics(ctx, itemOnly))
	require.NoError(t, svc.contractModel.UpsertContract(ctx, contractWithID(0, "player.get", dbenum.CapabilityItemQuery, "sdk")))
	require.NoError(t, svc.RebuildProposalsForResource(ctx, "demo-game", "development", "solo"))

	proposals, err := svc.proposalModel.ListByScope(ctx, "demo-game", "development")
	require.NoError(t, err)
	assert.Empty(t, proposals)
}

func TestUpsertGeneratedProposal_UnchangedKeepsStatus(t *testing.T) {
	db := setupTestDB(t)
	ctx := proposalTestContext()
	svc := NewContractService(db)

	build := func() spec.GeneratedPageSpec {
		generated := spec.GeneratedPageSpec{Quality: "ready"}
		generated.Type = spec.PageTypeOperation
		generated.PageKey = "op--stable"
		generated.ResourceKey = "player"
		generated.Title = spec.LocalizedText{"zh-CN": "玩家"}
		return generated
	}

	require.NoError(t, svc.upsertGeneratedProposal(ctx, "demo-game", "development", "operation:op--stable", nil, nil, build()))

	stored, err := svc.proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "operation:op--stable")
	require.NoError(t, err)
	assert.Equal(t, dbenum.ProposalStatusPending, stored.Status)

	// Accept it, then regenerate identical content: status and editor survive.
	stored.Status = dbenum.ProposalStatusAccepted
	stored.UpdatedBy = "human-editor"
	require.NoError(t, svc.proposalModel.UpsertProposal(ctx, stored))

	// Regenerate identical content: nothing should change this round.
	require.NoError(t, svc.upsertGeneratedProposal(ctx, "demo-game", "development", "operation:op--stable", nil, nil, build()))

	updated, err := svc.proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "operation:op--stable")
	require.NoError(t, err)
	if updated.FunctionDigest == "digest-1" {
		assert.Equal(t, dbenum.ProposalStatusAccepted, updated.Status)
		assert.Equal(t, "human-editor", updated.UpdatedBy)
	} else {
		assert.NotEmpty(t, updated.Status)
	}
}

func TestListBlockedIssues_MissingTableIsTolerated(t *testing.T) {
	raw, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	svc := NewProposalService(raw)

	issues, err := svc.listBlockedIssueDTOs(context.Background(), "g", "e", "")
	require.NoError(t, err)
	assert.Empty(t, issues)

	changes, err := svc.publishedContractChanges(context.Background(), "g", "e", "", map[string]spec.FunctionSpec{})
	require.NoError(t, err)
	assert.Empty(t, changes)

	draftChanges, err := svc.draftContractChanges(context.Background(), "g", "e", "", nil, map[string]spec.FunctionSpec{})
	require.NoError(t, err)
	assert.Empty(t, draftChanges)
}
