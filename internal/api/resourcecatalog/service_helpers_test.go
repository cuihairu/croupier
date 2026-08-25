package resourcecatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// isJSONPointer
// ---------------------------------------------------------------------------

func TestIsJSONPointer(t *testing.T) {
	assert.True(t, isJSONPointer(""))
	assert.True(t, isJSONPointer("/a/b"))
	assert.True(t, isJSONPointer("/"))
	assert.False(t, isJSONPointer("abc"))
	assert.False(t, isJSONPointer("a/b"))
}

// ---------------------------------------------------------------------------
// jsonPointerTokens
// ---------------------------------------------------------------------------

func TestJsonPointerTokens(t *testing.T) {
	assert.Nil(t, jsonPointerTokens(""))
	assert.Equal(t, []string{"a"}, jsonPointerTokens("/a"))
	assert.Equal(t, []string{"a", "b", "c"}, jsonPointerTokens("/a/b/c"))
	assert.Equal(t, []string{"a/b"}, jsonPointerTokens("/a~1b"))
	assert.Equal(t, []string{"a~b"}, jsonPointerTokens("/a~0b"))
}

// ---------------------------------------------------------------------------
// digestRawJSON
// ---------------------------------------------------------------------------

func TestDigestRawJSON(t *testing.T) {
	assert.Equal(t, "", digestRawJSON(nil))
	assert.Equal(t, "", digestRawJSON([]byte{}))
	d1 := digestRawJSON([]byte(`{"a":1}`))
	d2 := digestRawJSON([]byte(`{"a":1}`))
	assert.Equal(t, d1, d2)
	assert.NotEmpty(t, d1)
}

// ---------------------------------------------------------------------------
// parseRawObject
// ---------------------------------------------------------------------------

func TestParseRawObject(t *testing.T) {
	assert.Nil(t, parseRawObject(nil))
	assert.Nil(t, parseRawObject(json.RawMessage(`"not object"`)))
	obj := parseRawObject(json.RawMessage(`{"a":1,"b":"x"}`))
	assert.Len(t, obj, 2)
	assert.NotNil(t, obj["a"])
	assert.NotNil(t, obj["b"])
}

// ---------------------------------------------------------------------------
// schemaStringValue
// ---------------------------------------------------------------------------

func TestSchemaStringValue(t *testing.T) {
	assert.Equal(t, "", schemaStringValue(nil))
	assert.Equal(t, "string", schemaStringValue(json.RawMessage(`"string"`)))
	assert.Equal(t, "", schemaStringValue(json.RawMessage(`123`)))
}

// ---------------------------------------------------------------------------
// activeStatus
// ---------------------------------------------------------------------------

func TestActiveStatus(t *testing.T) {
	assert.Equal(t, "active", activeStatus(true))
	assert.Equal(t, "inactive", activeStatus(false))
}

// ---------------------------------------------------------------------------
// affectedKindOrder
// ---------------------------------------------------------------------------

func TestAffectedKindOrder(t *testing.T) {
	assert.Equal(t, 0, affectedKindOrder("published"))
	assert.Equal(t, 1, affectedKindOrder("draft"))
	assert.Equal(t, 2, affectedKindOrder("proposal"))
	assert.Equal(t, 3, affectedKindOrder("unknown"))
}

// ---------------------------------------------------------------------------
// countUnresolvedConflicts
// ---------------------------------------------------------------------------

func TestCountUnresolvedConflicts(t *testing.T) {
	assert.Equal(t, 0, countUnresolvedConflicts(nil))
	assert.Equal(t, 0, countUnresolvedConflicts([]byte(`[]`)))
	assert.Equal(t, 0, countUnresolvedConflicts([]byte(`[{"field":"a","resolution":"ok"}]`)))
	assert.Equal(t, 2, countUnresolvedConflicts([]byte(`[{"field":"a"},{"field":"b"}]`)))
	assert.Equal(t, 1, countUnresolvedConflicts([]byte(`[{"field":"a","resolution":""},{"field":"b","resolution":"done"}]`)))
}

