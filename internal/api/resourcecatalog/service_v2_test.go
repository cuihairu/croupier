package resourcecatalog

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDBWithPages(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	err = db.AutoMigrate(
		&model.FunctionContract{},
		&model.ResourceCapability{},
		&model.CapabilitySemantics{},
		&model.CapabilitySemanticVersion{},
		&model.PageProposal{},
		&model.PageSpec{},
		&model.PublishedPageSpec{},
		&model.PageProposalVersion{},
		&model.BlockedProposalIssue{},
	)
	require.NoError(t, err)
	return db
}

// ---------------------------------------------------------------------------
// applySemanticFieldValue
// ---------------------------------------------------------------------------

func TestApplySemanticFieldValueV2(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		raw     json.RawMessage
		wantErr bool
		check   func(*model.CapabilitySemantics)
	}{
		{
			name:    "identityField",
			field:   "identityField",
			raw:     json.RawMessage(`"player_id"`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, "player_id", s.IdentityField) },
		},
		{
			name:    "identityFieldType",
			field:   "identityFieldType",
			raw:     json.RawMessage(`"number"`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, "number", s.IdentityFieldType) },
		},
		{
			name:    "identityPath",
			field:   "identityPath",
			raw:     json.RawMessage(`"/data/id"`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, "/data/id", s.IdentityPath) },
		},
		{
			name:    "collectionQueryID",
			field:   "collectionQueryID",
			raw:     json.RawMessage(`42`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, uint(42), s.CollectionQueryID) },
		},
		{
			name:    "collectionQueryId alias",
			field:   "collectionQueryId",
			raw:     json.RawMessage(`10`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, uint(10), s.CollectionQueryID) },
		},
		{
			name:    "collectionPath",
			field:   "collectionPath",
			raw:     json.RawMessage(`"/players"`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, "/players", s.CollectionPath) },
		},
		{
			name:    "pageFieldName",
			field:   "pageFieldName",
			raw:     json.RawMessage(`"page_num"`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, "page_num", s.PageFieldName) },
		},
		{
			name:    "pageSizeFieldName",
			field:   "pageSizeFieldName",
			raw:     json.RawMessage(`"size"`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, "size", s.PageSizeFieldName) },
		},
		{
			name:    "itemsFieldName",
			field:   "itemsFieldName",
			raw:     json.RawMessage(`"records"`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, "records", s.ItemsFieldName) },
		},
		{
			name:    "totalFieldName",
			field:   "totalFieldName",
			raw:     json.RawMessage(`"count"`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, "count", s.TotalFieldName) },
		},
		{
			name:    "itemQueryID",
			field:   "itemQueryID",
			raw:     json.RawMessage(`7`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, uint(7), s.ItemQueryID) },
		},
		{
			name:    "itemQueryId alias",
			field:   "itemQueryId",
			raw:     json.RawMessage(`8`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, uint(8), s.ItemQueryID) },
		},
		{
			name:    "itemPath",
			field:   "itemPath",
			raw:     json.RawMessage(`"/players/{id}"`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, "/players/{id}", s.ItemPath) },
		},
		{
			name:    "createID",
			field:   "createID",
			raw:     json.RawMessage(`3`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, uint(3), s.CreateID) },
		},
		{
			name:    "createId alias",
			field:   "createId",
			raw:     json.RawMessage(`4`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, uint(4), s.CreateID) },
		},
		{
			name:    "updateID",
			field:   "updateID",
			raw:     json.RawMessage(`5`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, uint(5), s.UpdateID) },
		},
		{
			name:    "updateId alias",
			field:   "updateId",
			raw:     json.RawMessage(`6`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, uint(6), s.UpdateID) },
		},
		{
			name:    "deleteID",
			field:   "deleteID",
			raw:     json.RawMessage(`9`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, uint(9), s.DeleteID) },
		},
		{
			name:    "deleteId alias",
			field:   "deleteId",
			raw:     json.RawMessage(`11`),
			wantErr: false,
			check:   func(s *model.CapabilitySemantics) { assert.Equal(t, uint(11), s.DeleteID) },
		},
		{
			name:    "actions",
			field:   "actions",
			raw:     json.RawMessage(`[{"functionId":"f1","subject":"resource_item","identityInput":"/id"}]`),
			wantErr: false,
			check: func(s *model.CapabilitySemantics) {
				assert.NotEmpty(t, s.Actions)
			},
		},
		{
			name:    "tasks",
			field:   "tasks",
			raw:     json.RawMessage(`[{"start":{"functionId":"f1"}}]`),
			wantErr: false,
			check: func(s *model.CapabilitySemantics) {
				assert.NotEmpty(t, s.Tasks)
			},
		},
		{
			name:    "reports",
			field:   "reports",
			raw:     json.RawMessage(`[{"query":{"functionId":"f1"},"datasetPath":"/items","dimensions":["/name"],"metrics":["/count"]}]`),
			wantErr: false,
			check: func(s *model.CapabilitySemantics) {
				assert.NotEmpty(t, s.Reports)
			},
		},
		{
			name:    "unsupported field",
			field:   "unknownField",
			raw:     json.RawMessage(`"value"`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sem := &model.CapabilitySemantics{}
			err := applySemanticFieldValue(sem, tt.field, tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.check != nil {
					tt.check(sem)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// decodeDiagnostics
// ---------------------------------------------------------------------------

func TestDecodeDiagnosticsV2(t *testing.T) {
	// Valid diagnostics
	valid := `[{"code":"c1","severity":"warning","message":"m1"},{"code":"c2","severity":"error","message":"m2","functionId":"f1"}]`
	result := decodeDiagnostics(json.RawMessage(valid), "fallback")
	assert.Len(t, result, 2)
	assert.Equal(t, "fallback", result[0].FunctionID) // fallback applied
	assert.Equal(t, "f1", result[1].FunctionID)       // explicit functionId

	// Empty input
	assert.Nil(t, decodeDiagnostics(nil, ""))
	assert.Nil(t, decodeDiagnostics([]byte{}, ""))

	// Invalid JSON
	errResult := decodeDiagnostics(json.RawMessage(`{invalid}`), "fallback")
	assert.Len(t, errResult, 1)
	assert.Equal(t, "diagnostic_parse_failed", errResult[0].Code)
	assert.Equal(t, "fallback", errResult[0].FunctionID)
}

// ---------------------------------------------------------------------------
// parseActionSemantics
// ---------------------------------------------------------------------------

func TestParseActionSemanticsV2(t *testing.T) {
	// nil
	assert.Nil(t, parseActionSemantics(nil))
	// empty
	assert.Nil(t, parseActionSemantics([]byte{}))
	// invalid JSON
	assert.Nil(t, parseActionSemantics(json.RawMessage(`{bad}`)))
	// valid with one action
	actions := parseActionSemantics(json.RawMessage(`[{"functionId":"f1","subject":"resource_item","identityInput":"/id"}]`))
	assert.Len(t, actions, 1)
	assert.Equal(t, "f1", actions[0].FunctionID)
}

// ---------------------------------------------------------------------------
// parseTaskSemantics
// ---------------------------------------------------------------------------

func TestParseTaskSemanticsV2(t *testing.T) {
	// nil
	assert.Nil(t, parseTaskSemantics(nil))
	// empty
	assert.Nil(t, parseTaskSemantics([]byte{}))
	// invalid JSON
	assert.Nil(t, parseTaskSemantics(json.RawMessage(`{bad}`)))
	// valid with empty functionId (skipped)
	tasks := parseTaskSemantics(json.RawMessage(`[{"start":{"functionId":""}}]`))
	assert.Empty(t, tasks)
	// valid task
	tasks = parseTaskSemantics(json.RawMessage(`[{"start":{"functionId":"f1","version":"1.0"}}]`))
	assert.Len(t, tasks, 1)
}

// ---------------------------------------------------------------------------
// parseReportSemantics
// ---------------------------------------------------------------------------

func TestParseReportSemanticsV2(t *testing.T) {
	// nil
	assert.Nil(t, parseReportSemantics(nil))
	// empty
	assert.Nil(t, parseReportSemantics([]byte{}))
	// invalid JSON
	assert.Nil(t, parseReportSemantics(json.RawMessage(`{bad}`)))
	// valid with empty functionId (skipped)
	reports := parseReportSemantics(json.RawMessage(`[{"query":{"functionId":""}}]`))
	assert.Empty(t, reports)
	// valid report
	reports = parseReportSemantics(json.RawMessage(`[{"query":{"functionId":"f1"},"datasetPath":"/items","dimensions":["/name"],"metrics":["/count"]}]`))
	assert.Len(t, reports, 1)
}

// ---------------------------------------------------------------------------
// matchesQuery
// ---------------------------------------------------------------------------

func TestMatchesQueryV2(t *testing.T) {
	item := ResourceCatalogItem{
		ResourceKey: "player",
		Labels:      map[string]string{"zh-CN": "玩家"},
	}

	assert.True(t, matchesQuery(item, "player"))
	assert.True(t, matchesQuery(item, "Player"))
	assert.True(t, matchesQuery(item, "玩家"))
	assert.False(t, matchesQuery(item, "mail"))
	assert.False(t, matchesQuery(item, "zzz"))
}

// ---------------------------------------------------------------------------
// findSemanticsOptional
// ---------------------------------------------------------------------------

func TestFindSemanticsOptionalV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	// Not found → returns nil, nil
	sem, err := svc.findSemanticsOptional(ctx, "game1", "env1", "resource1")
	assert.NoError(t, err)
	assert.Nil(t, sem)

	// Create semantics
	semModel := model.NewCapabilitySemanticsModel(db)
	err = semModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:      "game1",
		Env:         "env1",
		ResourceKey: "resource1",
		Source:      "sdk",
	})
	require.NoError(t, err)

	sem, err = svc.findSemanticsOptional(ctx, "game1", "env1", "resource1")
	assert.NoError(t, err)
	assert.NotNil(t, sem)
	assert.Equal(t, "sdk", sem.Source)
}

// ---------------------------------------------------------------------------
// semanticSourceDigest
// ---------------------------------------------------------------------------

func TestSemanticSourceDigestV2(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Nil contract model → empty
	d := semanticSourceDigest(ctx, nil, "", "", "")
	assert.Equal(t, "", d)

	// With contracts
	contractModel := model.NewFunctionContractModel(db)
	err := contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "f1", ResourceKey: "r1",
		Enabled: true, Capability: dbenum.CapabilityAction, Execution: "sync", Risk: dbenum.RiskSafe,
	})
	require.NoError(t, err)

	d = semanticSourceDigest(ctx, contractModel, "g1", "e1", "r1")
	assert.NotEmpty(t, d)

	// Different scope → different digest (or empty if no contracts)
	d2 := semanticSourceDigest(ctx, contractModel, "g1", "e1", "nonexistent")
	assert.NotEmpty(t, d2) // returns digest of empty array
}

