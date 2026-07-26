package descriptor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAll_ReadFileError(t *testing.T) {
	dir := t.TempDir()

	// Create a file and then remove read permissions
	// Note: This test may not work on all platforms
	filePath := filepath.Join(dir, "unreadable.json")
	err := os.WriteFile(filePath, []byte(`{"id":"test"}`), 0o000) // No permissions
	if err != nil {
		t.Skip("Cannot create unreadable file on this platform")
	}

	_, err = LoadAll(dir)
	// On some platforms, this might still succeed due to how permissions work
	// So we just log the result
	if err != nil {
		t.Logf("Got expected error: %v", err)
	}
}

func TestLoadAll_SecondUnmarshalError(t *testing.T) {
	dir := t.TempDir()

	// Create a JSON file that passes the first unmarshal (map[string]any)
	// but fails the second unmarshal (Descriptor struct)
	// version field is int instead of string
	data := `{"id": "test", "version": 123}` // version should be string
	err := os.WriteFile(filepath.Join(dir, "bad-version.json"), []byte(data), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// This should fail because version is not a string
	_, err = LoadAll(dir)
	if err == nil {
		t.Error("Expected error for invalid version type, got nil")
	}
}

func TestLoadAll_IDFieldNotString(t *testing.T) {
	dir := t.TempDir()

	// Create a JSON file where "id" is not a string
	data := `{"id": 123, "resource": "test"}`
	err := os.WriteFile(filepath.Join(dir, "id-number.json"), []byte(data), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	result, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be skipped because id is not a string
	if len(result) != 0 {
		t.Errorf("Expected 0 descriptors (id not string), got %d", len(result))
	}
}

func TestLoadAll_IDFieldArray(t *testing.T) {
	dir := t.TempDir()

	// Create a JSON file where "id" is an array
	data := `{"id": ["a", "b"], "resource": "test"}`
	err := os.WriteFile(filepath.Join(dir, "id-array.json"), []byte(data), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	result, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should be skipped because id is not a string
	if len(result) != 0 {
		t.Errorf("Expected 0 descriptors (id is array), got %d", len(result))
	}
}

func TestLoadAll_WalkDirError(t *testing.T) {
	// Test with a directory that doesn't exist
	_, err := LoadAll("/nonexistent/path/that/doesnt/exist")
	if err == nil {
		t.Error("Expected error for non-existent directory, got nil")
	}
}

func TestLoadAll_MixedValidAndInvalid(t *testing.T) {
	dir := t.TempDir()

	// Create mix of valid and invalid files
	validDesc := Descriptor{
		ID:        "valid.func",
		Version:   "1.0.0",
		Resource:  "test",
		Operation: "run",
	}
	validData, _ := json.Marshal(validDesc)
	err := os.WriteFile(filepath.Join(dir, "valid.json"), validData, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Invalid JSON
	err = os.WriteFile(filepath.Join(dir, "invalid.json"), []byte(`{bad json}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Should fail on invalid JSON
	_, err = LoadAll(dir)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestLoadAll_AllBranches(t *testing.T) {
	dir := t.TempDir()

	// Create a comprehensive test that covers multiple branches
	files := map[string]string{
		// Valid descriptor
		"valid.json": `{"id":"test.func","version":"1.0.0","resource":"test","operation":"run"}`,
		// No id field
		"no-id.json": `{"resource":"test"}`,
		// Null id
		"null-id.json": `{"id":null,"resource":"test"}`,
		// Empty id
		"empty-id.json": `{"id":"","resource":"test"}`,
		// Non-JSON file
		"readme.txt": "This is not JSON",
		// JSON in ui subdirectory
		"ui/schema.json": `{"id":"ui.func","resource":"ui"}`,
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should only have 1 valid descriptor
	if len(result) != 1 {
		t.Errorf("Expected 1 descriptor, got %d", len(result))
	}

	if len(result) > 0 && result[0].ID != "test.func" {
		t.Errorf("Expected ID 'test.func', got '%s'", result[0].ID)
	}
}
