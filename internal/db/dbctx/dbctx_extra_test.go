package dbctx

import (
	"testing"
)

func TestWithDBNilContext(t *testing.T) {
	// ctx 为 nil 时不 panic，回落 Background
	got := WithDB(nil, nil) //nolint:staticcheck // 刻意传 nil Context：验证 nil 健壮性是本用例本体
	if got == nil {
		t.Fatal("WithDB(nil, nil) returned nil context")
	}
	if Resolve(got, nil) != nil {
		t.Fatal("Resolve should return the nil override")
	}
}