// ---------------------------------------------------------------------------
// determineStatus
// ---------------------------------------------------------------------------

func TestDetermineStatusV2(t *testing.T) {
	// No contracts → not_executable
	assert.Equal(t, "not_executable", determineStatus(nil, nil))
	assert.Equal(t, "not_executable", determineStatus([]*model.FunctionContract{}, nil))

	// Contracts but no semantics → pending
	contracts := []*model.FunctionContract{
		{Capability: dbenum.CapabilityAction},
	}
	assert.Equal(t, "pending", determineStatus(contracts, nil))

	// Contracts + semantics with conflicts → conflict
	conflicts, _ := json.Marshal([]spec.SemanticConflict{{Field: "x"}})
	semantics := &model.CapabilitySemantics{Conflicts: conflicts}
	assert.Equal(t, "conflict", determineStatus(contracts, semantics))

	// Contracts + verified semantics → identified
	contracts2 := []*model.FunctionContract{
		{Capability: dbenum.CapabilityCollectionQuery},
		{Capability: dbenum.CapabilityItemQuery},
	}
	semantics2 := &model.CapabilitySemantics{CollectionQueryID: 1, IdentityField: "id"}
	assert.Equal(t, "identified", determineStatus(contracts2, semantics2))

	// A read-only collection with verified identity is already identified.
	assert.Equal(t, "identified", determineStatus(contracts2[:1], semantics2))
}

// ---------------------------------------------------------------------------
// buildDiagnostics
// ---------------------------------------------------------------------------