// ---------------------------------------------------------------------------
// toStringMap
// ---------------------------------------------------------------------------

func TestToStringMap(t *testing.T) {
	assert.Nil(t, toStringMap(nil))
	m := toStringMap(map[string]interface{}{"a": "1", "b": 2, "c": nil, "d": "x"})
	assert.Equal(t, "1", m["a"])
	assert.Equal(t, "x", m["d"])
	_, hasB := m["b"]
	assert.False(t, hasB) // non-string values are skipped
}

// ---------------------------------------------------------------------------
// isMissingTableErr
// ---------------------------------------------------------------------------

func TestIsMissingTableErr(t *testing.T) {
	assert.False(t, isMissingTableErr(nil))
	assert.True(t, isMissingTableErr(fmt.Errorf("no such table: foo")))
	assert.True(t, isMissingTableErr(fmt.Errorf("table does not exist")))
	assert.False(t, isMissingTableErr(fmt.Errorf("permission denied")))
}

// ---------------------------------------------------------------------------
// determineStatus
// ---------------------------------------------------------------------------

func TestDetermineStatus(t *testing.T) {
	tests := []struct {
		name      string
		contracts []*model.FunctionContract
		semantics *model.CapabilitySemantics
		want      string
	}{
		{
			name:      "no contracts",
			contracts: nil,
			semantics: nil,
			want:      "not_executable",
		},
		{
			name:      "empty contracts",
			contracts: []*model.FunctionContract{},
			semantics: nil,
			want:      "not_executable",
		},
		{
			name:      "has contracts but no semantics",
			contracts: []*model.FunctionContract{{FunctionID: "player.list"}},
			semantics: nil,
			want:      "pending",
		},
		{
			name:      "has contracts with conflicts",
			contracts: []*model.FunctionContract{{FunctionID: "player.list"}},
			semantics: &model.CapabilitySemantics{Conflicts: []byte(`[{"field":"a"}]`)},
			want:      "conflict",
		},
		{
			name:      "has contracts without conflicts",
			contracts: []*model.FunctionContract{{FunctionID: "player.list"}},
			semantics: &model.CapabilitySemantics{Conflicts: []byte(`[]`)},
			want:      "pending",
		},
		{
			name: "has query and identity contracts",
			contracts: []*model.FunctionContract{
				{FunctionID: "player.list", Capability: dbenum.CapabilityCollectionQuery},
				{FunctionID: "player.get", Capability: dbenum.CapabilityItemQuery},
			},
			semantics: &model.CapabilitySemantics{Conflicts: []byte(`[]`), CollectionQueryID: 1, IdentityField: "id"},
			want:      "identified",
		},
		{
			name: "has only query contract",
			contracts: []*model.FunctionContract{
				{FunctionID: "player.list", Capability: dbenum.CapabilityCollectionQuery},
			},
			semantics: &model.CapabilitySemantics{Conflicts: []byte(`[]`)},
			want:      "pending",
		},
		{
			name: "has only identity contract",
			contracts: []*model.FunctionContract{
				{FunctionID: "player.get", Capability: dbenum.CapabilityItemQuery},
			},
			semantics: &model.CapabilitySemantics{Conflicts: []byte(`[]`)},
			want:      "pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineStatus(tt.contracts, tt.semantics)
			assert.Equal(t, tt.want, result)
		})
	}
}

// ---------------------------------------------------------------------------
// buildFunctionInfos
// ---------------------------------------------------------------------------

