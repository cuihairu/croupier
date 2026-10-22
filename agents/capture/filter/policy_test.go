package filter

import "testing"

func TestPolicyHeaderValidate(t *testing.T) {
	base := PolicyHeader{
		PolicyID: "game-item-monitor",
		Kind:     KindDetection,
		GameID:   "demo",
		Env:      "prod",
		Revision: 12,
		State:    StatePublished,
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("base header should validate: %v", err)
	}

	for _, kind := range []PolicyKind{KindSource, KindEvent, KindDetection, KindDelivery} {
		h := base
		h.Kind = kind
		if err := h.Validate(); err != nil {
			t.Fatalf("kind %q should be valid: %v", kind, err)
		}
	}
	for _, mutate := range []func(*PolicyHeader){
		func(h *PolicyHeader) { h.PolicyID = "" },
		func(h *PolicyHeader) { h.Kind = "magic" },
		func(h *PolicyHeader) { h.GameID = "" },
		func(h *PolicyHeader) { h.Env = "" },
		func(h *PolicyHeader) { h.Revision = 0 },
		func(h *PolicyHeader) { h.Revision = -1 },
		func(h *PolicyHeader) { h.State = "zombie" },
	} {
		h := base
		mutate(&h)
		if err := h.Validate(); err == nil {
			t.Fatalf("header %+v should be rejected", h)
		}
	}
}

func TestPolicyHeaderNewerThan(t *testing.T) {
	h := PolicyHeader{Revision: 12}
	// 严格新于才推进
	if !h.NewerThan(11) {
		t.Fatal("12 should be newer than 11")
	}
	// 同 revision = 同一份策略（幂等 no-op）
	if h.NewerThan(12) {
		t.Fatal("12 should not be newer than 12")
	}
	// 回退旧 revision 拒绝——回滚走「历史内容再发一版」，不认指针回拨
	if h.NewerThan(13) {
		t.Fatal("12 should not be newer than 13")
	}
}

// 生命周期闭集枚举（retired 只读留档不再下发）与未知态拒绝。
func TestPolicyStateClosedSet(t *testing.T) {
	for _, s := range []PolicyState{StateDraft, StatePublished, StateRetired} {
		if !s.Valid() {
			t.Fatalf("state %q should be valid", s)
		}
	}
	for _, bad := range []PolicyState{"zombie", "", "PUBLISHED"} {
		if bad.Valid() {
			t.Fatalf("state %q should be invalid", bad)
		}
	}
	for _, bad := range []PolicyKind{"", "magic", "SOURCE"} {
		if bad.Valid() {
			t.Fatalf("kind %q should be invalid", bad)
		}
	}
}