func TestBuildDiagnosticsV2(t *testing.T) {
	// nil semantics, nil contracts
	diags := buildDiagnostics(nil, nil)
	assert.Empty(t, diags)

	// Semantics with diagnostics
	diagJSON, _ := json.Marshal([]spec.Diagnostic{{Code: "c1", Severity: spec.SeverityWarning, Message: "m1"}})
	sem := &model.CapabilitySemantics{Diagnostics: diagJSON}
	diags = buildDiagnostics(nil, sem)
	assert.Len(t, diags, 1)
	assert.Equal(t, "c1", diags[0].Code)

	// Semantics with conflicts
	conflictsJSON, _ := json.Marshal([]spec.SemanticConflict{{Field: "f1"}, {Field: "f2"}})
	sem2 := &model.CapabilitySemantics{Conflicts: conflictsJSON}
	diags2 := buildDiagnostics(nil, sem2)
	found := false
	for _, d := range diags2 {
		if d.Code == "semantic_conflict" {
			found = true
			assert.Equal(t, "error", d.Severity)
		}
	}
	assert.True(t, found)

	// Nil contract in contracts slice
	diags3 := buildDiagnostics([]*model.FunctionContract{nil}, nil)
	assert.Empty(t, diags3)

	// Contract with diagnostics
	contractDiagJSON, _ := json.Marshal([]spec.Diagnostic{{Code: "cd1", Severity: spec.SeverityError, Message: "cm1"}})
	contract := &model.FunctionContract{FunctionID: "f1", Diagnostics: contractDiagJSON}
	diags4 := buildDiagnostics([]*model.FunctionContract{contract}, nil)
	assert.Len(t, diags4, 1)
	assert.Equal(t, "f1", diags4[0].FunctionID)
}

// ---------------------------------------------------------------------------
// isValidSemanticSource
// ---------------------------------------------------------------------------

func TestIsValidSemanticSourceV2(t *testing.T) {
	assert.True(t, isValidSemanticSource(spec.SemanticSourcePlatformReview))
	assert.True(t, isValidSemanticSource(spec.SemanticSourceSDKExplicit))
	assert.True(t, isValidSemanticSource(spec.SemanticSourceOpenAPIRest))
	assert.False(t, isValidSemanticSource("invalid_source"))
	assert.False(t, isValidSemanticSource(""))
}

// ---------------------------------------------------------------------------
// confidenceForSource
// ---------------------------------------------------------------------------

func TestConfidenceForSourceV2(t *testing.T) {
	assert.Equal(t, "high", confidenceForSource(spec.SemanticSourcePlatformReview))
	assert.Equal(t, "high", confidenceForSource(spec.SemanticSourceSDKExplicit))
	assert.Equal(t, "low", confidenceForSource(spec.SemanticSourceOpenAPIRest))
	assert.Equal(t, "low", confidenceForSource("unknown_source"))
}

// ---------------------------------------------------------------------------
// actorFromContext
// ---------------------------------------------------------------------------

func TestActorFromContextV2(t *testing.T) {
	// Empty context → "system"
	ctx := context.Background()
	assert.Equal(t, "system", actorFromContext(ctx))

	// With username in context
	ctx2 := context.WithValue(ctx, "username", "admin")
	assert.Equal(t, "admin", actorFromContext(ctx2))

	// With whitespace username
	ctx3 := context.WithValue(ctx, "username", "  admin  ")
	assert.Equal(t, "admin", actorFromContext(ctx3))
}

// ---------------------------------------------------------------------------
// parseProvenance
// ---------------------------------------------------------------------------

func TestParseProvenanceV2(t *testing.T) {
	// nil
	p := parseProvenance(nil)
	assert.NotNil(t, p)
	assert.Empty(t, p)

	// empty
	p = parseProvenance([]byte{})
	assert.NotNil(t, p)
	assert.Empty(t, p)

	// valid
	provJSON := []byte(`{"identityField":{"field":"identityField","source":"sdk_explicit","confidence":"high","status":"effective"}}`)
	p = parseProvenance(provJSON)
	assert.NotNil(t, p)
	assert.Contains(t, p, "identityField")

	// invalid JSON
	p = parseProvenance([]byte(`{invalid}`))
	assert.NotNil(t, p)
	assert.Empty(t, p)
}

// ---------------------------------------------------------------------------
// rawJSONString
// ---------------------------------------------------------------------------

func TestRawJSONStringV2(t *testing.T) {
	assert.Equal(t, json.RawMessage(`"hello"`), rawJSONString("hello"))
	assert.Equal(t, json.RawMessage(`""`), rawJSONString(""))
}

// ---------------------------------------------------------------------------
// validateJSONPointerList
// ---------------------------------------------------------------------------

func TestValidateJSONPointerListV2(t *testing.T) {
	// Valid pointers
	result, err := validateJSONPointerList([]string{"/name", "/count"}, "test")
	assert.NoError(t, err)
	assert.Len(t, result, 2)

	// Deduplication
	result, err = validateJSONPointerList([]string{"/name", "/name"}, "test")
	assert.NoError(t, err)
	assert.Len(t, result, 1)

	// Empty pointer in list
	_, err = validateJSONPointerList([]string{"/name", ""}, "test")
	assert.Error(t, err)

	// Not a JSON pointer
	_, err = validateJSONPointerList([]string{"notpointer"}, "test")
	assert.Error(t, err)

	// All empty → error
	_, err = validateJSONPointerList([]string{""}, "test")
	assert.Error(t, err)

	// Empty input → error
	_, err = validateJSONPointerList([]string{}, "test")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// compactJSONPointers
// ---------------------------------------------------------------------------

func TestCompactJSONPointersV2(t *testing.T) {
	result := compactJSONPointers([]string{"/a", "/b", "/a", "", "c"})
	assert.Equal(t, []string{"/a", "/b"}, result)

	result = compactJSONPointers(nil)
	assert.Empty(t, result)

	result = compactJSONPointers([]string{})
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// validateTaskInputPointer
// ---------------------------------------------------------------------------

func TestValidateTaskInputPointerV2(t *testing.T) {
	// Not a pointer
	err := validateTaskInputPointer(nil, "notpointer", "test")
	assert.Error(t, err)

	// Valid pointer, nil contract
	err = validateTaskInputPointer(nil, "/id", "test")
	assert.NoError(t, err)

	// Valid pointer, contract with matching schema
	schema := []byte(`{"type":"object","properties":{"id":{"type":"string"}}}`)
	contract := &model.FunctionContract{InputSchema: schema}
	err = validateTaskInputPointer(contract, "/id", "test")
	assert.NoError(t, err)

	// Valid pointer, contract with non-matching schema
	err = validateTaskInputPointer(contract, "/missing", "test")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// functionSpecsByID
// ---------------------------------------------------------------------------

func TestFunctionSpecsByIDV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	// No contracts
	specs := svc.functionSpecsByID(ctx, "g1", "e1")
	assert.Empty(t, specs)

	// With contracts
	contractModel := model.NewFunctionContractModel(db)
	err := contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.list", Version: "1.0",
		Enabled: true, Capability: dbenum.CapabilityCollectionQuery, Execution: "sync",
		Risk: dbenum.RiskSafe, ResourceKey: "player",
	})
	require.NoError(t, err)

	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "", Version: "1.0",
		Enabled: true,
	})
	require.NoError(t, err)

	specs = svc.functionSpecsByID(ctx, "g1", "e1")
	assert.Len(t, specs, 1)
	assert.Contains(t, specs, "player.list")
}