func TestBuildFunctionInfos(t *testing.T) {
	tests := []struct {
		name      string
		contracts []*model.FunctionContract
		wantLen   int
	}{
		{
			name:      "nil contracts",
			contracts: nil,
			wantLen:   0,
		},
		{
			name:      "empty contracts",
			contracts: []*model.FunctionContract{},
			wantLen:   0,
		},
		{
			name: "single contract",
			contracts: []*model.FunctionContract{
				{FunctionID: "player.list", Version: "1.0", Capability: dbenum.CapabilityCollectionQuery, Execution: "sync", Risk: dbenum.RiskSafe, Enabled: true, Source: "test"},
			},
			wantLen: 1,
		},
		{
			name: "multiple contracts",
			contracts: []*model.FunctionContract{
				{FunctionID: "player.list", Capability: dbenum.CapabilityCollectionQuery},
				{FunctionID: "player.get", Capability: dbenum.CapabilityItemQuery},
			},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildFunctionInfos(tt.contracts)
			assert.Len(t, result, tt.wantLen)
		})
	}
}

// ---------------------------------------------------------------------------
// buildSemanticsInfo
// ---------------------------------------------------------------------------

func TestBuildSemanticsInfo(t *testing.T) {
	tests := []struct {
		name      string
		semantics *model.CapabilitySemantics
		wantNil   bool
	}{
		{
			name:      "nil semantics",
			semantics: nil,
			wantNil:   true,
		},
		{
			name:      "empty semantics",
			semantics: &model.CapabilitySemantics{},
			wantNil:   false,
		},
		{
			name: "semantics with identity field",
			semantics: &model.CapabilitySemantics{
				IdentityField:     "id",
				IdentityFieldType: "string",
				IdentityPath:      "/id",
				Source:            "test",
				SourceDigest:      "abc123",
			},
			wantNil: false,
		},
		{
			name: "semantics with collection query",
			semantics: &model.CapabilitySemantics{
				CollectionQueryID: 1,
				CollectionPath:    "/items",
				PageFieldName:     "page",
				PageSizeFieldName: "pageSize",
				ItemsFieldName:    "items",
				TotalFieldName:    "total",
			},
			wantNil: false,
		},
		{
			name: "semantics with actions",
			semantics: &model.CapabilitySemantics{
				Actions: []byte(`[{"functionId":"player.ban","subject":"resource_item"}]`),
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSemanticsInfo(tt.semantics)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				if tt.semantics.IdentityField != "" {
					assert.True(t, result.HasIdentity)
					assert.Equal(t, tt.semantics.IdentityField, result.IdentityField)
				}
				if tt.semantics.CollectionQueryID > 0 {
					assert.True(t, result.HasCollection)
				}
				if tt.semantics.Actions != nil {
					assert.True(t, result.HasActions)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// compactActionSemantics
// ---------------------------------------------------------------------------

func TestCompactActionSemantics(t *testing.T) {
	actions := compactActionSemantics([]ActionSemanticInfo{
		{FunctionID: "f1", Subject: "resource_item", IdentityInput: "/id"},
		{FunctionID: "", Subject: "none"},                        // skipped: empty FunctionID
		{FunctionID: "f2", Subject: "none", IdentityInput: "/x"}, // IdentityInput cleared
		{FunctionID: "f3", Subject: "  ", IdentityInput: "/y"},   // skipped: empty Subject after trim
	})
	require.Len(t, actions, 2)
	assert.Equal(t, "f1", actions[0].FunctionID)
	assert.Equal(t, "/id", actions[0].IdentityInput)
	assert.Equal(t, "f2", actions[1].FunctionID)
	assert.Equal(t, "", actions[1].IdentityInput) // cleared for "none" subject
}

// ---------------------------------------------------------------------------
// schemaHasPointer / schemaObjectHasPointer
// ---------------------------------------------------------------------------

func TestSchemaHasPointer(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"name":{"type":"string"},"addr":{"type":"object","properties":{"city":{"type":"string"}}}}}`)
	assert.True(t, schemaHasPointer(schema, ""))
	assert.True(t, schemaHasPointer(schema, "/name"))
	assert.True(t, schemaHasPointer(schema, "/addr/city"))
	assert.False(t, schemaHasPointer(schema, "/missing"))
	assert.False(t, schemaHasPointer(schema, "/addr/zip"))
	assert.True(t, schemaHasPointer(nil, "/any"))
	assert.True(t, schemaHasPointer([]byte(`bad`), "/any"))
}

// ---------------------------------------------------------------------------
// arrayItemSchemaAtPointer
// ---------------------------------------------------------------------------

func TestArrayItemSchemaAtPointer(t *testing.T) {
	schema := []byte(`{"type":"object","properties":{"data":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"}}}}}}`)
	items, ok := arrayItemSchemaAtPointer(schema, "/data")
	assert.True(t, ok)
	require.NotNil(t, items)
	// items is the items schema object (has type and properties)
	assert.NotNil(t, items["type"])
	assert.NotNil(t, items["properties"])

	_, ok = arrayItemSchemaAtPointer(schema, "/missing")
	assert.False(t, ok)

	_, ok = arrayItemSchemaAtPointer(nil, "/data")
	assert.False(t, ok)

	_, ok = arrayItemSchemaAtPointer(schema, "")
	assert.False(t, ok) // root is object, not array
}

// ---------------------------------------------------------------------------
// ListSemanticVersions
// ---------------------------------------------------------------------------

func TestService_ListSemanticVersions(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	// No semantics → empty
	resp, err := service.ListSemanticVersions(ctx, &ListSemanticVersionsRequest{
		GameID: "g", Env: "e", ResourceKey: "player",
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	// Create semantics with versions
	semModel := model.NewCapabilitySemanticsModel(db)
	err = semModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID: "g", Env: "e", ResourceKey: "player", Source: "sdk",
	})
	require.NoError(t, err)
	sem, err := semModel.FindByScopeAndResourceKey(ctx, "g", "e", "player")
	require.NoError(t, err)

	verModel := model.NewCapabilitySemanticVersionModel(db)
	err = verModel.CreateVersion(ctx, &model.CapabilitySemanticVersion{
		SemanticsID:  sem.ID,
		Version:      1,
		SourceDigest: "abc",
		ChangeReason: "initial",
		CreatedBy:    "tester",
	})
	require.NoError(t, err)

	resp, err = service.ListSemanticVersions(ctx, &ListSemanticVersionsRequest{
		GameID: "g", Env: "e", ResourceKey: "player",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, 1, resp.Items[0].Version)
	assert.Equal(t, "abc", resp.Items[0].SourceDigest)
}

func TestService_ListSemanticVersions_PreservesHistoryAfterRebuild(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)
	semModel := model.NewCapabilitySemanticsModel(db)
	versionModel := model.NewCapabilitySemanticVersionModel(db)

	sem := &model.CapabilitySemantics{GameID: "g", Env: "e", ResourceKey: "player", Source: "sdk"}
	require.NoError(t, semModel.UpsertSemantics(ctx, sem))
	require.NoError(t, versionModel.CreateVersion(ctx, &model.CapabilitySemanticVersion{
		SemanticsID: sem.ID, Version: sem.Version, SourceDigest: "before",
	}))
	require.NoError(t, semModel.DeleteByScopeAndResourceKey(ctx, "g", "e", "player"))

	rebuilt := &model.CapabilitySemantics{GameID: "g", Env: "e", ResourceKey: "player", Source: "sdk", IdentityField: "id"}
	require.NoError(t, semModel.UpsertSemantics(ctx, rebuilt))
	require.NoError(t, versionModel.CreateVersion(ctx, &model.CapabilitySemanticVersion{
		SemanticsID: rebuilt.ID, Version: rebuilt.Version, SourceDigest: "after",
	}))

	resp, err := service.ListSemanticVersions(ctx, &ListSemanticVersionsRequest{
		GameID: "g", Env: "e", ResourceKey: "player",
	})
	require.NoError(t, err)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, []int{2, 1}, []int{resp.Items[0].Version, resp.Items[1].Version})
}

// ---------------------------------------------------------------------------
// ListConflicts
// ---------------------------------------------------------------------------

func TestService_ListConflicts(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	service := NewService(db, nil)

	// No semantics → empty
	resp, err := service.ListConflicts(ctx, &ListConflictsRequest{
		GameID: "g", Env: "e", ResourceKey: "player",
	})
	require.NoError(t, err)
	assert.Empty(t, resp.Conflicts)

	// Create semantics with conflicts
	conflicts := []spec.SemanticConflict{
		{Field: "identityField", Values: map[spec.SemanticSource]json.RawMessage{
			spec.SemanticSourceSDKExplicit: json.RawMessage(`"id"`),
		}},
	}
	confJSON, _ := json.Marshal(conflicts)
	semModel := model.NewCapabilitySemanticsModel(db)
	err = semModel.UpsertSemantics(ctx, &model.CapabilitySemantics{
		GameID: "g", Env: "e", ResourceKey: "player", Source: "sdk", Conflicts: confJSON,
	})
	require.NoError(t, err)

	resp, err = service.ListConflicts(ctx, &ListConflictsRequest{
		GameID: "g", Env: "e", ResourceKey: "player",
	})
	require.NoError(t, err)
	assert.Len(t, resp.Conflicts, 1)
	assert.Equal(t, "identityField", resp.Conflicts[0].Field)
}

// ---------------------------------------------------------------------------
// rawJSONUint
// ---------------------------------------------------------------------------

func TestRawJSONUint(t *testing.T) {
	assert.Equal(t, json.RawMessage("0"), rawJSONUint(0))
	assert.Equal(t, json.RawMessage("42"), rawJSONUint(42))
	assert.Equal(t, json.RawMessage("100"), rawJSONUint(100))
}

// ---------------------------------------------------------------------------
// assignString / assignUint
// ---------------------------------------------------------------------------

func TestAssignString(t *testing.T) {
	var dst string
	err := assignString(json.RawMessage(`"hello"`), &dst)
	require.NoError(t, err)
	assert.Equal(t, "hello", dst)

	err = assignString(json.RawMessage(`"  spaced  "`), &dst)
	require.NoError(t, err)
	assert.Equal(t, "spaced", dst) // trimmed

	err = assignString(json.RawMessage(`123`), &dst)
	assert.Error(t, err)
}

func TestAssignUint(t *testing.T) {
	var dst uint
	err := assignUint(json.RawMessage(`42`), &dst)
	require.NoError(t, err)
	assert.Equal(t, uint(42), dst)

	err = assignUint(json.RawMessage(`"not a number"`), &dst)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// decodeDiagnostics
// ---------------------------------------------------------------------------

func TestDecodeDiagnostics(t *testing.T) {
	tests := []struct {
		name               string
		raw                []byte
		fallbackFunctionID string
		wantLen            int
	}{
		{"nil", nil, "", 0},
		{"empty", []byte{}, "", 0},
		{"invalid", []byte(`{invalid`), "fn1", 1},
		{"valid", []byte(`[{"code":"test","severity":"info","message":"ok"}]`), "", 1},
		{"fallback applied", []byte(`[{"code":"test","severity":"info","message":"ok"}]`), "fn1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodeDiagnostics(tt.raw, tt.fallbackFunctionID)
			assert.Len(t, result, tt.wantLen)
			if tt.wantLen == 1 && tt.fallbackFunctionID != "" && len(tt.raw) > 0 && tt.raw[0] == '[' {
				assert.Equal(t, tt.fallbackFunctionID, result[0].FunctionID)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parsePublishedPageSpec
// ---------------------------------------------------------------------------

func TestParsePublishedPageSpec(t *testing.T) {
	tests := []struct {
		name      string
		published model.PublishedPageSpec
		wantKey   string
	}{
		{
			name: "empty",
			published: model.PublishedPageSpec{
				PageKey: "test-page",
			},
			wantKey: "test-page",
		},
		{
			name: "with data",
			published: model.PublishedPageSpec{
				PageKey:  "player.manage",
				SpecJSON: `{"pageKey":"player.manage","type":"resource","title":{"zh-CN":"玩家管理"}}`,
			},
			wantKey: "player.manage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePublishedPageSpec(tt.published)
			assert.Equal(t, tt.wantKey, result.PageKey)
		})
	}
}

// ---------------------------------------------------------------------------
// parseBindingContracts
// ---------------------------------------------------------------------------

func TestParseBindingContracts(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantLen int
	}{
		{"empty", "", 0},
		{"invalid json", "{invalid", 0},
		{"valid", `[{"functionId":"player.list","usage":"query"}]`, 1},
		{"multiple", `[{"functionId":"player.list"},{"functionId":"player.get"}]`, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBindingContracts(tt.raw)
			assert.Len(t, result, tt.wantLen)
		})
	}
}

// ---------------------------------------------------------------------------
// matchesQuery
// ---------------------------------------------------------------------------

func TestMatchesQuery(t *testing.T) {
	tests := []struct {
		name  string
		item  ResourceCatalogItem
		query string
		want  bool
	}{
		{
			name:  "empty query matches all",
			item:  ResourceCatalogItem{ResourceKey: "player"},
			query: "",
			want:  true,
		},
		{
			name:  "match by key",
			item:  ResourceCatalogItem{ResourceKey: "player"},
			query: "player",
			want:  true,
		},
		{
			name:  "no match",
			item:  ResourceCatalogItem{ResourceKey: "player"},
			query: "order",
			want:  false,
		},
		{
			name:  "match by label",
			item:  ResourceCatalogItem{ResourceKey: "player", Labels: map[string]string{"zh-CN": "玩家"}},
			query: "玩家",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesQuery(tt.item, tt.query)
			assert.Equal(t, tt.want, result)
		})
	}
}

// ---------------------------------------------------------------------------
// isValidSemanticSource
// ---------------------------------------------------------------------------

func TestIsValidSemanticSource(t *testing.T) {
	tests := []struct {
		source spec.SemanticSource
		want   bool
	}{
		{spec.SemanticSourceSDKExplicit, true},
		{spec.SemanticSourceOpenAPIRest, true},
		{spec.SemanticSourcePlatformReview, true},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			result := isValidSemanticSource(tt.source)
			assert.Equal(t, tt.want, result)
		})
	}
}

// ---------------------------------------------------------------------------
// confidenceForSource
// ---------------------------------------------------------------------------

func TestConfidenceForSource(t *testing.T) {
	tests := []struct {
		source spec.SemanticSource
		want   string
	}{
		{spec.SemanticSourceSDKExplicit, "high"},
		{spec.SemanticSourcePlatformReview, "high"},
		{spec.SemanticSourceOpenAPIRest, "low"},
		{"unknown", "low"},
		{"", "low"},
	}

	for _, tt := range tests {
		t.Run(string(tt.source), func(t *testing.T) {
			result := confidenceForSource(tt.source)
			assert.Equal(t, tt.want, result)
		})
	}
}

// ---------------------------------------------------------------------------
// sha256Bytes
// ---------------------------------------------------------------------------

func TestSha256Bytes(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"data", []byte(`{"key":"value"}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sha256Bytes(tt.raw)
			// sha256 always returns a hash, even for nil/empty input
			assert.NotNil(t, result)
			if tt.raw == nil || len(tt.raw) == 0 {
				// For nil/empty, we still get a hash (of empty input)
				assert.Len(t, result, 32) // SHA256 is 32 bytes
			} else {
				assert.NotEmpty(t, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rawJSONString
// ---------------------------------------------------------------------------

func TestRawJSONString(t *testing.T) {
	assert.Equal(t, json.RawMessage(`"hello"`), rawJSONString("hello"))
	assert.Equal(t, json.RawMessage(`""`), rawJSONString(""))
}

// ---------------------------------------------------------------------------
// applySemanticFieldValue
// ---------------------------------------------------------------------------

func TestApplySemanticFieldValue(t *testing.T) {
	tests := []struct {
		name    string
		sem     *model.CapabilitySemantics
		field   string
		value   json.RawMessage
		wantErr bool
	}{
		{"identity field", &model.CapabilitySemantics{}, "identityField", json.RawMessage(`"id"`), false},
		{"identity field type", &model.CapabilitySemantics{}, "identityFieldType", json.RawMessage(`"string"`), false},
		{"identity path", &model.CapabilitySemantics{}, "identityPath", json.RawMessage(`"/id"`), false},
		{"collection query id", &model.CapabilitySemantics{}, "collectionQueryID", json.RawMessage(`1`), false},
		{"collection path", &model.CapabilitySemantics{}, "collectionPath", json.RawMessage(`"/items"`), false},
		{"page field name", &model.CapabilitySemantics{}, "pageFieldName", json.RawMessage(`"page"`), false},
		{"page size field name", &model.CapabilitySemantics{}, "pageSizeFieldName", json.RawMessage(`"pageSize"`), false},
		{"items field name", &model.CapabilitySemantics{}, "itemsFieldName", json.RawMessage(`"items"`), false},
		{"total field name", &model.CapabilitySemantics{}, "totalFieldName", json.RawMessage(`"total"`), false},
		{"item query id", &model.CapabilitySemantics{}, "itemQueryID", json.RawMessage(`2`), false},
		{"item path", &model.CapabilitySemantics{}, "itemPath", json.RawMessage(`"/item"`), false},
		{"create id", &model.CapabilitySemantics{}, "createID", json.RawMessage(`3`), false},
		{"update id", &model.CapabilitySemantics{}, "updateID", json.RawMessage(`4`), false},
		{"delete id", &model.CapabilitySemantics{}, "deleteID", json.RawMessage(`5`), false},
		{"source", &model.CapabilitySemantics{}, "source", json.RawMessage(`"sdk"`), true},
		{"source digest", &model.CapabilitySemantics{}, "sourceDigest", json.RawMessage(`"abc123"`), true},
		{"version", &model.CapabilitySemantics{}, "version", json.RawMessage(`1`), true},
		{"unsupported field", &model.CapabilitySemantics{}, "unknownField", json.RawMessage(`"value"`), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := applySemanticFieldValue(tt.sem, tt.field, tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// compactJSONPointers
// ---------------------------------------------------------------------------

func TestCompactJSONPointers(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   []string
	}{
		{"nil", nil, []string{}},
		{"empty", []string{}, []string{}},
		{"single", []string{"/a"}, []string{"/a"}},
		{"duplicates", []string{"/a", "/a", "/b"}, []string{"/a", "/b"}},
		{"invalid skipped", []string{"/a", "invalid", "/b"}, []string{"/a", "/b"}},
		{"whitespace trimmed", []string{" /a ", "/b"}, []string{"/a", "/b"}},
		{"empty string skipped", []string{"", "/a"}, []string{"/a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compactJSONPointers(tt.values)
			assert.Equal(t, tt.want, result)
		})
	}
}

// ---------------------------------------------------------------------------
// validateJSONPointerList
// ---------------------------------------------------------------------------

func TestValidateJSONPointerList(t *testing.T) {
	tests := []struct {
		name    string
		values  []string
		field   string
		wantLen int
		wantErr bool
	}{
		{"valid", []string{"/a", "/b"}, "test", 2, false},
		{"empty", nil, "test", 0, true},
		{"invalid pointer", []string{"invalid"}, "test", 0, true},
		{"empty string", []string{""}, "test", 0, true},
		{"duplicates removed", []string{"/a", "/a", "/b"}, "test", 2, false},
		{"non-pointer prefix", []string{"a/b"}, "test", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validateJSONPointerList(tt.values, tt.field)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.wantLen)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateTaskInputPointer
// ---------------------------------------------------------------------------

func TestValidateTaskInputPointer(t *testing.T) {
	tests := []struct {
		name    string
		pointer string
		wantErr bool
	}{
		{"valid pointer", "/id", false},
		{"invalid pointer", "id", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTaskInputPointer(nil, tt.pointer, "test")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// rebuildProposals
// ---------------------------------------------------------------------------
