package otp

// R52 覆盖率补缺：otpauth.go 两翼
//   - :89-90 GenerateRecoveryCodes 的 recoveryRandRead 错误分支——镜像
//     coverage_gapfix_test.go 的 randRead 注入先例（recoveryRandRead 同为
//     包级缝隙变量）。本测试串行（无 t.Parallel）：包内 parallel 用例体在
//     全部串行用例结束后才恢复执行，缝隙变量替换无竞态。
//   - :139-140 RecoveryCodeMatches 的 got 侧 hex.DecodeString 失败分支——
//     dead-by-contract 登记（不写测试）：got 恒为 HashRecoveryCode 返回值
//     = hex.EncodeToString(sha256) 产物（64 个 hex 字符），DecodeString 对
//     其恒成功，任何输入都不可达（Encode→Decode 往返，同 re-Marshal 系判死）。

import (
	"errors"
	"testing"
)

func TestGenerateRecoveryCodes_RandReadError(t *testing.T) {
	injected := errors.New("injected recovery entropy failure")
	orig := recoveryRandRead
	recoveryRandRead = func(b []byte) (int, error) { return 0, injected }
	t.Cleanup(func() { recoveryRandRead = orig })

	codes, err := GenerateRecoveryCodes()
	if !errors.Is(err, injected) {
		t.Fatalf("GenerateRecoveryCodes() error = %v, want injected recovery entropy failure", err)
	}
	if len(codes) != 0 {
		t.Fatalf("GenerateRecoveryCodes() codes = %v, want empty on entropy failure", codes)
	}
}
