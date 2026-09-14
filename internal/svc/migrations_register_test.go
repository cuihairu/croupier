package svc

import (
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
)

// registerSvcMigrations 的重复注册 panic 分支：init 在包加载时已注册全部
// 迁移；清空全局后重新注册恢复现场（正常路径），再次注册时第一个版本号即
// 撞重复 → SetGlobalMigrations 遇错即停、全局状态不变 → panic 分支执行。
// 测试结束时全局注册与 init 后等价，不破坏后续用例（本用例为顺序执行，
// 并行用例在 t.Parallel() 挂起阶段，二者不交错）。
func TestRegisterSvcMigrations_PanicsOnDuplicateRegistration(t *testing.T) {
	goose.ResetGlobalMigrations()
	registerSvcMigrations() // 恢复全局注册（正常路径再覆盖一次）
	assert.PanicsWithValue(t,
		"svc: register goose go migrations: go migration with version 2 already registered",
		registerSvcMigrations,
	)
}
