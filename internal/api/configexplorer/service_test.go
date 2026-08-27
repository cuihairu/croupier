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

func TestRestoreDSNPassword(t *testing.T) {
	cases := []struct {
		name, oldDSN, newDSN, want string
	}{
		{"keeps old host tail", "user:secret@tcp(db:3306)/x", "user:******@tcp(newdb:3307)/y", "user:secret@tcp(newdb:3307)/y"},
		{"no at in old", "plain-dsn", "user:******@h/d", "plain-dsn"},
		{"no at in new", "user:secret@h/d", "plain", "user:secret@h/d"},
		{"old cred without colon", "user@h/d", "user:******@h2/d", "user@h/d"},
		{"pg url", "postgres://u:p@h:5432/db", "postgres://u:******@h2:5432/db2", "postgres://u:p@h2:5432/db2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := restoreDSNPassword(tc.oldDSN, tc.newDSN); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsTextFormat(t *testing.T) {
	text := []string{"json", "yaml", "csv", "ini", "xml", "lua", "python", "txt", "md", "toml", "properties", "plaintext"}
	for _, f := range text {
		if !isTextFormat(f) {
			t.Errorf("isTextFormat(%q) = false, want true", f)
		}
	}
	for _, f := range []string{"png", "xlsx", "bin", ""} {
		if isTextFormat(f) {
			t.Errorf("isTextFormat(%q) = true, want false", f)
		}
	}
}

func TestParseID(t *testing.T) {
	if id, err := parseID("42"); err != nil || id != 42 {
		t.Fatalf("parseID(42) = (%d, %v)", id, err)
	}
	if _, err := parseID("abc"); err == nil {
		t.Fatal("parseID(abc) should fail")
	}
	if _, err := parseID("-1"); err == nil {
		t.Fatal("parseID(-1) should fail")
	}
}
