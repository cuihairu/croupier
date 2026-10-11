package filter

import "testing"

func TestNewEventFilterValidation(t *testing.T) {
	if _, err := NewEventFilter([]string{"insert", "bogus"}); err == nil {
		t.Fatal("unknown op should be rejected at build time")
	}
	for _, ok := range [][]string{nil, {}, {"insert"}, {"insert", "update", "delete"}} {
		if _, err := NewEventFilter(ok); err != nil {
			t.Fatalf("NewEventFilter(%v): %v", ok, err)
		}
	}
}

func TestEventFilterEvaluate(t *testing.T) {
	// 空白名单 = 全放行（直通）
	passthrough, err := NewEventFilter(nil)
	if err != nil {
		t.Fatalf("NewEventFilter: %v", err)
	}
	if passthrough.Evaluate("insert") != ResultMatch || passthrough.Evaluate("weird") != ResultMatch {
		t.Fatal("empty whitelist should pass everything through")
	}

	f := mustFilter(t, []string{"insert", "delete"})
	if got := f.Evaluate("update"); got != ResultNoMatch {
		t.Fatalf("unlisted legal op should be NoMatch, got %v", got)
	}
	if got := f.Evaluate("insert"); got != ResultMatch {
		t.Fatalf("listed op should Match, got %v", got)
	}
	// 未知操作 = Error（形态不可评估），不是 NoMatch
	if got := f.Evaluate("truncate"); got != ResultError {
		t.Fatalf("unknown op should be Error, got %v", got)
	}
}

func mustFilter(t *testing.T, ops []string) *EventFilter {
	t.Helper()
	f, err := NewEventFilter(ops)
	if err != nil {
		t.Fatalf("NewEventFilter(%v): %v", ops, err)
	}
	return f
}
