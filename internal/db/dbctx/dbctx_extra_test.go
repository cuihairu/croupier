package dbctx

import (
	"testing"
)

func TestWithDBNilContext(t *testing.T) {
	// ctx 为 nil 时不 panic，回落 Background
	got := WithDB(nil, nil)
	if got == nil {
		t.Fatal("WithDB(nil, nil) returned nil context")
	}
	if Resolve(got, nil) != nil {
		t.Fatal("Resolve should return the nil override")
	}
}
