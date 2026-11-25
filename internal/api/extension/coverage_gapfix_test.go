package extension

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// handler-layer success paths: Install / InstallationList / UpdateConfig /
// Upgrade.
func TestExtensionGapfix_HandlerSuccessPaths(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.gapfix", "1.0.0", nil)
	env.seedCatalogReleaseRow(t, "demo.gapfix", "1.1.0", nil)

	// Install 成功（HTTP）
	rec := env.do(t, http.MethodPost, "/api/v1/extensions/installations",
		`{"extensionId":"demo.gapfix","releaseVersion":"1.0.0","scopeType":"global","scopeId":"global","targetType":"global"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// InstallationList 成功（HTTP，合法 query）
	rec = env.do(t, http.MethodGet, "/api/v1/extensions/installations?page=1&pageSize=10", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var listResp ExtensionInstallationListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listResp))
	require.Equal(t, int64(1), listResp.Total)
	require.Len(t, listResp.Items, 1)
	id := listResp.Items[0].ID

	// UpdateConfig 成功（HTTP）
	rec = env.do(t, http.MethodPut, fmt.Sprintf("/api/v1/extensions/installations/%d/config", id), `{"config":{"gapfix":true}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Upgrade 成功（HTTP）
	rec = env.do(t, http.MethodPost, fmt.Sprintf("/api/v1/extensions/installations/%d/upgrade", id), `{"releaseVersion":"1.1.0"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// CompatUpgrade success path (both extension id and numeric id work).

// Upgrade target version depends on a missing extension -> the
// validateDependencies error passes through.
func TestExtensionGapfix_UpgradeDependencyFailure(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.gapup", "1.0.0", nil)
	env.seedCatalogReleaseRow(t, "demo.gapup", "2.0.0", map[string]any{
		"dependencies": []any{"demo.gapmissing"},
	})
	id := env.install(t, "demo.gapup", "1.0.0")

	_, err := env.service.Upgrade(env.ctx, id, "2.0.0", "tester")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing dependency extension")
}

// Two bindings for the same provider (different BindingKey, normalized via
// spec.provider): the second hits the provider branch where detailIndex
// already exists, so the operation lists merge.
// Note: the capability config_keys merge path requires two bindings with the
// same installation and the same BindingKey, which the (installation_id,
// binding_key) unique index rules out -- unreachable.
func TestExtensionGapfix_ProviderBindingMerge(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.gapprov", "1.0.0", nil)
	id := env.install(t, "demo.gapprov", "1.0.0")

	require.NoError(t, env.db.Create(&model.ExtensionRuntimeBinding{
		InstallationID: id, BindingType: "provider", BindingKey: "primary",
		SpecJSON: model.JSON([]byte(`{"provider":"github","operations":["listRepos"]}`)),
	}).Error)
	require.NoError(t, env.db.Create(&model.ExtensionRuntimeBinding{
		InstallationID: id, BindingType: "provider", BindingKey: "secondary",
		SpecJSON: model.JSON([]byte(`{"provider":"github","operations":["getUser"]}`)),
	}).Error)

	caps, err := env.service.Capabilities(env.ctx, id)
	require.NoError(t, err)
	var detail *ExtensionCapabilityDetail
	for i := range caps.Details {
		if caps.Details[i].Capability == "external.github" {
			detail = &caps.Details[i]
		}
	}
	require.NotNil(t, detail, "应存在 external.github 的 capability 明细")
	assert.ElementsMatch(t, []string{"listrepos", "getuser"}, detail.Operations)
}