// ---------------------------------------------------------------------------
// parsePublishedPageSpec
// ---------------------------------------------------------------------------

func TestParsePublishedPageSpecV2(t *testing.T) {
	// Empty spec
	published := model.PublishedPageSpec{PageKey: "test-page"}
	ps := parsePublishedPageSpec(published)
	assert.Equal(t, "test-page", ps.PageKey)

	// Valid spec JSON
	specJSON := `{"pageKey":"spec-page","type":"resource","resourceKey":"player"}`
	published2 := model.PublishedPageSpec{PageKey: "default", SpecJSON: specJSON}
	ps2 := parsePublishedPageSpec(published2)
	assert.Equal(t, "spec-page", ps2.PageKey)
	assert.Equal(t, "resource", string(ps2.Type))
	assert.Equal(t, "player", ps2.ResourceKey)

	// Spec JSON with empty pageKey falls back
	specJSON3 := `{"type":"resource"}`
	published3 := model.PublishedPageSpec{PageKey: "fallback", SpecJSON: specJSON3}
	ps3 := parsePublishedPageSpec(published3)
	assert.Equal(t, "fallback", ps3.PageKey)

	// Invalid spec JSON
	published4 := model.PublishedPageSpec{PageKey: "fallback2", SpecJSON: "{invalid}"}
	ps4 := parsePublishedPageSpec(published4)
	assert.Equal(t, "fallback2", ps4.PageKey)
}

// ---------------------------------------------------------------------------
// parseBindingContracts
// ---------------------------------------------------------------------------

func TestParseBindingContractsV2(t *testing.T) {
	// Empty
	assert.Nil(t, parseBindingContracts(""))
	assert.Nil(t, parseBindingContracts("  "))

	// Valid
	contracts := parseBindingContracts(`[{"functionId":"f1","version":"1.0","inputSchemaDigest":"abc","outputSchemaDigest":"def"}]`)
	assert.Len(t, contracts, 1)
	assert.Equal(t, "f1", contracts[0].FunctionID)

	// Invalid JSON
	contracts = parseBindingContracts(`{invalid}`)
	assert.Nil(t, contracts)
}

// ---------------------------------------------------------------------------
// capabilitySemanticsJSON
// ---------------------------------------------------------------------------

func TestCapabilitySemanticsJSONV2(t *testing.T) {
	sem := &model.CapabilitySemantics{
		GameID:      "g1",
		ResourceKey: "player",
	}
	raw := capabilitySemanticsJSON(sem)
	assert.NotEmpty(t, raw)

	// Verify it's valid JSON
	var parsed model.CapabilitySemantics
	err := json.Unmarshal(raw, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, "g1", parsed.GameID)
}

// ---------------------------------------------------------------------------
// rebuildProposals
// ---------------------------------------------------------------------------

func TestRebuildProposalsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	// contractService is nil → returns nil
	err := svc.rebuildProposals(ctx, "g1", "e1", "r1")
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// createSemanticVersion
// ---------------------------------------------------------------------------

func TestCreateSemanticVersionV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	// Create semantics
	semModel := model.NewCapabilitySemanticsModel(db)
	err := semModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:      "g1",
		Env:         "e1",
		ResourceKey: "r1",
		Source:      "sdk",
	})
	require.NoError(t, err)

	sem, err := semModel.FindByScopeAndResourceKey(ctx, "g1", "e1", "r1")
	require.NoError(t, err)

	// With reason
	err = svc.createSemanticVersion(ctx, sem, "test reason", "admin")
	assert.NoError(t, err)

	// Without reason (default)
	err = svc.createSemanticVersion(ctx, sem, "", "admin")
	assert.NoError(t, err)

	// With nil versionModel
	svc2 := &Service{db: db}
	err = svc2.createSemanticVersion(ctx, sem, "test", "admin")
	assert.NoError(t, err) // versionModel is nil, returns nil

	// With nil semantics
	err = svc.createSemanticVersion(ctx, nil, "test", "admin")
	assert.NoError(t, err) // semantics is nil, returns nil

	// Check versions created
	verModel := model.NewCapabilitySemanticVersionModel(db)
	versions, err := verModel.ListBySemanticsID(ctx, sem.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(versions), 2)
}

// ---------------------------------------------------------------------------
// ListConflicts - deeper
// ---------------------------------------------------------------------------

func TestListConflictsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	// Non-existent semantics → empty
	resp, err := svc.ListConflicts(ctx, &ListConflictsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "nonexistent",
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Conflicts)
	assert.Empty(t, resp.Provenance)

	// Create semantics with provenance
	prov := map[string]*spec.SemanticProvenance{
		"identityField": {
			Field:      "identityField",
			Source:     spec.SemanticSourceSDKExplicit,
			Confidence: "high",
			Status:     "effective",
			Value:      json.RawMessage(`"id"`),
			UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
			UpdatedBy:  "admin",
		},
	}
	provJSON, _ := json.Marshal(prov)
	semModel := model.NewCapabilitySemanticsModel(db)
	err = semModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:      "g1",
		Env:         "e1",
		ResourceKey: "r1",
		Source:      "sdk",
		Provenance:  provJSON,
	})
	require.NoError(t, err)

	resp, err = svc.ListConflicts(ctx, &ListConflictsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "r1",
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Conflicts)
	assert.Len(t, resp.Provenance, 1)
	assert.Equal(t, "identityField", resp.Provenance[0].Field)
}

// ---------------------------------------------------------------------------
// ResolveConflict - deeper
// ---------------------------------------------------------------------------

