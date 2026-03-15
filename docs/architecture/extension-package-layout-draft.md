# Extension Package Layout Draft

更新时间：2026-03-15
状态：草案

本文件定义扩展系统在后端代码中的包结构落地方案。

---

## 1. 目标

把前面已经完成的文档落成可编码的包结构，避免后续实现时继续在旧目录里散着加逻辑。

要求：

- 核心扩展运行时集中管理
- repo / service / api / runtime 边界清楚
- 官方扩展实现不直接混进 core 包

---

## 2. 推荐目录结构

```text
internal/
  core/
    extension/
      catalog/
      manifest/
      installation/
      runtime/
      binding/
      sync/
  api/
    extension/
  model/
    extension_*.go
  repo/
    gorm/
      extension/
  extensions/
    official/
      externalplatform/
      analytics/
```

---

## 3. 包职责

### 3.1 `internal/core/extension/catalog`

职责：

- catalog 查询
- release 查询
- catalog/release 基础校验

建议文件：

- `service.go`
- `types.go`
- `errors.go`

### 3.2 `internal/core/extension/manifest`

职责：

- manifest parse
- manifest validate
- manifest normalize

建议文件：

- `parser.go`
- `validator.go`
- `types.go`

### 3.3 `internal/core/extension/installation`

职责：

- install
- enable
- disable
- upgrade
- uninstall
- update config

建议文件：

- `service.go`
- `types.go`
- `state_machine.go`
- `errors.go`

### 3.4 `internal/core/extension/runtime`

职责：

- build runtime plan
- reconcile
- trigger binding refresh
- trigger health refresh

建议文件：

- `service.go`
- `planner.go`
- `reconcile.go`
- `types.go`

### 3.5 `internal/core/extension/binding`

职责：

- function/provider/page/workflow binding 转换
- binding 落库

建议文件：

- `service.go`
- `types.go`

### 3.6 `internal/core/extension/sync`

职责：

- 生成 Agent sync payload
- 处理 Agent report

建议文件：

- `service.go`
- `payload.go`
- `report.go`

### 3.7 `internal/api/extension`

职责：

- HTTP DTO
- handler
- 聚合调用 core extension services

建议文件：

- `dto.go`
- `handler.go`
- `service.go`

注意：

- 这里的 `service.go` 是 API 聚合服务，不是 runtime service 真正实现

### 3.8 `internal/repo/gorm/extension`

职责：

- extension 相关 repo 实现

建议文件：

- `catalog_repo.go`
- `release_repo.go`
- `installation_repo.go`
- `binding_repo.go`
- `event_repo.go`

---

## 4. 接口依赖方向

依赖方向必须保持：

`api -> core/extension -> repo/model`

官方扩展依赖方向：

`extensions/official/* -> core/extension -> repo/model`

禁止：

- `core/extension` 反向依赖具体官方扩展
- `repo` 依赖 `api`
- 官方扩展直接写 `internal/api/extension` 内部实现

---

## 5. Service 装配建议

建议在 `svc.ServiceContext` 中集中挂载：

- `ExtensionCatalogService`
- `ExtensionManifestService`
- `ExtensionInstallationService`
- `ExtensionRuntimeService`
- `ExtensionSyncService`

第一阶段可以继续挂在 `ServiceContext`，后续再考虑进一步拆出 module registry。

---

## 6. Repo 接口建议

建议先在 `internal/core/extension/*` 中定义接口，在 `internal/repo/gorm/extension` 中实现。

例如：

```go
type CatalogRepo interface {
    List(ctx context.Context, query CatalogQuery) ([]model.ExtensionCatalog, int64, error)
    GetByExtensionID(ctx context.Context, extensionID string) (*model.ExtensionCatalog, error)
    Upsert(ctx context.Context, item *model.ExtensionCatalog) error
}
```

这样可以避免 core 被 GORM 紧耦合死。

---

## 7. 官方扩展包结构

### 7.1 `internal/extensions/official/externalplatform`

建议结构：

```text
internal/extensions/official/externalplatform/
  manifest/
  schema/
  service/
  runtime/
  adapter/
```

职责：

- manifest：扩展元数据
- schema：config/capability/binding 示例
- service：配置校验、页面视图转换
- runtime：installation 转 binding plan
- adapter：兼容旧平台 runtime

### 7.2 `internal/extensions/official/analytics`

建议结构：

```text
internal/extensions/official/analytics/
  manifest/
  schema/
  service/
  pages/
  runtime/
```

---

## 8. 第一阶段落地顺序

建议按以下顺序起包：

1. `internal/model/extension_*.go`
2. `internal/repo/gorm/extension`
3. `internal/core/extension/manifest`
4. `internal/core/extension/catalog`
5. `internal/core/extension/installation`
6. `internal/core/extension/runtime`
7. `internal/core/extension/sync`
8. `internal/api/extension`

不要先起官方扩展实现包，先把 runtime 骨架打稳。

---

## 9. 当前实施建议

当前真正进入编码时，第一批文件建议是：

- `internal/model/extension_catalog.go`
- `internal/model/extension_release.go`
- `internal/model/extension_installation.go`
- `internal/model/extension_runtime_binding.go`
- `internal/model/extension_event.go`
- `internal/repo/gorm/extension/*.go`
- `internal/core/extension/manifest/*.go`
- `internal/core/extension/installation/*.go`
- `internal/api/extension/*.go`

---

## 10. 下一步

下一步需要补：

- `svc.ServiceContext` 挂载草案
- `internal/api/handler/routes.go` 扩展路由草案
- 第一批 repo method 列表
