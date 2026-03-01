package assignment

import "testing"

func TestDiffFunctions(t *testing.T) {
	added, removed := diffFunctions(
		[]string{"a", "b", "c"},
		[]string{"b", "c", "d"},
	)
	if len(added) != 1 || added[0] != "d" {
		t.Fatalf("unexpected added: %#v", added)
	}
	if len(removed) != 1 || removed[0] != "a" {
		t.Fatalf("unexpected removed: %#v", removed)
	}
}