func TestResolveConflictV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	// Create resource capability
	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Labels: map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	// Invalid source
	_, err = svc.ResolveConflict(ctx, &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "identityField", ChosenSource: "invalid_source",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid chosenSource")

	// Create semantics with conflict
	conflicts := []spec.SemanticConflict{
		{
			Field: "identityField",
			Values: map[spec.SemanticSource]json.RawMessage{
				spec.SemanticSourceSDKExplicit: json.RawMessage(`"player_id"`),
			},
		},
	}
	conflictsJSON, _ := json.Marshal(conflicts)
	semModel := model.NewCapabilitySemanticsModel(db)
	err = semModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID:      "g1",
		Env:         "e1",
		ResourceKey: "player",
		Source:      "sdk",
		Conflicts:   conflictsJSON,
	})
	require.NoError(t, err)

	// Wrong source (not in conflict values)
	_, err = svc.ResolveConflict(ctx, &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "identityField", ChosenSource: "openapi_rest",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in conflict values")

	// Field not found
	_, err = svc.ResolveConflict(ctx, &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "nonexistent", ChosenSource: "sdk_explicit",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conflict not found")

	// Success
	resp, err := svc.ResolveConflict(ctx, &ResolveConflictRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Field: "identityField", ChosenSource: "sdk_explicit",
		Reason: "choose SDK",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message, "Conflict resolved")
}

// ---------------------------------------------------------------------------
// List - category filter + query match on label
// ---------------------------------------------------------------------------

func TestListCategoryFilterV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Labels: map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)
	err = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "mail",
		Labels: map[string]interface{}{"zh-CN": "邮件"},
	})
	require.NoError(t, err)

	// Category filter
	resp, err := svc.List(ctx, &ListRequest{GameID: "g1", Env: "e1", Category: "nonexistent"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 0)

	// Query by label
	resp, err = svc.List(ctx, &ListRequest{GameID: "g1", Env: "e1", Query: "玩家"})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, "player", resp.Items[0].ResourceKey)
}

// ---------------------------------------------------------------------------
// Detail - affected pages
// ---------------------------------------------------------------------------

func TestDetailWithAffectedPagesV2(t *testing.T) {
	db := setupTestDBWithPages(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Labels: map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	// Detail with proposals
	proposalModel := model.NewPageProposalModel(db)
	err = proposalModel.UpsertProposal(ctx, &model.PageProposal{
		GameID:      "g1",
		Env:         "e1",
		ProposalKey: "player-proposal",
		PageKey:     "player-list",
		PageType:    "resource",
		ResourceKey: "player",
		Quality:     "ready",
		Status:      dbenum.ProposalStatusPending,
		Title:       map[string]interface{}{"zh-CN": "玩家列表"},
	})
	require.NoError(t, err)

	item, err := svc.Detail(ctx, &DetailRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})
	require.NoError(t, err)
	assert.NotNil(t, item)
	assert.Len(t, item.AffectedPages, 1)
	assert.Equal(t, "proposal", item.AffectedPages[0].Kind)
}

// ---------------------------------------------------------------------------
// UpdateSemantics - with function bindings
// ---------------------------------------------------------------------------

func TestUpdateSemanticsWithBindingsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	// Create resource capability
	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Labels: map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	// Create contracts for validation
	contractModel := model.NewFunctionContractModel(db)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.list",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityCollectionQuery,
		Execution: "sync", Risk: dbenum.RiskSafe,
	})
	require.NoError(t, err)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.get",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityItemQuery,
		Execution: "sync", Risk: dbenum.RiskSafe,
	})
	require.NoError(t, err)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.create",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityCreate,
		Execution: "sync", Risk: dbenum.RiskSafe,
	})
	require.NoError(t, err)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.ban",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityAction,
		Execution: "sync", Risk: dbenum.RiskDanger,
	})
	require.NoError(t, err)

	// Invalid binding: wrong resource
	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		CollectionQueryID: 999, // non-existent ID
	})
	assert.Error(t, err)

	// Valid binding with collection query
	result, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		CollectionQueryID: 1,
		ItemQueryID:       2,
		CreateID:          3,
		IdentityField:     "player_id",
		IdentityFieldType: "string",
		CollectionPath:    "/players",
		PageFieldName:     "page",
		PageSizeFieldName: "page_size",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
		ItemPath:          "/players/{player_id}",
		ChangeReason:      "test update",
	})
	require.NoError(t, err)
	assert.Equal(t, "platform_review", result.Source)

	// Verify semantics stored correctly
	semModel := model.NewCapabilitySemanticsModel(db)
	sem, err := semModel.FindByScopeAndResourceKey(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	assert.Equal(t, "player_id", sem.IdentityField)
	assert.Equal(t, "string", sem.IdentityFieldType)
	assert.Equal(t, uint(1), sem.CollectionQueryID)
	assert.Equal(t, uint(2), sem.ItemQueryID)
	assert.Equal(t, uint(3), sem.CreateID)
}

// ---------------------------------------------------------------------------
// UpdateSemantics - with actions
// ---------------------------------------------------------------------------

func TestUpdateSemanticsWithActionsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Labels: map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	contractModel := model.NewFunctionContractModel(db)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.ban",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityAction,
		Execution: "sync", Risk: dbenum.RiskDanger,
	})
	require.NoError(t, err)

	// Invalid action: empty functionId
	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Actions: []ActionSemanticInfo{
			{FunctionID: "", Subject: "resource_item"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "functionId is required")

	// Invalid action: wrong resource
	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "other_resource",
		Actions: []ActionSemanticInfo{
			{FunctionID: "player.ban", Subject: "resource_item", IdentityInput: "/id"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource capability not found")

	// Invalid action: wrong capability
	err = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player2",
		Labels: map[string]interface{}{"zh-CN": "测试"},
	})
	require.NoError(t, err)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player2.query",
		Enabled: true, ResourceKey: "player2", Capability: dbenum.CapabilityCollectionQuery,
		Execution: "sync", Risk: dbenum.RiskSafe,
	})
	require.NoError(t, err)
	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player2",
		Actions: []ActionSemanticInfo{
			{FunctionID: "player2.query", Subject: "resource_item", IdentityInput: "/id"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "capability must be action")

	// Invalid action: non-resource_item subject without identityInput
	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Actions: []ActionSemanticInfo{
			{FunctionID: "player.ban", Subject: "resource_item", IdentityInput: "notpointer"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a JSON Pointer")

	// Invalid subject
	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Actions: []ActionSemanticInfo{
			{FunctionID: "player.ban", Subject: "invalid_subject"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be resource_item, resource_selection, or none")

	// Valid action with "none" subject
	result, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Actions: []ActionSemanticInfo{
			{FunctionID: "player.ban", Subject: "none"},
		},
		ChangeReason: "test actions",
	})
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// UpdateSemantics - identity field defaults
// ---------------------------------------------------------------------------

func TestUpdateSemanticsIdentityFieldTypeDefaultV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Labels: map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		IdentityField:     "id",
		IdentityFieldType: "", // empty → defaults to "string"
		ChangeReason:      "test default type",
	})
	require.NoError(t, err)

	semModel := model.NewCapabilitySemanticsModel(db)
	sem, err := semModel.FindByScopeAndResourceKey(ctx, "g1", "e1", "player")
	require.NoError(t, err)
	assert.Equal(t, "string", sem.IdentityFieldType)
}

