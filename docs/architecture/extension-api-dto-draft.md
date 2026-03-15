# Extension API DTO Draft

更新时间：2026-03-15
状态：草案

本文件定义扩展系统第一阶段的 API DTO 草案，用于指导 `internal/api/extension` 模块的接口设计。

---

## 1. 设计目标

第一阶段 API 只覆盖：

- catalog 查询
- installation 管理
- config 更新
- runtime 状态查询
- event 查询
- 手工 reconcile

不在第一阶段展开：

- 复杂依赖管理
- 多级审批安装流程
- 灰度升级批次控制

---

## 2. 模块建议

建议新增：

```text
internal/api/extension/
  dto.go
  handler.go
  service.go
```

如果后续体量变大，再拆：

- `internal/api/extensioncatalog`
- `internal/api/extensioninstallation`

第一阶段先不要过度拆碎。

---

## 3. Catalog DTO

### 3.1 `ExtensionCatalogListRequest`

```go
type ExtensionCatalogListRequest struct {
    Keyword        string `form:"keyword" json:"keyword"`
    Kind           string `form:"kind" json:"kind"`
    Status         string `form:"status" json:"status"`
    InstalledOnly  bool   `form:"installed_only" json:"installed_only"`
    ReleaseChannel string `form:"release_channel" json:"release_channel"`
    Page           int    `form:"page,default=1" json:"page"`
    PageSize       int    `form:"page_size,default=20" json:"page_size"`
}
```

### 3.2 `ExtensionCatalogItem`

```go
type ExtensionCatalogItem struct {
    ID             string   `json:"id"`
    Name           string   `json:"name"`
    DisplayName    string   `json:"display_name"`
    Vendor         string   `json:"vendor"`
    Kind           string   `json:"kind"`
    Summary        string   `json:"summary"`
    IconURL        string   `json:"icon_url"`
    Status         string   `json:"status"`
    LatestVersion  string   `json:"latest_version"`
    Installed      bool     `json:"installed"`
    DefaultInstall bool     `json:"default_install"`
    Tags           []string `json:"tags"`
}
```

### 3.3 `ExtensionCatalogListResponse`

```go
type ExtensionCatalogListResponse struct {
    Code    int                    `json:"code"`
    Message string                 `json:"message"`
    Total   int64                  `json:"total"`
    Items   []ExtensionCatalogItem `json:"items"`
}
```

### 3.4 `ExtensionCatalogDetailRequest`

```go
type ExtensionCatalogDetailRequest struct {
    ExtensionID string `path:"id" json:"extension_id"`
}
```

### 3.5 `ExtensionReleaseItem`

```go
type ExtensionReleaseItem struct {
    Version        string `json:"version"`
    ReleaseChannel string `json:"release_channel"`
    MinCoreVersion string `json:"min_core_version"`
    PublishedAt    int64  `json:"published_at"`
    Changelog      string `json:"changelog"`
}
```

### 3.6 `ExtensionCatalogDetailResponse`

```go
type ExtensionCatalogDetailResponse struct {
    Code         int                    `json:"code"`
    Message      string                 `json:"message"`
    Item         *ExtensionCatalogItem  `json:"item"`
    Releases     []ExtensionReleaseItem `json:"releases"`
    Manifest     map[string]any         `json:"manifest"`
    Capabilities []string               `json:"capabilities"`
}
```

---

## 4. Installation DTO

### 4.1 `ExtensionInstallRequest`

```go
type ExtensionInstallRequest struct {
    ExtensionID    string         `json:"extension_id" binding:"required"`
    ReleaseVersion string         `json:"release_version" binding:"required"`
    ScopeType      string         `json:"scope_type" binding:"required"`
    ScopeID        string         `json:"scope_id" binding:"required"`
    TargetType     string         `json:"target_type" binding:"required"`
    TargetID       string         `json:"target_id"`
    Config         map[string]any `json:"config"`
    SecretRefs     map[string]string `json:"secret_refs"`
}
```

### 4.2 `ExtensionInstallResponse`

```go
type ExtensionInstallResponse struct {
    Code           int    `json:"code"`
    Message        string `json:"message"`
    InstallationID uint   `json:"installation_id"`
    Status         string `json:"status"`
}
```

### 4.3 `ExtensionInstallationListRequest`

```go
type ExtensionInstallationListRequest struct {
    ExtensionID string `form:"extension_id" json:"extension_id"`
    ScopeType   string `form:"scope_type" json:"scope_type"`
    ScopeID     string `form:"scope_id" json:"scope_id"`
    TargetType  string `form:"target_type" json:"target_type"`
    TargetID    string `form:"target_id" json:"target_id"`
    Status      string `form:"status" json:"status"`
    Enabled     *bool  `form:"enabled" json:"enabled"`
    Page        int    `form:"page,default=1" json:"page"`
    PageSize    int    `form:"page_size,default=20" json:"page_size"`
}
```

### 4.4 `ExtensionInstallationItem`

```go
type ExtensionInstallationItem struct {
    ID             uint   `json:"id"`
    InstallationKey string `json:"installation_key"`
    ExtensionID    string `json:"extension_id"`
    DisplayName    string `json:"display_name"`
    ReleaseVersion string `json:"release_version"`
    ScopeType      string `json:"scope_type"`
    ScopeID        string `json:"scope_id"`
    TargetType     string `json:"target_type"`
    TargetID       string `json:"target_id"`
    Status         string `json:"status"`
    DesiredState   string `json:"desired_state"`
    Enabled        bool   `json:"enabled"`
    HealthStatus   string `json:"health_status"`
    LastError      string `json:"last_error"`
    UpdatedAt      int64  `json:"updated_at"`
}
```

