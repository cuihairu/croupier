package approvals

import (
	"testing"
	"time"
)

func TestMemStore_BasicFlow(t *testing.T) {
	s := NewMemStore()
	a1 := &Approval{ID: "a1", CreatedAt: time.Now().Add(-time.Hour), Actor: "u1", FunctionID: "f1", State: "pending", Mode: "invoke", GameID: "g1", Env: "dev"}
	a2 := &Approval{ID: "a2", CreatedAt: time.Now(), Actor: "u2", FunctionID: "f2", State: "pending", Mode: "start_job", GameID: "g1", Env: "dev"}
	if err := s.Create(a1); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(a2); err != nil {
		t.Fatal(err)
	}

	// list default sort desc
	items, total, err := s.List(Filter{State: "pending"}, Page{Page: 1, Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("want 2, got total=%d len=%d", total, len(items))
	}
	if items[0].ID != "a2" {
		t.Fatalf("expect a2 first (desc), got %s", items[0].ID)
	}

	// filter function
	items, total, err = s.List(Filter{FunctionID: "f1"}, Page{Page: 1, Size: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || items[0].ID != "a1" {
		t.Fatalf("filter failed: total=%d id=%s", total, items[0].ID)
	}

	// approve transition
	a, err := s.Approve("a1")
	if err != nil {
		t.Fatal(err)
	}
	if a.State != "approved" {
		t.Fatalf("approve failed: %s", a.State)
	}

	// reject only pending
	if _, err := s.Reject("a1", "no"); err == nil {
		t.Fatalf("reject on approved should fail")
	}
}