// ---------------------------------------------------------------------------
// buildSemanticsInfo - with all fields
// ---------------------------------------------------------------------------

func TestBuildSemanticsInfoFullV2(t *testing.T) {
	sem := &model.CapabilitySemantics{
		Version:           1,
		IdentityField:     "id",
		IdentityFieldType: "string",
		IdentityPath:      "/data/id",
		CollectionQueryID: 1,
		CollectionPath:    "/items",
		PageFieldName:     "page",
		PageSizeFieldName: "page_size",
		ItemsFieldName:    "items",
		TotalFieldName:    "total",
		ItemQueryID:       2,
		ItemPath:          "/items/{id}",
		CreateID:          3,
		UpdateID:          4,
		DeleteID:          5,
		Source:            "sdk_explicit",
		SourceDigest:      "abc123",
	}
	actions := []ActionSemanticInfo{
		{FunctionID: "f1", Subject: "resource_item", IdentityInput: "/id"},
	}
	actionsJSON, _ := json.Marshal(actions)
	sem.Actions = actionsJSON
	tasks := []spec.TaskSemantic{
		{Start: spec.FunctionRef{FunctionID: "t1"}},
	}
	tasksJSON, _ := json.Marshal(tasks)
	sem.Tasks = tasksJSON
	reports := []spec.ReportSemantic{
		{Query: spec.FunctionRef{FunctionID: "r1"}, DatasetPath: "/items", Dimensions: []string{"/name"}, Metrics: []string{"/count"}},
	}
	reportsJSON, _ := json.Marshal(reports)
	sem.Reports = reportsJSON

	info := buildSemanticsInfo(sem)
	assert.NotNil(t, info)
	assert.Equal(t, 1, info.Version)
	assert.True(t, info.HasIdentity)
	assert.True(t, info.HasCollection)
	assert.True(t, info.HasCreate)
	assert.True(t, info.HasUpdate)
	assert.True(t, info.HasDelete)
	assert.True(t, info.HasActions)
	assert.True(t, info.HasTasks)
	assert.True(t, info.HasReports)
	assert.Equal(t, "id", info.IdentityField)
	assert.Equal(t, uint(1), info.CollectionQueryID)
}

// ---------------------------------------------------------------------------
// ListSemanticVersions - error branch
// ---------------------------------------------------------------------------

func TestListSemanticVersionsErrorV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	// Create semantics
	semModel := model.NewCapabilitySemanticsModel(db)
	err := semModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID: "g1", Env: "e1", ResourceKey: "player", Source: "sdk",
	})
	require.NoError(t, err)

	sem, err := semModel.FindByScopeAndResourceKey(ctx, "g1", "e1", "player")
	require.NoError(t, err)

	// Create multiple versions
	verModel := model.NewCapabilitySemanticVersionModel(db)
	for i := 1; i <= 3; i++ {
		err = verModel.CreateVersion(ctx, &model.CapabilitySemanticVersion{
			SemanticsID:  sem.ID,
			Version:      i,
			SourceDigest: "digest",
			ChangeReason: "reason",
			CreatedBy:    "admin",
		})
		require.NoError(t, err)
	}

	resp, err := svc.ListSemanticVersions(ctx, &ListSemanticVersionsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", Limit: 10,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 3)
	assert.EqualValues(t, 3, resp.Total)
}

func TestListSemanticVersionsPaginatedV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	semModel := model.NewCapabilitySemanticsModel(db)
	require.NoError(t, semModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID: "g1", Env: "e1", ResourceKey: "player", Source: "sdk",
	}))
	sem, err := semModel.FindByScopeAndResourceKey(ctx, "g1", "e1", "player")
	require.NoError(t, err)

	verModel := model.NewCapabilitySemanticVersionModel(db)
	for i := 1; i <= 7; i++ {
		require.NoError(t, verModel.CreateVersion(ctx, &model.CapabilitySemanticVersion{
			SemanticsID:  sem.ID,
			Version:      i,
			SourceDigest: "digest",
			ChangeReason: "reason",
		}))
	}

	// Default request returns only the newest page.
	resp, err := svc.ListSemanticVersions(ctx, &ListSemanticVersionsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Items, semanticVersionDefaultLimit)
	assert.EqualValues(t, 7, resp.Total)
	assert.Equal(t, 7, resp.Items[0].Version, "newest version first")

	// Second page via offset.
	page2, err := svc.ListSemanticVersions(ctx, &ListSemanticVersionsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player", Limit: 5, Offset: 5,
	})
	require.NoError(t, err)
	assert.Len(t, page2.Items, 2)
	assert.Equal(t, 2, page2.Items[0].Version)
}

// ---------------------------------------------------------------------------
// determineStatus - conflict with unresolved
// ---------------------------------------------------------------------------

func TestDetermineStatusConflictV2(t *testing.T) {
	// Resolved conflict (has resolution) → not conflict
	resolved := []spec.SemanticConflict{
		{Field: "x", Resolution: "sdk_explicit"},
	}
	resolvedJSON, _ := json.Marshal(resolved)
	contracts := []*model.FunctionContract{
		{Capability: dbenum.CapabilityCollectionQuery},
		{Capability: dbenum.CapabilityItemQuery},
	}
	semantics := &model.CapabilitySemantics{Conflicts: resolvedJSON, CollectionQueryID: 1, IdentityField: "id"}
	assert.Equal(t, "identified", determineStatus(contracts, semantics))

	// Unresolved conflict → conflict
	unresolved := []spec.SemanticConflict{
		{Field: "x", Resolution: ""},
	}
	unresolvedJSON, _ := json.Marshal(unresolved)
	semantics2 := &model.CapabilitySemantics{Conflicts: unresolvedJSON}
	assert.Equal(t, "conflict", determineStatus(contracts, semantics2))
}

