// Coverage targets (group E):
//   - the Installation.Upgrade failure branch of Upgrade (service.go:779-781)
//   - the Installation.List failure branch of ensureNoActiveDependents
//     (service.go:1567-1569)
//
// Also contains three behavioral regression cases (duplicate manifest
// capability, empty dependency ID): the empty-ID/duplicate-item continue
// branches of validateDependencies/extractCapabilities are in fact guaranteed
// by the upstream filtering of parseDependencies/extractCapabilities and
// cannot be reached at line-coverage level; this file only pins the behavioral
// contract that "empty dependencies and duplicate capabilities do not block
// install/display".
package extension

import (
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The manifest declares a duplicate capability: extractCapabilities already
// deduplicates, so the capability list keeps a single item; pins the display
// behavior of the manifest fallback.
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

// All UPDATEs injected to fail: the validation chain passes entirely, only
// Installation.Upgrade errors when landing in the database, covering the
// upgrade storage-error branch.
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

// The top-level dependency list contains empty IDs: after parseDependencies
// filters them, the install succeeds and no "missing dependency" is reported.
func TestExtensionGroupE_InstallSkipsEmptyDependencyID(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.groupe.emptydep", "1.0.0", map[string]any{
		"dependencies": []any{"", "   "},
	})
	id := env.install(t, "demo.groupe.emptydep", "1.0.0")
	require.Greater(t, id, uint(0))
}

// Recursion level: the depended-on extension's manifest contains an empty-ID
// dependency item, which parses to nothing, so the recursive validation passes
// directly and the install succeeds.
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

// capability branch re-entry: the unique index constrains the stored raw
// BindingKey, while capability normalizes to TrimSpace(BindingKey) -- the
// three variants "ws.cap" / " ws.cap" / "ws.cap " do not collide but normalize
// to the same capability. The second binding triggers the seen pre-fill loop
// of appendConfigKeys; the third passes a duplicate key, triggering the
// continue dedup branch.
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

// Injection fails a list query shaped exactly like ListBindings (Dest being a
// slice of installations): Get succeeds, List fails, covering the list error
// branch of ensureNoActiveDependents.
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
