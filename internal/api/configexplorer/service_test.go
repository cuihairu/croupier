package configexplorer

import (
	"testing"
)

func TestMergeMaskedConfig(t *testing.T) {
	old := `{"addr":"1.2.3.4:6379","password":"hunter2","prefix":"cfg:"}`
	// 提交值中 password 保持脱敏占位 → 沿用旧凭据；addr 更新
	updated := `{"addr":"5.6.7.8:6379","password":"******","prefix":"cfg:"}`
	merged := mergeMaskedConfig(old, updated)
	want := `{"addr":"5.6.7.8:6379","password":"hunter2","prefix":"cfg:"}`
	if merged != want {
		t.Errorf("merged = %s want %s", merged, want)
	}

	// 空 config 整体保留旧值
	if got := mergeMaskedConfig(old, "  "); got != old {
		t.Errorf("empty new = %s want old", got)
	}

	// 旧值不存在该字段时删除占位
	fresh := `{"addr":"1.1.1.1:6379","password":"******"}`
	merged = mergeMaskedConfig(`{"addr":"1.1.1.1:6379"}`, fresh)
	if merged != `{"addr":"1.1.1.1:6379"}` {
		t.Errorf("orphan placeholder = %s", merged)
	}

	// DSN 内嵌密码占位沿用
	oldDSN := `{"dsn":"user:secret@tcp(1.2.3.4)/db"}`
	newDSN := `{"dsn":"user:******@tcp(1.2.3.4)/db2"}`
	merged = mergeMaskedConfig(oldDSN, newDSN)
	wantDSN := `{"dsn":"user:secret@tcp(1.2.3.4)/db2"}`
	if merged != wantDSN {
		t.Errorf("dsn merged = %s want %s", merged, wantDSN)
	}
}

func TestFormatOf(t *testing.T) {
	cases := map[string]string{
		"a/b.json":   "json",
		"x.YML":      "yaml",
		"conf.py":    "python",
		"tbl.csv":    "csv",
		"noext":      "plaintext",
		"rule.lua":   "lua",
		"sheet.xlsx": "xlsx",
	}
	for path, want := range cases {
		if got := formatOf(path); got != want {
			t.Errorf("formatOf(%q) = %q want %q", path, got, want)
		}
	}
}