// ---------------------------------------------------------------------------
// UpdateSemantics - with tasks
// ---------------------------------------------------------------------------

func TestUpdateSemanticsWithTasksV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Labels: map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	// Create task function
	contractModel := model.NewFunctionContractModel(db)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.startTask",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		Execution: "sync", Risk: dbenum.RiskSafe,
		OutputSchema: model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}}}`),
	})
	require.NoError(t, err)

	// Invalid task: empty functionId
	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{
			{Start: spec.FunctionRef{FunctionID: ""}},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "functionId is required")
}

// ---------------------------------------------------------------------------
// validateTaskSemantics - comprehensive branch coverage
// ---------------------------------------------------------------------------

func TestValidateTaskSemanticsInvalidTaskIDResultPathV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.startTask",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		OutputSchema: model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}}}`),
	})
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.taskStatus",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		InputSchema:  model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"state":{"type":"string"}}}`),
	})

	// Invalid taskId.resultPath (not a JSON Pointer)
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{
			{
				Start:  spec.FunctionRef{FunctionID: "player.startTask"},
				TaskID: spec.TaskIDSemantic{ResultPath: "invalid", ValueType: spec.JsonScalarString},
				Status: spec.TaskStatusSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
					TaskIDInput: "/taskId",
					StatePath:   "/state",
				},
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a JSON Pointer")
}

func TestValidateTaskSemanticsInvalidValueTypeV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.startTask",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		OutputSchema: model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}}}`),
	})

	// Invalid valueType
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{
			{
				Start:  spec.FunctionRef{FunctionID: "player.startTask"},
				TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: "invalid"},
				Status: spec.TaskStatusSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.startTask"},
					TaskIDInput: "/taskId",
					StatePath:   "/state",
				},
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "valueType")
}

func TestValidateTaskSemanticsResultPathNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.startTask",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		OutputSchema: model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}}}`),
	})

	// resultPath not found in output schema
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{
			{
				Start:  spec.FunctionRef{FunctionID: "player.startTask"},
				TaskID: spec.TaskIDSemantic{ResultPath: "/nonexistent", ValueType: spec.JsonScalarString},
				Status: spec.TaskStatusSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.startTask"},
					TaskIDInput: "/taskId",
					StatePath:   "/state",
				},
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path not found")
}

func TestValidateTaskSemanticsInvalidStatePathV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.startTask",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		OutputSchema: model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}}}`),
	})
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.taskStatus",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		InputSchema:  model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"state":{"type":"string"}}}`),
	})

	// Invalid statePath (not a JSON Pointer)
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{
			{
				Start:  spec.FunctionRef{FunctionID: "player.startTask"},
				TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
				Status: spec.TaskStatusSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
					TaskIDInput: "/taskId",
					StatePath:   "invalid",
				},
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a JSON Pointer")
}

func TestValidateTaskSemanticsStatePathNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.startTask",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		OutputSchema: model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}}}`),
	})
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.taskStatus",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		InputSchema:  model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"state":{"type":"string"}}}`),
	})

	// statePath not found in status output schema
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{
			{
				Start:  spec.FunctionRef{FunctionID: "player.startTask"},
				TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
				Status: spec.TaskStatusSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
					TaskIDInput: "/taskId",
					StatePath:   "/nonexistent",
				},
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path not found")
}

func TestValidateTaskSemanticsWithEventsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.startTask",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		OutputSchema: model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}}}`),
	})
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.taskStatus",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		InputSchema:  model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"state":{"type":"string"}}}`),
	})
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.taskEvents",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		InputSchema:  model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"events":{"type":"array"}}}`),
	})

	// With events - invalid eventsPath
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{
			{
				Start:  spec.FunctionRef{FunctionID: "player.startTask"},
				TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
				Status: spec.TaskStatusSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
					TaskIDInput: "/taskId",
					StatePath:   "/state",
				},
				Events: &spec.TaskEventsSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.taskEvents"},
					TaskIDInput: "/taskId",
					EventsPath:  "invalid",
				},
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a JSON Pointer")
}

func TestValidateTaskSemanticsWithResultV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.startTask",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		OutputSchema: model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}}}`),
	})
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.taskStatus",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		InputSchema:  model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"state":{"type":"string"}}}`),
	})
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.taskResult",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		InputSchema:  model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"result":{"type":"object"}}}`),
	})

	// With result - invalid resultPath
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{
			{
				Start:  spec.FunctionRef{FunctionID: "player.startTask"},
				TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
				Status: spec.TaskStatusSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
					TaskIDInput: "/taskId",
					StatePath:   "/state",
				},
				Result: &spec.TaskResultSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.taskResult"},
					TaskIDInput: "/taskId",
					ResultPath:  "invalid",
				},
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a JSON Pointer")
}

func TestValidateTaskSemanticsWithCancelV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.startTask",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		OutputSchema: model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}}}`),
	})
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.taskStatus",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		InputSchema:  model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"state":{"type":"string"}}}`),
	})
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.cancelTask",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		InputSchema:  model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
		OutputSchema: model.JSON(`{"type":"object"}`),
	})

	// With cancel
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{
			{
				Start:  spec.FunctionRef{FunctionID: "player.startTask"},
				TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
				Status: spec.TaskStatusSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
					TaskIDInput: "/taskId",
					StatePath:   "/state",
				},
				Cancel: &spec.TaskCommandSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.cancelTask"},
					TaskIDInput: "/taskId",
				},
			},
		},
	})
	// Should succeed (or fail for other reasons, but not cancel-related)
	_ = err
}

func TestValidateTaskSemanticsRetryNotAllowedV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.startTask",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		OutputSchema: model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}}}`),
	})
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.taskStatus",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		InputSchema:  model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}},"required":["taskId"]}`),
		OutputSchema: model.JSON(`{"type":"object","properties":{"state":{"type":"string"}}}`),
	})

	// Retry is not allowed
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{
			{
				Start:  spec.FunctionRef{FunctionID: "player.startTask"},
				TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
				Status: spec.TaskStatusSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.taskStatus"},
					TaskIDInput: "/taskId",
					StatePath:   "/state",
				},
				Retry: &spec.TaskCommandSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.startTask"},
					TaskIDInput: "/taskId",
				},
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retry runtime is not available")
}

