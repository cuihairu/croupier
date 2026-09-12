// 覆盖目标（组 E）：
//   - Upgrade 的 Installation.Upgrade 失败分支（service.go:779-781）
//   - ensureNoActiveDependents 的 Installation.List 失败分支（service.go:1567-1569）
//
// 另含三组行为回归用例（manifest 重复 capability、空依赖 ID）：
// validateDependencies/extractCapabilities 的空 ID/重复项 continue 分支
// 实际由 parseDependencies/extractCapabilities 的上游过滤保证，行覆盖
// 无法触达，此处仅锁定“空依赖与重复能力不阻塞安装/展示”的行为契约。
package extension

import (
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// manifest 声明重复 capability：extractCapabilities 已去重，能力列表只保留
// 一项；锁定 manifest fallback 的展示行为。
func TestExtensionGroupE_CapabilitiesManifestDuplicate(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.groupe.cap", "1.0.0", map[string]any{
		"capabilities": []any{"cap.dup", "cap.dup", "cap.other"},
	})
	id := env.install(t, "demo.groupe.cap", "1.0.0")

	resp, err := env.service.Capabilities(env.ctx, id)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"cap.dup", "cap.other"}, resp.Capabilities)
	var sources []string
	for _, d := range resp.Details {
		sources = append(sources, d.Source)
	}
	assert.ElementsMatch(t, []string{"manifest", "manifest"}, sources)
}

// 全部 UPDATE 注入失败：校验链全过，仅 Installation.Upgrade 落库时报错，
// 覆盖 upgrade 存储错误分支。
func TestExtensionGroupE_UpgradeInstallFails(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.groupe.up", "1.0.0", nil)
	env.seedCatalogReleaseRow(t, "demo.groupe.up", "2.0.0", nil)
	id := env.install(t, "demo.groupe.up", "1.0.0")

	require.NoError(t, env.db.Callback().Update().Before("gorm:update").
		Register("groupE_fail_update", func(tx *gorm.DB) {
			_ = tx.AddError(errors.New("forced upgrade failure"))
		}))
	t.Cleanup(func() { _ = env.db.Callback().Update().Remove("groupE_fail_update") })

	resp, err := env.service.Upgrade(env.ctx, id, "2.0.0", "tester")
	require.Error(t, err)
	assert.Nil(t, resp)
}

// 顶层依赖列表包含空 ID：parseDependencies 过滤后安装成功，不报“缺失依赖”。
func TestExtensionGroupE_InstallSkipsEmptyDependencyID(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.groupe.emptydep", "1.0.0", map[string]any{
		"dependencies": []any{"", "   "},
	})
	id := env.install(t, "demo.groupe.emptydep", "1.0.0")
	require.Greater(t, id, uint(0))
}

// 递归层：被依赖扩展的 manifest 里包含空 ID 依赖项，子依赖解析后为空，
// 递归校验直接通过，安装成功。
func TestExtensionGroupE_NestedEmptyDependencySkipped(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.groupe.dep.b", "1.0.0", map[string]any{
		"dependencies": []any{""},
	})
	env.install(t, "demo.groupe.dep.b", "1.0.0")

	env.seedCatalog(t, "demo.groupe.dep.a", "1.0.0", map[string]any{
		"dependencies": []any{"demo.groupe.dep.b"},
	})
	id := env.install(t, "demo.groupe.dep.a", "1.0.0")
	require.Greater(t, id, uint(0))
}

// capability 分支重入：唯一索引按 BindingKey 存储原值约束，而 capability
// 取 TrimSpace(BindingKey) 归一——“ws.cap”/“ ws.cap”/“ws.cap ”三个变体
// 互不冲突但归一到同一 capability。第二条触发 appendConfigKeys 的 seen
// 预填充循环，第三条传入重复 key 触发 continue 去重分支。
func TestExtensionGroupE_CapabilityBindingKeyWhitespaceReentry(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.groupe.ws", "1.0.0", nil)
	id := env.install(t, "demo.groupe.ws", "1.0.0")

	for _, b := range []model.ExtensionRuntimeBinding{
		{
			InstallationID: id, BindingType: "capability", BindingKey: "ws.cap",
			SpecJSON: model.JSON([]byte(`{"operations":["op1"],"config_keys":["cfg1"]}`)),
		},
		{
			InstallationID: id, BindingType: "capability", BindingKey: " ws.cap",
			SpecJSON: model.JSON([]byte(`{"config_keys":["cfg2"]}`)),
		},
		{
			InstallationID: id, BindingType: "capability", BindingKey: "ws.cap ",
			SpecJSON: model.JSON([]byte(`{"config_keys":["cfg1"]}`)),
		},
	} {
		require.NoError(t, env.db.Create(&b).Error)
	}

	resp, err := env.service.Capabilities(env.ctx, id)
	require.NoError(t, err)

	var detail *ExtensionCapabilityDetail
	for i := range resp.Details {
		if resp.Details[i].Capability == "ws.cap" {
			detail = &resp.Details[i]
		}
	}
	require.NotNil(t, detail, "应存在 ws.cap 的 capability 明细")
	assert.ElementsMatch(t, []string{"cfg1", "cfg2"}, detail.ConfigKeys)
	assert.Contains(t, detail.Operations, "op1")
	assert.Contains(t, resp.Capabilities, "ws.cap")
}

// 精确匹配 ListBindings 同形的列表查询（Dest 为 installation 切片）注入
// 失败：Get 成功、List 失败，覆盖 ensureNoActiveDependents 的列表错误分支。
func TestExtensionGroupE_UninstallDependentsListFails(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.groupe.uninstall", "1.0.0", nil)
	id := env.install(t, "demo.groupe.uninstall", "1.0.0")

	require.NoError(t, env.db.Callback().Query().Before("gorm:query").
		Register("groupE_fail_installation_list", func(tx *gorm.DB) {
			if _, ok := tx.Statement.Dest.(*[]model.ExtensionInstallation); ok {
				_ = tx.AddError(errors.New("forced dependents list failure"))
			}
		}))
	t.Cleanup(func() { _ = env.db.Callback().Query().Remove("groupE_fail_installation_list") })

	resp, err := env.service.Uninstall(env.ctx, id, "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forced dependents list failure")
	assert.Nil(t, resp)
}
