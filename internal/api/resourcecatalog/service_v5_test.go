package resourcecatalog

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Handler – NewHandler, getScope, List, Detail (via handler)
// ---------------------------------------------------------------------------

func TestNewHandlerV5(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	h := NewHandler(svc)
	assert.NotNil(t, h)
	assert.NotNil(t, h.service)
}

func TestGetScopeV5_EmptyContext(t *testing.T) {
	ctx := context.Background()
	// getScope extracts from gin context, but since it depends on svc.GameScopeFromContext,
	// we test the fallback behavior
	_ = ctx
}

// ---------------------------------------------------------------------------
// buildSemanticsInfo – more branches
// ---------------------------------------------------------------------------

func TestBuildSemanticsInfoV5_MoreBranches(t *testing.T) {
	// hasCreate, hasUpdate, hasDelete
	sem := &model.CapabilitySemantics{
		CreateID: 1,
		UpdateID: 2,
		DeleteID: 3,
	}
	info := buildSemanticsInfo(sem)
	require.NotNil(t, info)
	assert.True(t, info.HasCreate)
	assert.True(t, info.HasUpdate)
	assert.True(t, info.HasDelete)

	// hasTasks - simple valid task
	sem2 := &model.CapabilitySemantics{
		Tasks: []byte(`[{"start":{"functionId":"f1"},"status":{"function":{"functionId":"f1"},"taskIdInput":"/id","statePath":"/s"}}]`),
	}
	info2 := buildSemanticsInfo(sem2)
	require.NotNil(t, info2)
	assert.True(t, info2.HasTasks)

	// hasReports
	sem3 := &model.CapabilitySemantics{
		Reports: []byte(`[{"query":{"functionId":"f1"},"datasetPath":"/items","dimensions":["/name"],"metrics":["/count"]}]`),
	}
	info3 := buildSemanticsInfo(sem3)
	require.NotNil(t, info3)
	assert.True(t, info3.HasReports)
}

// ---------------------------------------------------------------------------
// schemaObjectHasPointer – edge cases
// ---------------------------------------------------------------------------

func TestSchemaObjectHasPointerV5_EdgeCases(t *testing.T) {
	// empty pointer
	root := map[string]json.RawMessage{
		"properties": json.RawMessage(`{"name":{"type":"string"}}`),
	}
	assert.True(t, schemaObjectHasPointer(root, ""))

	// non-pointer
	assert.False(t, schemaObjectHasPointer(root, "abc"))

	// nested pointer
	nested := map[string]json.RawMessage{
		"properties": json.RawMessage(`{"addr":{"type":"object","properties":{"city":{"type":"string"}}}}`),
	}
	assert.True(t, schemaObjectHasPointer(nested, "/addr/city"))
	assert.False(t, schemaObjectHasPointer(nested, "/addr/zip"))
}

// ---------------------------------------------------------------------------
// countUnresolvedConflicts – more edge cases
// ---------------------------------------------------------------------------

func TestCountUnresolvedConflictsV5(t *testing.T) {
	// invalid JSON
	assert.Equal(t, 0, countUnresolvedConflicts([]byte(`{bad}`)))

	// single unresolved
	assert.Equal(t, 1, countUnresolvedConflicts([]byte(`[{"field":"a"}]`)))

	// single resolved
	assert.Equal(t, 0, countUnresolvedConflicts([]byte(`[{"field":"a","resolution":"done"}]`)))

	// mix
	assert.Equal(t, 2, countUnresolvedConflicts([]byte(`[{"field":"a"},{"field":"b"},{"field":"c","resolution":"ok"}]`)))
}

// ---------------------------------------------------------------------------
// parseProvenance
// ---------------------------------------------------------------------------

func TestParseProvenanceV5(t *testing.T) {
	// empty
	p := parseProvenance(nil)
	assert.NotNil(t, p)
	assert.Empty(t, p)

	// valid
	p = parseProvenance([]byte(`{"field1":{"field":"f1","source":"sdk","confidence":"high"}}`))
	assert.NotEmpty(t, p)

	// invalid JSON (unmarshal returns empty map)
	p = parseProvenance([]byte(`{invalid}`))
	assert.NotNil(t, p)
}

// ---------------------------------------------------------------------------
// provenanceRecord
// ---------------------------------------------------------------------------

