package model

import (
	"encoding/json"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// TestFunctionMigration tests the migration of Function model
// to support OpenAPI 3.0.3 format.
func TestFunctionMigration(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Run migration
	if err := db.AutoMigrate(&Function{}); err != nil {
		t.Fatalf("failed to migrate Function model: %v", err)
	}

	// Verify new columns exist
	if !db.Migrator().HasColumn(&Function{}, "spec_format") {
		t.Error("spec_format column should exist")
	}
	// GORM may use different column naming, so we check the struct field instead
	if !db.Migrator().HasColumn(&Function{}, "OpenAPISpec") {
		t.Error("OpenAPISpec column should exist")
	}

	// Test creating a function with OpenAPI format
	func1 := Function{
		FunctionID: "test.openapi.function",
		Name:       "Test OpenAPI Function",
		SpecFormat: "openapi3.0.3",
		OpenAPISpec: datatypes.JSONMap{
			"operationId": "test.openapi.function",
			"summary":     "Test function",
			"requestBody": map[string]interface{}{
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{
						"schema": map[string]interface{}{
							"type": "object",
						},
					},
				},
			},
			"responses": map[string]interface{}{
				"200": map[string]interface{}{
					"description": "Success",
				},
			},
		},
	}

	if err := db.Create(&func1).Error; err != nil {
		t.Fatalf("failed to create function: %v", err)
	}

	// Test retrieving the function
	var retrieved Function
	if err := db.Where("function_id = ?", "test.openapi.function").First(&retrieved).Error; err != nil {
		t.Fatalf("failed to retrieve function: %v", err)
	}

	if retrieved.SpecFormat != "openapi3.0.3" {
		t.Errorf("expected spec_format 'openapi3.0.3', got '%s'", retrieved.SpecFormat)
	}

	if retrieved.OpenAPISpec == nil {
		t.Error("openapi_spec should not be nil")
	}

	// Test legacy format function (for backward compatibility)
	func2 := Function{
		FunctionID: "test.legacy.function",
		Name:       "Test Legacy Function",
		SpecFormat: "legacy",
		Schema: datatypes.JSONMap{
			"input": map[string]interface{}{
				"type": "object",
			},
			"output": map[string]interface{}{
				"type": "object",
			},
		},
	}

	if err := db.Create(&func2).Error; err != nil {
		t.Fatalf("failed to create legacy function: %v", err)
	}

	// Verify both functions exist
	var count int64
	if err := db.Model(&Function{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count functions: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 functions, got %d", count)
	}
}

func TestAutoMigrate_BackfillsLegacyOpenAPIOperation(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Simulate an old schema variant from SQL migrations.
	if err := db.Exec(`
		CREATE TABLE functions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			function_id TEXT,
			name TEXT,
			openapi_operation TEXT,
			spec_format TEXT
		)
	`).Error; err != nil {
		t.Fatalf("failed to create legacy functions table: %v", err)
	}
	if err := db.Exec(`
		INSERT INTO functions (function_id, name, openapi_operation, spec_format)
		VALUES ('legacy.openapi', 'Legacy OpenAPI', '{"operationId":"legacy.openapi"}', '')
	`).Error; err != nil {
		t.Fatalf("failed to seed legacy row: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	var got Function
	if err := db.Where("function_id = ?", "legacy.openapi").First(&got).Error; err != nil {
		t.Fatalf("failed to query migrated row: %v", err)
	}
	if got.SpecFormat != "openapi3.0.3" {
		t.Fatalf("expected spec_format=openapi3.0.3, got %q", got.SpecFormat)
	}
	if got.OpenAPISpec == nil {
		t.Fatalf("expected open_api_spec to be backfilled")
	}
	raw, err := json.Marshal(got.OpenAPISpec)
	if err != nil {
		t.Fatalf("marshal open_api_spec: %v", err)
	}
	if string(raw) == "null" || string(raw) == "{}" {
		t.Fatalf("expected non-empty open_api_spec, got %s", string(raw))
	}
}

// TestFunctionIndexBySpecFormat tests querying functions by spec format
func TestFunctionIndexBySpecFormat(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	if err := db.AutoMigrate(&Function{}); err != nil {
		t.Fatalf("failed to migrate Function model: %v", err)
	}

	// Create test data
	functions := []Function{
		{
			FunctionID: "func1.openapi",
			SpecFormat: "openapi3.0.3",
		},
		{
			FunctionID: "func2.openapi",
			SpecFormat: "openapi3.0.3",
		},
		{
			FunctionID: "func3.legacy",
			SpecFormat: "legacy",
		},
	}

	for _, fn := range functions {
		if err := db.Create(&fn).Error; err != nil {
			t.Fatalf("failed to create function: %v", err)
		}
	}

	// Query OpenAPI format functions
	var openAPIFuncs []Function
	if err := db.Where("spec_format = ?", "openapi3.0.3").Find(&openAPIFuncs).Error; err != nil {
		t.Fatalf("failed to query OpenAPI functions: %v", err)
	}

	if len(openAPIFuncs) != 2 {
		t.Errorf("expected 2 OpenAPI functions, got %d", len(openAPIFuncs))
	}

	// Query legacy format functions
	var legacyFuncs []Function
	if err := db.Where("spec_format = ?", "legacy").Find(&legacyFuncs).Error; err != nil {
		t.Fatalf("failed to query legacy functions: %v", err)
	}

	if len(legacyFuncs) != 1 {
		t.Errorf("expected 1 legacy function, got %d", len(legacyFuncs))
	}
}
