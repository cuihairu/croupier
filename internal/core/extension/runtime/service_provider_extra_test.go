package runtime

// service.go provider 路径补测：Reconcile 的 providerBlock 错误透传臂、
// providerBlock 的 release 加载/manifest 解码/块解析三个错误臂、
// providerBindingFromInstallation 的 ConfigJSON 非法/provider 名缺失/
// 缺省 type/providerConfig 归空臂，以及 stringValue、bindingStatus、
// mustJSONValue 的纯函数分支。
//
// 说明：325 的缺省 type 臂仅当 manifest 无 provider 块时可达——
// ParseProviderBlock 对存在的 provider 块自身会把缺省 type 填为 openapi。

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	extmanifest "github.com/cuihairu/croupier/internal/core/extension/manifest"
	"github.com/cuihairu/croupier/internal/model"
	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newProviderFixtureDB 建立 external-platform 安装 + 指定 manifest/release
// 版本的 fixture，返回 (service, installation, db)。releaseVersion 为空时
// 不创建 release 行。
func newProviderFixtureDB(t *testing.T, releaseVersion, manifestJSON string) (*Service, *model.ExtensionInstallation, *gorm.DB) {
	t.Helper()
	db := setupTestDB(t)
	installation := &model.ExtensionInstallation{
		ExtensionID:    "acme.external-platform",
		ReleaseVersion: releaseVersion,
		Enabled:        true,
	}
	require.NoError(t, db.Create(installation).Error)
	if manifestJSON != "" {
		require.NoError(t, db.Create(&model.ExtensionRelease{
			ExtensionID:  installation.ExtensionID,
			Version:      releaseVersion,
			ManifestJSON: model.JSON(manifestJSON),
		}).Error)
	}
	s := NewService(
		extensiongorm.NewInstallationRepo(db),
		extensiongorm.NewReleaseRepo(db),
		extensiongorm.NewBindingRepo(db),
		extensiongorm.NewEventRepo(db),
	)
	return s, installation, db
}

