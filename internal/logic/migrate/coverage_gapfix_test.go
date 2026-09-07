// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package migrate

// 覆盖目标：saveMigrateHistory 的 json.MarshalIndent 失败分支。
// []MigrationResult 全部字段均为 string，序列化在真实输入下恒成功；
// 通过 marshalIndent 缝隙注入故障，断言错误原样向上传播且不落盘。

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveMigrateHistoryMarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "history.json")

	injected := errors.New("injected marshal failure")
	orig := marshalIndent
	marshalIndent = func(v interface{}, prefix, indent string) ([]byte, error) {
		return nil, injected
	}
	t.Cleanup(func() { marshalIndent = orig })

	err := saveMigrateHistory(path, []MigrationResult{{Name: "m1", Status: "success"}})
	if !errors.Is(err, injected) {
		t.Fatalf("saveMigrateHistory() error = %v, want injected marshal failure", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("saveMigrateHistory() must not write file on marshal failure, stat err = %v", statErr)
	}
}
