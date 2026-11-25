package dbctx

import (
	"testing"
)

func TestWithDBNilContext(t *testing.T) {
	// No panic when ctx is nil; fall back to Background
	got := WithDB(nil, nil) //nolint:staticcheck // deliberately pass a nil Context: nil robustness is the very point of this test
	if got == nil {
		t.Fatal("WithDB(nil, nil) returned nil context")
	}
	if Resolve(got, nil) != nil {
		t.Fatal("Resolve should return the nil override")
	}
}
