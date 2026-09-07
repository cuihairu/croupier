package otp

// 覆盖目标：GenerateSecret 的 rand.Read 失败分支（crypto/rand 熵源读取错误）。
// 生产中 crypto/rand.Read 由 OS 熵源支撑，实际不可失败；这里通过 randRead
// 缝隙注入故障，断言错误原样向上传播且不返回部分编码的 secret。

import (
	"errors"
	"testing"
)

func TestGenerateSecret_RandReadError(t *testing.T) {
	injected := errors.New("injected entropy failure")
	orig := randRead
	randRead = func(b []byte) (int, error) { return 0, injected }
	t.Cleanup(func() { randRead = orig })

	secret, err := GenerateSecret()
	if !errors.Is(err, injected) {
		t.Fatalf("GenerateSecret() error = %v, want injected entropy failure", err)
	}
	if secret != "" {
		t.Fatalf("GenerateSecret() secret = %q, want empty string on entropy failure", secret)
	}
}