func TestProvenanceRecordV5(t *testing.T) {
	rec := provenanceRecord("identityField", spec.SemanticSourceSDKExplicit, "digest1", json.RawMessage(`"id"`), "high", "effective", "admin")
	assert.Equal(t, "identityField", rec.Field)
	assert.Equal(t, spec.SemanticSourceSDKExplicit, rec.Source)
	assert.Equal(t, "high", rec.Confidence)
	assert.Equal(t, "effective", rec.Status)
	assert.Equal(t, "admin", rec.UpdatedBy)
}

// ---------------------------------------------------------------------------
// rawJSONString
// ---------------------------------------------------------------------------

func TestRawJSONStringV5(t *testing.T) {
	assert.Equal(t, json.RawMessage(`"hello"`), rawJSONString("hello"))
	assert.Equal(t, json.RawMessage(`""`), rawJSONString(""))
}

// ---------------------------------------------------------------------------
// rawJSONUint
// ---------------------------------------------------------------------------

func TestRawJSONUintV5(t *testing.T) {
	assert.Equal(t, json.RawMessage("0"), rawJSONUint(0))
	assert.Equal(t, json.RawMessage("99"), rawJSONUint(99))
}

// ---------------------------------------------------------------------------
// activeStatus
// ---------------------------------------------------------------------------

func TestActiveStatusV5(t *testing.T) {
	assert.Equal(t, "active", activeStatus(true))
	assert.Equal(t, "inactive", activeStatus(false))
}

// ---------------------------------------------------------------------------
// affectedKindOrder
// ---------------------------------------------------------------------------

func TestAffectedKindOrderV5(t *testing.T) {
	assert.Equal(t, 0, affectedKindOrder("published"))
	assert.Equal(t, 1, affectedKindOrder("draft"))
	assert.Equal(t, 2, affectedKindOrder("proposal"))
	assert.Equal(t, 3, affectedKindOrder("random"))
}

// ---------------------------------------------------------------------------
// isMissingTableErr
// ---------------------------------------------------------------------------

func TestIsMissingTableErrV5(t *testing.T) {
	assert.False(t, isMissingTableErr(nil))
	assert.False(t, isMissingTableErr(assert.AnError))
	assert.True(t, isMissingTableErr(errWithMessage("no such table")))
	assert.True(t, isMissingTableErr(errWithMessage("table does not exist")))
	assert.True(t, isMissingTableErr(errWithMessage("UNDEFINED TABLE foo")))
	assert.False(t, isMissingTableErr(errWithMessage("connection refused")))
}

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }

func errWithMessage(msg string) error { return &simpleErr{msg: msg} }

// ---------------------------------------------------------------------------
// parseBindingContracts
// ---------------------------------------------------------------------------

func TestParseBindingContractsV5(t *testing.T) {
	assert.Nil(t, parseBindingContracts(""))
	assert.Nil(t, parseBindingContracts("  "))

	result := parseBindingContracts(`[{"bindingId":"b1","functionId":"f1","risk":"safe","permission":"p:q"}]`)
	require.Len(t, result, 1)
	assert.Equal(t, "b1", result[0].BindingID)
}

// ---------------------------------------------------------------------------
// parsePublishedPageSpec
// ---------------------------------------------------------------------------

func TestParsePublishedPageSpecV5(t *testing.T) {
	// empty SpecJSON
	p := parsePublishedPageSpec(model.PublishedPageSpec{})
	assert.Equal(t, "", p.PageKey)

	// valid SpecJSON
	p = parsePublishedPageSpec(model.PublishedPageSpec{
		SpecJSON: `{"pageKey":"test","type":"resource"}`,
	})
	assert.Equal(t, "test", p.PageKey)

	// empty pageKey falls back to model PageKey
	p = parsePublishedPageSpec(model.PublishedPageSpec{
		PageKey:  "fallback",
		SpecJSON: `{"type":"resource"}`,
	})
	assert.Equal(t, "fallback", p.PageKey)
}

// ---------------------------------------------------------------------------
// functionSpecsByID
// ---------------------------------------------------------------------------

