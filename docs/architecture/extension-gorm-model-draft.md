# Extension GORM Model Draft

更新时间：2026-03-15
状态：草案

本文件定义扩展系统第一阶段的 GORM model 草案，用于指导后端代码落地。

---

## 1. 设计目标

目标：

- 与现有 `internal/model` 风格一致
- 支撑 catalog / release / installation / binding / event 的最小闭环
- 为后续 capability / health / secret binding 留扩展空间

第一阶段建议优先实现：

- `ExtensionCatalog`
- `ExtensionRelease`
- `ExtensionInstallation`
- `ExtensionRuntimeBinding`
- `ExtensionEvent`

---

## 2. 文件建议

建议新增文件：

- `internal/model/extension_catalog.go`
- `internal/model/extension_release.go`
- `internal/model/extension_installation.go`
- `internal/model/extension_runtime_binding.go`
- `internal/model/extension_event.go`

第二阶段再补：

- `internal/model/extension_capability.go`
- `internal/model/extension_health.go`
- `internal/model/extension_secret_binding.go`

---

## 3. Model 草案

### 3.1 `ExtensionCatalog`

```go
type ExtensionCatalog struct {
    gorm.Model
    ExtensionID   string `gorm:"size:128;uniqueIndex;not null" json:"extension_id"`
    Name          string `gorm:"size:128;not null" json:"name"`
    DisplayName   string `gorm:"size:255;not null" json:"display_name"`
    Vendor        string `gorm:"size:128;not null" json:"vendor"`
    Kind          string `gorm:"size:32;not null;index" json:"kind"`
    Summary       string `gorm:"type:text" json:"summary"`
    IconURL       string `gorm:"size:512" json:"icon_url"`
    HomepageURL   string `gorm:"size:512" json:"homepage_url"`
    Status        string `gorm:"size:32;not null;index" json:"status"`
    LatestVersion string `gorm:"size:64" json:"latest_version"`
}

func (ExtensionCatalog) TableName() string {
    return "extension_catalogs"
}
```

### 3.2 `ExtensionRelease`

```go
type ExtensionRelease struct {
    gorm.Model
    ExtensionID     string `gorm:"size:128;not null;index:idx_extension_release_version,priority:1" json:"extension_id"`
    Version         string `gorm:"size:64;not null;index:idx_extension_release_version,priority:2" json:"version"`
    ReleaseChannel  string `gorm:"size:32;not null;index" json:"release_channel"`
    ManifestJSON    string `gorm:"type:longtext;not null" json:"manifest_json"`
    PackageRef      string `gorm:"size:512" json:"package_ref"`
    Checksum        string `gorm:"size:128" json:"checksum"`
    MinCoreVersion  string `gorm:"size:64" json:"min_core_version"`
    Changelog       string `gorm:"type:text" json:"changelog"`
    PublishedAtUnix int64  `gorm:"not null;default:0;index" json:"published_at_unix"`
}

func (ExtensionRelease) TableName() string {
    return "extension_releases"
}
```

说明：

- 现有项目里很多模型仍用字符串保存复杂 JSON，这里第一阶段可以先保持一致，后续再换 JSON 列类型策略。

### 3.3 `ExtensionInstallation`

```go
type ExtensionInstallation struct {
    gorm.Model
    InstallationKey string `gorm:"size:191;uniqueIndex;not null" json:"installation_key"`
    ExtensionID     string `gorm:"size:128;not null;index" json:"extension_id"`
    ReleaseVersion  string `gorm:"size:64;not null" json:"release_version"`
    ScopeType       string `gorm:"size:32;not null;index" json:"scope_type"`
    ScopeID         string `gorm:"size:128;not null;index" json:"scope_id"`
    TargetType      string `gorm:"size:32;not null;index" json:"target_type"`
    TargetID        string `gorm:"size:128;index" json:"target_id"`
    Status          string `gorm:"size:32;not null;index" json:"status"`
    DesiredState    string `gorm:"size:32;not null;index" json:"desired_state"`
    Enabled         bool   `gorm:"not null;default:false;index" json:"enabled"`
    ConfigJSON      string `gorm:"type:longtext" json:"config_json"`
    SecretRefsJSON  string `gorm:"type:longtext" json:"secret_refs_json"`
    LastError       string `gorm:"type:text" json:"last_error"`
    InstalledBy     string `gorm:"size:128" json:"installed_by"`
    InstalledAtUnix int64  `gorm:"not null;default:0;index" json:"installed_at_unix"`
}

func (ExtensionInstallation) TableName() string {
    return "extension_installations"
}
```

### 3.4 `ExtensionRuntimeBinding`

```go
type ExtensionRuntimeBinding struct {
    gorm.Model
    InstallationID uint   `gorm:"not null;index" json:"installation_id"`
    BindingType    string `gorm:"size:32;not null;index" json:"binding_type"`
    BindingKey     string `gorm:"size:191;not null;index" json:"binding_key"`
    TargetRef      string `gorm:"size:255" json:"target_ref"`
    SpecJSON       string `gorm:"type:longtext" json:"spec_json"`
    Status         string `gorm:"size:32;not null;index" json:"status"`
    LastError      string `gorm:"type:text" json:"last_error"`
}

func (ExtensionRuntimeBinding) TableName() string {
    return "extension_runtime_bindings"
}
```

