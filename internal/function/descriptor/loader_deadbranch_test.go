package descriptor

import (
	"os"
	"path/filepath"
	"testing"
)

// loader.go:60 的 `if desc.ID == ""` 是防御分支：probe 已要求 "id" 为非空字符串，
// 且 encoding/json 对重复 key 一律「后者生效」，两次 Unmarshal 输入相同字节，
// desc.ID 不可能为空。本测试以重复 key 文档化该语义（last-wins），
// 证明防御分支经由 encoding/json 语义不可触发。
func TestLoadAll_DuplicateIDKeyLastWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dup.json")
	if err := os.WriteFile(path, []byte(`{"id":"first","id":"second"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	descs, err := LoadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(descs) != 1 {
		t.Fatalf("want 1 descriptor, got %d", len(descs))
	}
	if descs[0].ID != "second" {
		t.Fatalf("duplicate key must be last-wins, got ID %q", descs[0].ID)
	}
}

// 同一文件再次确认：大小写不同的 key（"ID"）不会进入 probe 的精确 "id" 匹配，
// 而是提前在 probe 阶段被跳过，也不会到达 desc.ID == "" 分支。
func TestLoadAll_CaseInsensitiveKeySkippedByProbe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upper.json")
	if err := os.WriteFile(path, []byte(`{"ID":"upper"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	descs, err := LoadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(descs) != 0 {
		t.Fatalf("uppercase key should be skipped by probe, got %d", len(descs))
	}
}