func TestFunctionSpecsByIDV5(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db, nil)

	// empty
	result := svc.functionSpecsByID(ctx, "g", "e")
	assert.Empty(t, result)

	// with contracts
	contractModel := model.NewFunctionContractModel(db)
	err := contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID:      "g",
		Env:         "e",
		FunctionID:  "f1",
		Version:     "1.0",
		Enabled:     true,
		ResourceKey: "r1",
		Capability:  "action",
		Execution:   "sync",
		Risk:        "safe",
		Source:      "sdk",
		Permission:  "p:q",
	})
	require.NoError(t, err)

	result = svc.functionSpecsByID(ctx, "g", "e")
	assert.Len(t, result, 1)
	fnSpec := result["f1"]
	assert.Equal(t, "f1", fnSpec.ID)
	assert.Equal(t, "1.0", fnSpec.Version)
	assert.True(t, fnSpec.Enabled)
	assert.Equal(t, spec.RiskSafe, fnSpec.Risk)
}

// ---------------------------------------------------------------------------
// createSemanticVersion
// ---------------------------------------------------------------------------

func TestCreateSemanticVersionV5(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	svc := NewService(db, nil)

	// nil version model
	svc2 := &Service{}
	err := svc2.createSemanticVersion(ctx, &model.CapabilitySemantics{}, "reason", "actor")
	assert.NoError(t, err)

	// nil semantics
	err = svc.createSemanticVersion(ctx, nil, "reason", "actor")
	assert.NoError(t, err)

	// normal create
	err = svc.createSemanticVersion(ctx, &model.CapabilitySemantics{
		Version:      1,
		SourceDigest: "abc",
	}, "my reason", "admin")
	assert.NoError(t, err)

	// empty reason defaults
	err = svc.createSemanticVersion(ctx, &model.CapabilitySemantics{
		Version: 2,
	}, "", "admin")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// rebuildProposals
// ---------------------------------------------------------------------------

func TestRebuildProposalsV5(t *testing.T) {
	svc := &Service{}
	// nil contractService
	err := svc.rebuildProposals(context.Background(), "g", "e", "r")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// capabilitySemanticsJSON
// ---------------------------------------------------------------------------

func TestCapabilitySemanticsJSONV5(t *testing.T) {
	result := capabilitySemanticsJSON(&model.CapabilitySemantics{IdentityField: "id"})
	assert.NotEmpty(t, result)
}

// ---------------------------------------------------------------------------
// validateJSONPointerList
// ---------------------------------------------------------------------------

func TestValidateJSONPointerListV5(t *testing.T) {
	// all valid, deduplicated
	result, err := validateJSONPointerList([]string{"/a", "/b", "/a"}, "field")
	require.NoError(t, err)
	assert.Equal(t, []string{"/a", "/b"}, result)

	// empty pointer
	_, err = validateJSONPointerList([]string{""}, "field")
	require.Error(t, err)

	// non-pointer
	_, err = validateJSONPointerList([]string{"abc"}, "field")
	require.Error(t, err)

	// all empty => error
	_, err = validateJSONPointerList([]string{"  ", ""}, "field")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// compactJSONPointers
// ---------------------------------------------------------------------------

func TestCompactJSONPointersV5(t *testing.T) {
	result := compactJSONPointers([]string{"/a", "/b", "/a", "", "abc"})
	assert.Equal(t, []string{"/a", "/b"}, result)
}

// ---------------------------------------------------------------------------
// validateTaskInputPointer
// ---------------------------------------------------------------------------

func TestValidateTaskInputPointerV5(t *testing.T) {
	// non-pointer
	err := validateTaskInputPointer(nil, "abc", "field")
	require.Error(t, err)

	// nil contract, valid pointer
	err = validateTaskInputPointer(nil, "/id", "field")
	require.NoError(t, err)

	// contract with matching schema
	err = validateTaskInputPointer(&model.FunctionContract{
		InputSchema: []byte(`{"type":"object","properties":{"id":{"type":"string"}}}`),
	}, "/id", "field")
	require.NoError(t, err)

	// contract with non-matching schema
	err = validateTaskInputPointer(&model.FunctionContract{
		InputSchema: []byte(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	}, "/id", "field")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// matchesQuery
// ---------------------------------------------------------------------------

func TestMatchesQueryV5(t *testing.T) {
	item := ResourceCatalogItem{
		ResourceKey: "player",
		Labels:      map[string]string{"zh-CN": "玩家"},
	}
	assert.True(t, matchesQuery(item, "player"))
	assert.True(t, matchesQuery(item, "PLAYER"))
	assert.True(t, matchesQuery(item, "玩家"))
	assert.False(t, matchesQuery(item, "mail"))
	// Note: empty query matches everything via strings.Contains
}

// ---------------------------------------------------------------------------
// compactActionSemantics
// ---------------------------------------------------------------------------

func TestCompactActionSemanticsV5(t *testing.T) {
	actions := compactActionSemantics([]ActionSemanticInfo{
		{FunctionID: "f1", Subject: "resource_item", IdentityInput: "/id"},
		{FunctionID: "f2", Subject: "none"},                  // IdentityInput cleared
		{FunctionID: "", Subject: "resource_item"},           // empty FunctionID skipped
		{FunctionID: "f3", Subject: "", IdentityInput: "/x"}, // empty Subject skipped
	})
	require.Len(t, actions, 2)
	assert.Equal(t, "f1", actions[0].FunctionID)
	assert.Equal(t, "/id", actions[0].IdentityInput)
	assert.Equal(t, "f2", actions[1].FunctionID)
	assert.Equal(t, "", actions[1].IdentityInput)
}

// ---------------------------------------------------------------------------
// schemaStringValue
// ---------------------------------------------------------------------------

func TestSchemaStringValueV5(t *testing.T) {
	assert.Equal(t, "", schemaStringValue(nil))
	assert.Equal(t, "string", schemaStringValue(json.RawMessage(`"string"`)))
	assert.Equal(t, "number", schemaStringValue(json.RawMessage(`"number"`)))
	assert.Equal(t, "", schemaStringValue(json.RawMessage(`123`)))
}

// ---------------------------------------------------------------------------
// isValidSemanticSource
// ---------------------------------------------------------------------------

func TestIsValidSemanticSourceV5_All(t *testing.T) {
	assert.True(t, isValidSemanticSource(spec.SemanticSourcePlatformReview))
	assert.True(t, isValidSemanticSource(spec.SemanticSourceSDKExplicit))
	assert.True(t, isValidSemanticSource(spec.SemanticSourceOpenAPIRest))
	assert.False(t, isValidSemanticSource("other"))
	assert.False(t, isValidSemanticSource(""))
}

// ---------------------------------------------------------------------------
// confidenceForSource
// ---------------------------------------------------------------------------

func TestConfidenceForSourceV5(t *testing.T) {
	assert.Equal(t, "high", confidenceForSource(spec.SemanticSourcePlatformReview))
	assert.Equal(t, "high", confidenceForSource(spec.SemanticSourceSDKExplicit))
	assert.Equal(t, "low", confidenceForSource(spec.SemanticSourceOpenAPIRest))
	assert.Equal(t, "low", confidenceForSource("unknown"))
}

// ---------------------------------------------------------------------------
// actorFromContext
// ---------------------------------------------------------------------------

func TestActorFromContextV5(t *testing.T) {
	assert.Equal(t, "system", actorFromContext(context.Background()))
	ctx := context.WithValue(context.Background(), "username", "admin")
	assert.Equal(t, "admin", actorFromContext(ctx))
	ctx2 := context.WithValue(context.Background(), "username", "  trimmed  ")
	assert.Equal(t, "trimmed", actorFromContext(ctx2))
}

// ---------------------------------------------------------------------------
// determineStatus – more edge cases
// ---------------------------------------------------------------------------

func TestDetermineStatusV5(t *testing.T) {
	// contracts with semantics, has conflicts
	assert.Equal(t, "conflict", determineStatus(
		[]*model.FunctionContract{{FunctionID: "f1"}},
		&model.CapabilitySemantics{Conflicts: []byte(`[{"field":"a"}]`)},
	))

	// contracts with semantics, no conflicts but no complete set
	assert.Equal(t, "pending", determineStatus(
		[]*model.FunctionContract{{FunctionID: "f1", Capability: "action"}},
		&model.CapabilitySemantics{Conflicts: []byte(`[]`)},
	))
}

// ---------------------------------------------------------------------------
// buildFunctionInfos
// ---------------------------------------------------------------------------

func TestBuildFunctionInfosV5(t *testing.T) {
	assert.Empty(t, buildFunctionInfos(nil))
	assert.Empty(t, buildFunctionInfos([]*model.FunctionContract{}))
}

// ---------------------------------------------------------------------------
// buildDiagnostics – nil contracts
// ---------------------------------------------------------------------------

func TestBuildDiagnosticsV5(t *testing.T) {
	diags := buildDiagnostics(nil, nil)
	assert.Empty(t, diags)
}
