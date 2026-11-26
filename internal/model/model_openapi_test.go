package model

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

// Split from model_test.go; test bodies moved verbatim.

func TestOpenAPISource_GetSetSpec(t *testing.T) {
	source := &OpenAPISource{
		SpecJSON: `{"openapi":"3.0.3","info":{"title":"Test"}}`,
	}

	spec := source.GetSpec()
	assert.Equal(t, `{"openapi":"3.0.3","info":{"title":"Test"}}`, string(spec))

	newSpec := json.RawMessage(`{"openapi":"3.0.3","info":{"title":"Updated"}}`)
	source.SetSpec(newSpec)
	assert.Equal(t, `{"openapi":"3.0.3","info":{"title":"Updated"}}`, source.SpecJSON)
}

func TestOpenAPISource_GetOperations(t *testing.T) {
	source := &OpenAPISource{
		OperationsJSON: `[{"operationId":"getUsers","method":"GET","path":"/users"}]`,
	}

	var ops []map[string]interface{}
	err := source.GetOperations(&ops)
	require.NoError(t, err)
	assert.Len(t, ops, 1)
	assert.Equal(t, "getUsers", ops[0]["operationId"])
}

func TestOpenAPISource_GetOperations_Empty(t *testing.T) {
	source := &OpenAPISource{
		OperationsJSON: "",
	}

	var ops []map[string]interface{}
	err := source.GetOperations(&ops)
	require.NoError(t, err)
	assert.Nil(t, ops)
}

func TestOpenAPISource_SetOperations(t *testing.T) {
	source := &OpenAPISource{}

	ops := []map[string]interface{}{
		{"operationId": "getUsers", "method": "GET", "path": "/users"},
	}
	err := source.SetOperations(ops)
	require.NoError(t, err)
	assert.NotEmpty(t, source.OperationsJSON)

	// Verify round-trip
	var decoded []map[string]interface{}
	err = source.GetOperations(&decoded)
	require.NoError(t, err)
	assert.Len(t, decoded, 1)
}

func TestOpenAPISource_GetDiagnostics(t *testing.T) {
	source := &OpenAPISource{
		DiagnosticsJSON: `[{"level":"warning","message":"unused schema"}]`,
	}

	var diags []map[string]interface{}
	err := source.GetDiagnostics(&diags)
	require.NoError(t, err)
	assert.Len(t, diags, 1)
	assert.Equal(t, "warning", diags[0]["level"])
}

func TestOpenAPISource_GetDiagnostics_Empty(t *testing.T) {
	source := &OpenAPISource{
		DiagnosticsJSON: "",
	}

	var diags []map[string]interface{}
	err := source.GetDiagnostics(&diags)
	require.NoError(t, err)
	assert.Nil(t, diags)
}

func TestOpenAPISource_SetDiagnostics(t *testing.T) {
	source := &OpenAPISource{}

	diags := []map[string]interface{}{
		{"level": "error", "message": "invalid spec"},
	}
	err := source.SetDiagnostics(diags)
	require.NoError(t, err)
	assert.NotEmpty(t, source.DiagnosticsJSON)

	// Verify round-trip
	var decoded []map[string]interface{}
	err = source.GetDiagnostics(&decoded)
	require.NoError(t, err)
	assert.Len(t, decoded, 1)
}

func TestOpenAPISource_TableName(t *testing.T) {
	source := &OpenAPISource{}
	assert.Equal(t, "openapi_sources", source.TableName())
}

func TestOpenAPISourceBinding_TableName(t *testing.T) {
	binding := &OpenAPISourceBinding{}
	assert.Equal(t, "openapi_source_bindings", binding.TableName())
}

func TestNewOpenAPISourceModel(t *testing.T) {
	db := setupTestDB(t)
	model := NewOpenAPISourceModel(db)
	assert.NotNil(t, model)
}

func TestNewOpenAPISourceBindingModel(t *testing.T) {
	db := setupTestDB(t)
	model := NewOpenAPISourceBindingModel(db)
	assert.NotNil(t, model)
}

// ===== GameEnvBinding Tests =====

func TestOpenAPISourceModel_CRUD(t *testing.T) {
	db := setupTestDB(t)
	err := db.AutoMigrate(&OpenAPISource{})
	require.NoError(t, err)

	model := NewOpenAPISourceModel(db)
	ctx := context.Background()

	// Create
	source := &OpenAPISource{
		GameID:         "oss-game1",
		Env:            "prod",
		SourceID:       "src-1",
		Name:           "Test API",
		Format:         "yaml",
		OpenAPIVersion: "3.0.3",
		ContentHash:    "abc123",
	}
	err = model.Create(ctx, source)
	require.NoError(t, err)

	// ListByScope
	sources, err := model.ListByScope(ctx, "oss-game1", "prod")
	require.NoError(t, err)
	assert.Len(t, sources, 1)

	// FindByScopeAndSourceID
	found, err := model.FindByScopeAndSourceID(ctx, "oss-game1", "prod", "src-1")
	require.NoError(t, err)
	assert.Equal(t, "Test API", found.Name)

	// Update
	found.Name = "Updated API"
	err = model.Update(ctx, found)
	require.NoError(t, err)

	// Verify update
	updated, err := model.FindByScopeAndSourceID(ctx, "oss-game1", "prod", "src-1")
	require.NoError(t, err)
	assert.Equal(t, "Updated API", updated.Name)
}

// ===== PageProposalModel Tests =====
