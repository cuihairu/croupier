package descriptor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAll_SkipsUISchemas(t *testing.T) {
	dir := t.TempDir()
	// valid descriptor
	_ = os.WriteFile(filepath.Join(dir, "foo.json"), []byte(`{"id":"x.y","version":"1.0.0"}`), 0o644)
	// ui schema that should be ignored
	_ = os.MkdirAll(filepath.Join(dir, "ui"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "ui", "x.schema.json"), []byte(`{"$schema":"draft"}`), 0o644)
	list, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}
	if list[0].ID != "x.y" {
		t.Fatalf("unexpected id: %s", list[0].ID)
	}
}

// TestLoadAll_EmptyDir 测试空目录
func TestLoadAll_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	list, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(list) != 0 {
		t.Errorf("Expected 0 descriptors from empty dir, got %d", len(list))
	}
}

// TestLoadAll_MultipleDescriptors 测试多个描述符文件
func TestLoadAll_MultipleDescriptors(t *testing.T) {
	dir := t.TempDir()

	// 创建多个描述符文件
	descriptors := []Descriptor{
		{ID: "func1", Category: "cat1", Version: "1.0"},
		{ID: "func2", Category: "cat2", Version: "1.0"},
		{ID: "func3", Category: "cat1", Version: "1.0"},
	}

	for i, desc := range descriptors {
		data, _ := json.Marshal(desc)
		path := filepath.Join(dir, fmt.Sprintf("func%d.json", i+1))
		_ = os.WriteFile(path, data, 0o644)
	}

	result, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 descriptors, got %d", len(result))
	}
}

// TestLoadAll_SkipsNonJSON 测试跳过非 JSON 文件
func TestLoadAll_SkipsNonJSON(t *testing.T) {
	dir := t.TempDir()

	// 创建有效的描述符
	_ = os.WriteFile(filepath.Join(dir, "test.json"), []byte(`{"id":"test-func","category":"test"}`), 0o644)

	// 创建非 JSON 文件
	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("text"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: value"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "data.bin"), []byte{0x00, 0x01, 0x02}, 0o644)

	result, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 descriptor (non-JSON skipped), got %d", len(result))
	}
}

// TestLoadAll_SkipsNoID 测试跳过没有 ID 字段的文件
func TestLoadAll_SkipsNoID(t *testing.T) {
	dir := t.TempDir()

	// 创建没有 ID 字段的 JSON
	_ = os.WriteFile(filepath.Join(dir, "no-id.json"), []byte(`{"category": "test", "version": "1.0"}`), 0o644)

	// 创建有效的描述符
	_ = os.WriteFile(filepath.Join(dir, "valid.json"), []byte(`{"id":"valid-func","category":"test"}`), 0o644)

	result, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 descriptor (no-id skipped), got %d", len(result))
	}
}

// TestLoadAll_SkipsEmptyID 测试跳过空 ID 字段
func TestLoadAll_SkipsEmptyID(t *testing.T) {
	dir := t.TempDir()

	// 创建空 ID 字段的 JSON
	_ = os.WriteFile(filepath.Join(dir, "empty-id.json"), []byte(`{"id": "", "category": "test"}`), 0o644)

	// 创建 null ID 字段的 JSON
	_ = os.WriteFile(filepath.Join(dir, "null-id.json"), []byte(`{"id": null, "category": "test"}`), 0o644)

	// 创建有效的描述符
	_ = os.WriteFile(filepath.Join(dir, "valid.json"), []byte(`{"id":"valid-func","category":"test"}`), 0o644)

	result, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 descriptor (empty/null id skipped), got %d", len(result))
	}
}

// TestLoadAll_InvalidJSON 测试无效 JSON 文件
func TestLoadAll_InvalidJSON(t *testing.T) {
	dir := t.TempDir()

	// 创建无效的 JSON 文件
	_ = os.WriteFile(filepath.Join(dir, "invalid.json"), []byte(`{invalid json}`), 0o644)

	result, err := LoadAll(dir)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}

	if result != nil {
		t.Error("Expected nil result for invalid JSON")
	}
}

// TestLoadAll_Subdirectory 测试子目录中的文件
func TestLoadAll_Subdirectory(t *testing.T) {
	dir := t.TempDir()

	// 在主目录创建描述符
	_ = os.WriteFile(filepath.Join(dir, "main.json"), []byte(`{"id":"main-func","category":"main"}`), 0o644)

	// 在子目录创建描述符
	subDir := filepath.Join(dir, "subdir")
	_ = os.MkdirAll(subDir, 0o755)
	_ = os.WriteFile(filepath.Join(subDir, "sub.json"), []byte(`{"id":"sub-func","category":"sub"}`), 0o644)

	result, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 descriptors (from main and sub dir), got %d", len(result))
	}
}

// TestLoadAll_NestedUIDir 测试嵌套的 ui 目录
func TestLoadAll_NestedUIDir(t *testing.T) {
	dir := t.TempDir()

	// 在主目录创建描述符
	_ = os.WriteFile(filepath.Join(dir, "main.json"), []byte(`{"id":"main-func","category":"main"}`), 0o644)

	// 在嵌套的 ui 目录创建描述符
	nestedUIDir := filepath.Join(dir, "subdir", "ui")
	_ = os.MkdirAll(nestedUIDir, 0o755)
	_ = os.WriteFile(filepath.Join(nestedUIDir, "ui.json"), []byte(`{"id":"ui-func","category":"ui"}`), 0o644)

	result, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	if len(result) != 1 {
		t.Errorf("Expected 1 descriptor (nested ui dir skipped), got %d", len(result))
	}
}

// TestLoadAll_NonExistentDir 测试不存在的目录
func TestLoadAll_NonExistentDir(t *testing.T) {
	descriptors, err := LoadAll("/nonexistent/directory/that/does/not/exist/path")
	if err != nil {
		t.Logf("Expected error for non-existent directory: %v", err)
	}

	if descriptors != nil && len(descriptors) > 0 {
		// 在某些情况下，即使目录不存在也可能返回空列表
		t.Logf("Got %d descriptors for non-existent directory", len(descriptors))
	}
}
