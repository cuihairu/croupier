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

// handler 层成功路径：Install / InstallationList / UpdateConfig / Upgrade。
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

// CompatUpgrade 成功路径（扩展 id 或数字 id 均可）。
func TestExtensionGapfix_CompatUpgradeSuccess(t *testing.T) {
	env := setupExtensionEnv(t)
	env.seedCatalog(t, "demo.gapcompat", "1.0.0", nil)
	env.seedCatalogReleaseRow(t, "demo.gapcompat", "1.2.0", nil)
	id := env.install(t, "demo.gapcompat", "1.0.0")

	env.router.POST("/gapfix/compat/:id/upgrade", env.handler.CompatUpgrade)
	rec := env.do(t, http.MethodPost, fmt.Sprintf("/gapfix/compat/%d/upgrade", id), `{"releaseVersion":"1.2.0"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// Upgrade 目标版本依赖缺失扩展 → validateDependencies 错误透传。
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

// 同 provider（不同 BindingKey，经 spec.provider 归一）双绑定：第二条
// 命中 provider 分支的 detailIndex 已存在路径，操作列表合并。
// 注：capability 的 config_keys 合并路径要求同 installation 同 BindingKey
// 的两条绑定，被 (installation_id, binding_key) 唯一索引排除，不可达。
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
