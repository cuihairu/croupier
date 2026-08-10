package resourcecatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
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