// providerBlock：release 加载失败透传（297）+ Reconcile 透传（58）。
func TestProviderBlock_LoadReleaseError(t *testing.T) {
	s, installation, _ := newProviderFixtureDB(t, "9.9.9", "") // 无对应 release 行
	_, err := s.Reconcile(context.Background(), installation.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load extension release")
}

// providerBlock：manifest 非法 JSON（302）。
func TestProviderBlock_BadManifestJSON(t *testing.T) {
	s, installation, _ := newProviderFixtureDB(t, "2.0.0", "not-json")
	_, err := s.Reconcile(context.Background(), installation.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode extension release")
}

// providerBlock：provider 块形态非法（307）。
func TestProviderBlock_BadProviderBlock(t *testing.T) {
	s, installation, _ := newProviderFixtureDB(t, "2.0.0", `{"provider":"not-an-object"}`)
	_, err := s.Reconcile(context.Background(), installation.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse provider block")
}

// Reconcile：manifest 无 provider 块 → 空 block → 缺省 type openapi（325），
// provider 绑定与内置绑定并存，事件落库。
func TestReconcile_ProviderBindingDefaultType(t *testing.T) {
	db := setupTestDB(t)
	installation := &model.ExtensionInstallation{
		ExtensionID:    "acme.external-platform",
		ReleaseVersion: "2.0.0",
		Enabled:        true,
		ConfigJSON:     model.JSON(`{"provider":"stripe","config":{"api_key":"k"}}`),
	}
	require.NoError(t, db.Create(installation).Error)
	require.NoError(t, db.Create(&model.ExtensionRelease{
		ExtensionID:  installation.ExtensionID,
		Version:      "2.0.0",
		ManifestJSON: model.JSON(`{"summary":"no provider block"}`),
	}).Error)
	s := NewService(
		extensiongorm.NewInstallationRepo(db),
		extensiongorm.NewReleaseRepo(db),
		extensiongorm.NewBindingRepo(db),
		extensiongorm.NewEventRepo(db),
	)
	result, err := s.Reconcile(context.Background(), installation.ID)
	require.NoError(t, err)
	// 内置默认 1 条 + provider 1 条
	assert.Equal(t, 2, result.Applied)

	var bindings []model.ExtensionRuntimeBinding
	require.NoError(t, db.Where("installation_id = ?", installation.ID).Find(&bindings).Error)
	var provider *model.ExtensionRuntimeBinding
	for i := range bindings {
		if bindings[i].BindingType == "provider" {
			provider = &bindings[i]
			break
		}
	}
	require.NotNil(t, provider, "必须产出 provider 绑定")
	assert.Equal(t, "stripe", provider.BindingKey)
	var spec map[string]any
	require.NoError(t, json.Unmarshal(provider.SpecJSON, &spec))
	assert.Equal(t, extmanifest.DefaultProviderType, spec["type"])
	assert.Equal(t, true, spec["enabled"])
}

// providerBindingFromInstallation：ConfigJSON 非法 → nil（316）。
func TestProviderBindingFromInstallation_BadConfigJSON(t *testing.T) {
	item := &model.ExtensionInstallation{
		ExtensionID: "acme.external-platform",
		ConfigJSON:  model.JSON("not-json"),
	}
	assert.Nil(t, providerBindingFromInstallation(item, extmanifest.ProviderBlock{Type: "webhook"}))
}

// providerBindingFromInstallation：provider 名缺失/清洗为空 → nil（321）。
func TestProviderBindingFromInstallation_MissingProvider(t *testing.T) {
	item := &model.ExtensionInstallation{ExtensionID: "acme.external-platform"}
	assert.Nil(t, providerBindingFromInstallation(item, extmanifest.ProviderBlock{}))

	item.ConfigJSON = model.JSON(`{"provider":"   "}`)
	assert.Nil(t, providerBindingFromInstallation(item, extmanifest.ProviderBlock{}))
}

// providerBindingFromInstallation：config 与 secret_refs 均缺 → providerConfig
// 归 nil，SpecJSON 中 config 为 null（343）；secret_refs 覆盖进 providerConfig。
func TestProviderBindingFromInstallation_EmptyProviderConfig(t *testing.T) {
	// 无 config / 无 secret_refs → providerConfig nil
	item := &model.ExtensionInstallation{
		ExtensionID: "acme.external-platform",
		ConfigJSON:  model.JSON(`{"provider":"stripe"}`),
	}
	binding := providerBindingFromInstallation(item, extmanifest.ProviderBlock{Type: "webhook", Operations: []string{"charge"}})
	require.Len(t, binding, 1)
	var spec map[string]any
	require.NoError(t, json.Unmarshal(binding[0].SpecJSON, &spec))
	assert.Nil(t, spec["config"])
	assert.Equal(t, []any{"charge"}, spec["operations"])
	assert.Equal(t, "disabled", binding[0].Status, "Enabled=false → disabled")

	// secret_refs 存在 → providerConfig 非空（与 config 合并，secret 优先）
	item.SecretRefsJSON = model.JSON(`{"secret_key":"ref:vault"}`)
	binding = providerBindingFromInstallation(item, extmanifest.ProviderBlock{})
	require.Len(t, binding, 1)
	require.NoError(t, json.Unmarshal(binding[0].SpecJSON, &spec))
	require.Contains(t, spec["config"], "secret_key")
}

// stringValue：nil config / 键缺失 / 非字符串值三臂（364/368/373）。
func TestStringValue_Arms(t *testing.T) {
	assert.Empty(t, stringValue(nil, "provider"))
	assert.Empty(t, stringValue(map[string]any{}, "provider"))
	assert.Empty(t, stringValue(map[string]any{"provider": nil}, "provider"))
	assert.Equal(t, "42", stringValue(map[string]any{"provider": 42}, "provider"))
	assert.Equal(t, "stripe", stringValue(map[string]any{"provider": "  stripe  "}, "provider"))
}

// bindingStatus：disabled 分支（380）与 active 分支。
func TestBindingStatus_Arms(t *testing.T) {
	assert.Equal(t, "active", bindingStatus(true))
	assert.Equal(t, "disabled", bindingStatus(false))
}

// mustJSONValue：marshal 失败（NaN 不可序列化）→ 空 JSON（386）。
func TestMustJSONValue_MarshalError(t *testing.T) {
	out := mustJSONValue(map[string]any{"v": math.NaN()})
	assert.Len(t, strings.TrimSpace(string(out)), 0)
}
