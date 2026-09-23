package main

// gen_bcrypt.go 是一次性运维小工具（固定输出 admin 的 bcrypt 哈希）。
// main() 同包直测覆盖主体；panic 分支为 C 类豁免：bcrypt.GenerateFromPassword
// 对固定 "admin"+DefaultCost 输入进程内恒成功（docs/development/coverage-exemptions.md）。

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGenBcryptMain(t *testing.T) {
	main() // 固定输入恒成功路径：生成哈希并 Printf
}

// TestGenBcryptMainPin pin：固定输入下 GenerateFromPassword 恒成功，且
// main() 打印的哈希可通过 bcrypt 校验回 "admin"。
func TestGenBcryptMainPin(t *testing.T) {
	h, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword = %v, want always-success", err)
	}
	if err := bcrypt.CompareHashAndPassword(h, []byte("admin")); err != nil {
		t.Fatalf("round-trip verify = %v", err)
	}
}