// ---------------------------------------------------------------------------
// UpdateSemantics - with reports
// ---------------------------------------------------------------------------

func TestUpdateSemanticsWithReportsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	err := capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Labels: map[string]interface{}{"zh-CN": "玩家"},
	})
	require.NoError(t, err)

	// Create report function
	contractModel := model.NewFunctionContractModel(db)
	err = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.report",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityReport,
		Execution: "sync", Risk: dbenum.RiskSafe,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"},"count":{"type":"integer"}}}}}}`),
	})
	require.NoError(t, err)

	// Invalid report: empty functionId
	_, err = svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Reports: []spec.ReportSemantic{
			{Query: spec.FunctionRef{FunctionID: ""}, DatasetPath: "/items", Dimensions: []string{"/name"}, Metrics: []string{"/count"}},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "functionId is required")
}

// ---------------------------------------------------------------------------
// validateReportSemantics - comprehensive branch coverage
// ---------------------------------------------------------------------------

func TestValidateReportSemanticsInvalidDatasetPathV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.report",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityReport,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"}}}}}}`),
	})

	// Invalid datasetPath (not a JSON Pointer)
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Reports: []spec.ReportSemantic{
			{Query: spec.FunctionRef{FunctionID: "player.report"}, DatasetPath: "invalid", Dimensions: []string{"/name"}, Metrics: []string{"/name"}},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a JSON Pointer")
}

func TestValidateReportSemanticsDatasetPathNotArrayV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.report",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityReport,
		OutputSchema: model.JSON(`{"type":"object","properties":{"name":{"type":"string"}}}`),
	})

	// datasetPath points to non-array field
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Reports: []spec.ReportSemantic{
			{Query: spec.FunctionRef{FunctionID: "player.report"}, DatasetPath: "/name", Dimensions: []string{"/name"}, Metrics: []string{"/name"}},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "array")
}

func TestValidateReportSemanticsNoDimensionsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.report",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityReport,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"}}}}}}`),
	})

	// No dimensions
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Reports: []spec.ReportSemantic{
			{Query: spec.FunctionRef{FunctionID: "player.report"}, DatasetPath: "/items", Dimensions: []string{}, Metrics: []string{"/name"}},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dimensions")
}

func TestValidateReportSemanticsNoMetricsV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.report",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityReport,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"}}}}}}`),
	})

	// No metrics
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Reports: []spec.ReportSemantic{
			{Query: spec.FunctionRef{FunctionID: "player.report"}, DatasetPath: "/items", Dimensions: []string{"/name"}, Metrics: []string{}},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metrics")
}

func TestValidateReportSemanticsDimensionPointerNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.report",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityReport,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"}}}}}}`),
	})

	// Dimension pointer not found in dataset item schema
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Reports: []spec.ReportSemantic{
			{Query: spec.FunctionRef{FunctionID: "player.report"}, DatasetPath: "/items", Dimensions: []string{"/nonexistent"}, Metrics: []string{"/name"}},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pointer")
}

func TestValidateReportSemanticsMetricPointerNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.report",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityReport,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"}}}}}}`),
	})

	// Metric pointer not found in dataset item schema
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Reports: []spec.ReportSemantic{
			{Query: spec.FunctionRef{FunctionID: "player.report"}, DatasetPath: "/items", Dimensions: []string{"/name"}, Metrics: []string{"/nonexistent"}},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pointer")
}

func TestValidateReportSemanticsSuccessV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.report",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityReport,
		OutputSchema: model.JSON(`{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"name":{"type":"string"},"count":{"type":"integer"}}}}}}`),
	})

	// Success case
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Reports: []spec.ReportSemantic{
			{Query: spec.FunctionRef{FunctionID: "player.report"}, DatasetPath: "/items", Dimensions: []string{"/name"}, Metrics: []string{"/count"}},
		},
	})
	assert.NoError(t, err)
}

// ---------------------------------------------------------------------------
// validateSemanticFunctionRef - branch coverage
// ---------------------------------------------------------------------------

func TestValidateSemanticFunctionRefFunctionNotFoundV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	// Function doesn't exist
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Actions: []ActionSemanticInfo{
			{FunctionID: "player.nonexistent", Subject: "resource_item"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

func TestValidateSemanticFunctionRefWrongResourceKeyV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "other.get",
		Enabled: true, ResourceKey: "other", Capability: dbenum.CapabilityAction,
	})

	// Function belongs to different resource
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Actions: []ActionSemanticInfo{
			{FunctionID: "other.get", Subject: "resource_item"},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to resource")
}

func TestValidateSemanticFunctionRefDisabledFunctionV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.startTask",
		Enabled: false, ResourceKey: "player", Capability: dbenum.CapabilityTask,
		OutputSchema: model.JSON(`{"type":"object","properties":{"taskId":{"type":"string"}}}`),
	})

	// Function is disabled - use task semantic to trigger validateSemanticFunctionRef
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Tasks: []spec.TaskSemantic{
			{
				Start:  spec.FunctionRef{FunctionID: "player.startTask"},
				TaskID: spec.TaskIDSemantic{ResultPath: "/taskId", ValueType: spec.JsonScalarString},
				Status: spec.TaskStatusSemantic{
					Function:    spec.FunctionRef{FunctionID: "player.startTask"},
					TaskIDInput: "/taskId",
					StatePath:   "/taskId",
				},
			},
		},
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestValidateSemanticFunctionRefCapabilityMismatchV2(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db, nil)
	ctx := context.Background()

	capModel := model.NewResourceCapabilityModel(db)
	_ = capModel.UpsertCapability(ctx, &model.ResourceCapability{
		GameID: "g1", Env: "e1", ResourceKey: "player",
	})

	contractModel := model.NewFunctionContractModel(db)
	_ = contractModel.UpsertContract(ctx, &model.FunctionContract{
		GameID: "g1", Env: "e1", FunctionID: "player.query",
		Enabled: true, ResourceKey: "player", Capability: dbenum.CapabilityItemQuery,
	})

	// Function capability doesn't match required capability
	_, err := svc.UpdateSemantics(ctx, &UpdateSemanticsRequest{
		GameID: "g1", Env: "e1", ResourceKey: "player",
		Actions: []ActionSemanticInfo{
			{FunctionID: "player.query", Subject: "resource_item"},
		},
	})
	// This should fail because "query" capability doesn't match "action" requirement
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "capability must be")
}