建议唯一索引：

- `installation_id + binding_key`

### 3.5 `ExtensionEvent`

```go
type ExtensionEvent struct {
    gorm.Model
    InstallationID uint   `gorm:"not null;index" json:"installation_id"`
    EventType      string `gorm:"size:32;not null;index" json:"event_type"`
    Level          string `gorm:"size:16;not null;index" json:"level"`
    Message        string `gorm:"type:text;not null" json:"message"`
    PayloadJSON    string `gorm:"type:longtext" json:"payload_json"`
    CreatedBy      string `gorm:"size:128" json:"created_by"`
}

func (ExtensionEvent) TableName() string {
    return "extension_events"
}
```

---

## 4. 第二阶段预留模型

### 4.1 `ExtensionCapability`

```go
type ExtensionCapability struct {
    gorm.Model
    InstallationID uint   `gorm:"not null;index" json:"installation_id"`
    CapabilityKey  string `gorm:"size:128;not null;index" json:"capability_key"`
    OperationKey   string `gorm:"size:64;not null;index" json:"operation_key"`
    DisplayName    string `gorm:"size:255;not null" json:"display_name"`
    Enabled        bool   `gorm:"not null;default:true;index" json:"enabled"`
    Visible        bool   `gorm:"not null;default:true;index" json:"visible"`
    SpecJSON       string `gorm:"type:longtext" json:"spec_json"`
}
```

### 4.2 `ExtensionHealth`

```go
type ExtensionHealth struct {
    gorm.Model
    InstallationID uint   `gorm:"not null;index" json:"installation_id"`
    Status         string `gorm:"size:32;not null;index" json:"status"`
    Message        string `gorm:"type:text" json:"message"`
    DetailsJSON    string `gorm:"type:longtext" json:"details_json"`
    CheckedAtUnix  int64  `gorm:"not null;default:0;index" json:"checked_at_unix"`
}
```

### 4.3 `ExtensionSecretBinding`

```go
type ExtensionSecretBinding struct {
    gorm.Model
    InstallationID uint   `gorm:"not null;index" json:"installation_id"`
    SecretKey      string `gorm:"size:128;not null" json:"secret_key"`
    SecretRef      string `gorm:"size:255;not null" json:"secret_ref"`
    Required       bool   `gorm:"not null;default:false" json:"required"`
}
```

---

## 5. 索引建议

### 5.1 安装实例查询热点

建议重点索引：

- `extension_id`
- `scope_type + scope_id`
- `target_type + target_id`
- `status`
- `enabled`

### 5.2 绑定查询热点

建议重点索引：

- `installation_id`
- `binding_type`
- `binding_key`

### 5.3 事件查询热点

建议重点索引：

- `installation_id`
- `event_type`
- `created_at`

---

## 6. 第一阶段仓储接口建议

建议新增 repo 接口：

```go
type ExtensionCatalogRepo interface {
    Upsert(ctx context.Context, item *model.ExtensionCatalog) error
    List(ctx context.Context, query CatalogQuery) ([]model.ExtensionCatalog, error)
    GetByExtensionID(ctx context.Context, extensionID string) (*model.ExtensionCatalog, error)
}

type ExtensionReleaseRepo interface {
    Create(ctx context.Context, item *model.ExtensionRelease) error
    ListByExtensionID(ctx context.Context, extensionID string) ([]model.ExtensionRelease, error)
    GetByVersion(ctx context.Context, extensionID, version string) (*model.ExtensionRelease, error)
}

type ExtensionInstallationRepo interface {
    Create(ctx context.Context, item *model.ExtensionInstallation) error
    Update(ctx context.Context, item *model.ExtensionInstallation) error
    GetByID(ctx context.Context, id uint) (*model.ExtensionInstallation, error)
    List(ctx context.Context, query InstallationQuery) ([]model.ExtensionInstallation, error)
}

type ExtensionRuntimeBindingRepo interface {
    ReplaceForInstallation(ctx context.Context, installationID uint, bindings []model.ExtensionRuntimeBinding) error
    ListByInstallationID(ctx context.Context, installationID uint) ([]model.ExtensionRuntimeBinding, error)
}

type ExtensionEventRepo interface {
    Create(ctx context.Context, item *model.ExtensionEvent) error
    ListByInstallationID(ctx context.Context, installationID uint) ([]model.ExtensionEvent, error)
}
```

---

## 7. 第一阶段实现建议

第一阶段优先做：

1. model
2. repo
3. service
4. API DTO

不要一开始就把 migration、health、capability 全部展开。

---

## 8. 下一步

下一步需要补：

- SQL migration 草案
- API DTO 草案
- repo/gorm 目录规划