### 4.5 `ExtensionInstallationListResponse`

```go
type ExtensionInstallationListResponse struct {
    Code    int                         `json:"code"`
    Message string                      `json:"message"`
    Total   int64                       `json:"total"`
    Items   []ExtensionInstallationItem `json:"items"`
}
```

### 4.6 `ExtensionInstallationDetailRequest`

```go
type ExtensionInstallationDetailRequest struct {
    InstallationID uint `path:"id" json:"installation_id"`
}
```

### 4.7 `ExtensionCapabilityItem`

```go
type ExtensionCapabilityItem struct {
    CapabilityKey string `json:"capability_key"`
    OperationKey  string `json:"operation_key"`
    DisplayName   string `json:"display_name"`
    Enabled       bool   `json:"enabled"`
    Visible       bool   `json:"visible"`
}
```

### 4.8 `ExtensionBindingItem`

```go
type ExtensionBindingItem struct {
    BindingType string `json:"binding_type"`
    BindingKey  string `json:"binding_key"`
    TargetRef   string `json:"target_ref"`
    Status      string `json:"status"`
    LastError   string `json:"last_error"`
}
```

### 4.9 `ExtensionEventItem`

```go
type ExtensionEventItem struct {
    EventType string `json:"event_type"`
    Level     string `json:"level"`
    Message   string `json:"message"`
    CreatedBy string `json:"created_by"`
    CreatedAt int64  `json:"created_at"`
}
```

### 4.10 `ExtensionInstallationDetailResponse`

```go
type ExtensionInstallationDetailResponse struct {
    Code         int                       `json:"code"`
    Message      string                    `json:"message"`
    Installation *ExtensionInstallationItem `json:"installation"`
    ConfigSchema map[string]any            `json:"config_schema"`
    Config       map[string]any            `json:"config"`
    SecretRefs   map[string]string         `json:"secret_refs"`
    Capabilities []ExtensionCapabilityItem `json:"capabilities"`
    Bindings     []ExtensionBindingItem    `json:"bindings"`
    Events       []ExtensionEventItem      `json:"events"`
}
```

---

## 5. Action DTO

### 5.1 `ExtensionInstallationActionRequest`

```go
type ExtensionInstallationActionRequest struct {
    InstallationID uint `path:"id" json:"installation_id"`
}
```

### 5.2 `ExtensionActionResponse`

```go
type ExtensionActionResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Status  string `json:"status"`
}
```

### 5.3 `ExtensionUpgradeRequest`

```go
type ExtensionUpgradeRequest struct {
    InstallationID uint   `path:"id" json:"installation_id"`
    ReleaseVersion string `json:"release_version" binding:"required"`
}
```

### 5.4 `ExtensionConfigUpdateRequest`

```go
type ExtensionConfigUpdateRequest struct {
    InstallationID uint              `path:"id" json:"installation_id"`
    Config         map[string]any    `json:"config"`
    SecretRefs     map[string]string `json:"secret_refs"`
}
```

### 5.5 `ExtensionReconcileRequest`

```go
type ExtensionReconcileRequest struct {
    InstallationID uint `path:"id" json:"installation_id"`
    Force          bool `json:"force"`
}
```

### 5.6 `ExtensionReconcileResponse`

```go
type ExtensionReconcileResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Status  string `json:"status"`
    Applied int    `json:"applied"`
    Failed  int    `json:"failed"`
}
```

---

## 6. Event DTO

### 6.1 `ExtensionEventListRequest`

```go
type ExtensionEventListRequest struct {
    InstallationID uint   `path:"id" json:"installation_id"`
    EventType      string `form:"event_type" json:"event_type"`
    Level          string `form:"level" json:"level"`
    Page           int    `form:"page,default=1" json:"page"`
    PageSize       int    `form:"page_size,default=50" json:"page_size"`
}
```

### 6.2 `ExtensionEventListResponse`

```go
type ExtensionEventListResponse struct {
    Code    int                  `json:"code"`
    Message string               `json:"message"`
    Total   int64                `json:"total"`
    Items   []ExtensionEventItem `json:"items"`
}
```

---

## 7. API 路由建议

建议路由：

- `GET /api/v1/extensions/catalog`
- `GET /api/v1/extensions/catalog/:id`
- `GET /api/v1/extensions/installations`
- `POST /api/v1/extensions/install`
- `GET /api/v1/extensions/installations/:id`
- `PUT /api/v1/extensions/installations/:id/config`
- `POST /api/v1/extensions/installations/:id/enable`
- `POST /api/v1/extensions/installations/:id/disable`
- `POST /api/v1/extensions/installations/:id/upgrade`
- `POST /api/v1/extensions/installations/:id/reconcile`
- `DELETE /api/v1/extensions/installations/:id`
- `GET /api/v1/extensions/installations/:id/events`

---

## 8. 第一阶段建议

第一阶段先确保这些 DTO 落地：

- catalog list/detail
- installation install/list/detail
- enable/disable/uninstall
- reconcile

其他 DTO 可以随着实现再补。
