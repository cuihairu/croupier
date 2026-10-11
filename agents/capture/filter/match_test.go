package filter

import "testing"

func mustMatcher(t *testing.T, include, exclude []string) *TableMatcher {
	t.Helper()
	m, err := NewTableMatcher(include, exclude)
	if err != nil {
		t.Fatalf("NewTableMatcher(%v, %v): %v", include, exclude, err)
	}
	return m
}

func TestTableMatcherDefaultDeny(t *testing.T) {
	// 默认拒绝：include 空 = 不授权任何表（契约红线，简档 §1.1）
	m := mustMatcher(t, nil, nil)
	if m.Authorized("game", "items") {
		t.Fatal("empty include must deny all tables")
	}
	if m.Result("game", "items") != ResultNoMatch {
		t.Fatal("empty include Result should be NoMatch")
	}
}

func TestTableMatcherExactAndWildcard(t *testing.T) {
	m := mustMatcher(t, []string{"game.items", "game.*", "ledger.main"}, nil)

	if !m.Authorized("game", "items") {
		t.Fatal("exact include should authorize")
	}
	if !m.Authorized("game", "anything") {
		t.Fatal("db wildcard should authorize")
	}
	if !m.Authorized("ledger", "main") {
		t.Fatal("second exact include should authorize")
	}
	if m.Authorized("other", "tbl") {
		t.Fatal("unlisted db should be denied")
	}
	if m.Tables()[0] == "" {
		t.Fatal("Tables should echo include keys")
	}
}

func TestTableMatcherExcludeWins(t *testing.T) {
	// 排除优先：显式 exclude 压过宽 include（运维关闸手段）
	m := mustMatcher(t, []string{"game.*"}, []string{"game.secret"})
	if m.Authorized("game", "secret") {
		t.Fatal("exact exclude must win over db wildcard include")
	}
	if !m.Authorized("game", "items") {
		t.Fatal("non-excluded table should stay authorized")
	}

	// 库级 exclude 压过一切 include（关整个库）
	m2 := mustMatcher(t, []string{"game.items", "game.*"}, []string{"game.*"})
	if m2.Authorized("game", "items") {
		t.Fatal("db-level exclude must deny whole db")
	}
}

func TestTableMatcherBuildErrors(t *testing.T) {
	// include/exclude 同项 = 构造即错（冲突发布时检测，简档 §3）
	if _, err := NewTableMatcher([]string{"game.items"}, []string{"game.items"}); err == nil {
		t.Fatal("include/exclude conflict should be rejected at build time")
	}
	// 非法表项
	for _, bad := range [][]string{
		{"items"},
		{"game.items.extra"},
		{".items"},
		{"game."},
		{""},
	} {
		if _, err := NewTableMatcher(bad, nil); err == nil {
			t.Fatalf("include %v should be rejected", bad)
		}
	}
	if _, err := NewTableMatcher(nil, []string{"no-dot"}); err == nil {
		t.Fatal("bad exclude entry should be rejected")
	}
}

func TestMissingDependencies(t *testing.T) {
	m := mustMatcher(t, []string{"game.items", "game.*"}, nil)

	// 对账双表都在范围内 = 依赖满足
	if got := m.MissingDependencies([]string{"game.items", "game.ledger"}); len(got) != 0 {
		t.Fatalf("dependencies should be satisfied, missing=%v", got)
	}
	// 流水表未授权 = 点名缺失（不为省事件量漏掉业务流水表）
	if got := m.MissingDependencies([]string{"game.items", "audit.flow"}); len(got) != 1 || got[0] != "audit.flow" {
		t.Fatalf("missing should name audit.flow, got %v", got)
	}
	// 非法表形态视为缺失（保守）
	if got := m.MissingDependencies([]string{"no-dot"}); len(got) != 1 {
		t.Fatalf("malformed dependency should be missing, got %v", got)
	}
	// nil matcher = 全部缺失（无授权信息不得声称依赖满足）
	var nilMatcher *TableMatcher
	if got := nilMatcher.MissingDependencies([]string{"game.items"}); len(got) != 1 {
		t.Fatalf("nil matcher should report all missing, got %v", got)
	}
}

func TestResultString(t *testing.T) {
	cases := map[Result]string{
		ResultMatch:       "match",
		ResultNoMatch:     "no_match",
		ResultError:       "error",
		ResultUnsupported: "unsupported",
		Result(99):        "unknown",
	}
	for r, want := range cases {
		if r.String() != want {
			t.Fatalf("Result(%d).String = %q, want %q", r, r.String(), want)
		}
	}
}
