# Extension ServiceContext Draft

更新时间：2026-03-15
状态：草案

本文件定义扩展系统第一阶段如何挂载到 `svc.ServiceContext`。

---

## 1. 目标

让扩展运行时可以像现有核心服务一样被统一构建和注入，但不要一开始就把 `ServiceContext` 进一步膨胀成无法维护的总线。

原则：

- 第一阶段允许集中挂载
- 但字段必须按扩展 runtime 领域聚合
- 不要直接把每个扩展实例服务都挂到 `ServiceContext`

---

## 2. 当前问题

当前 `ServiceContext` 已承载：

- DB
- model
- platform loader
- registry
- dispatcher
- cache
- object store
- approval store
- 权限与中间件

如果继续按旧习惯往里一个个塞，会继续失控。

所以扩展系统需要按“聚合服务”挂载，而不是“每个 service 一个字段”。

---

## 3. 第一阶段建议挂载方式

建议在 `ServiceContext` 中新增一个聚合字段：

```go
type ExtensionServices struct {
    Catalog      extensioncatalog.Service
    Manifest     extensionmanifest.Service
    Installation extensioninstallation.Service
    Runtime      extensionruntime.Service
    Sync         extensionsync.Service
}
```

然后在 `ServiceContext` 中挂：

```go
type ServiceContext struct {
    ...
    Extensions *ExtensionServices
}
```

不要第一阶段直接挂：

- `ExtensionCatalogService`
- `ExtensionManifestService`
- `ExtensionInstallationService`
- `ExtensionRuntimeService`
- `ExtensionSyncService`

分散字段太多，不利于继续收敛。

---

## 4. 包依赖建议

建议别名：

```go
import (
    extensioncatalog "github.com/cuihairu/croupier/internal/core/extension/catalog"
    extensionmanifest "github.com/cuihairu/croupier/internal/core/extension/manifest"
    extensioninstallation "github.com/cuihairu/croupier/internal/core/extension/installation"
    extensionruntime "github.com/cuihairu/croupier/internal/core/extension/runtime"
    extensionsync "github.com/cuihairu/croupier/internal/core/extension/sync"
)
```

---

## 5. 初始化顺序建议

在 `NewServiceContext` 中建议按以下顺序初始化：

1. DB
2. auto migrate
3. core model
4. repo/gorm/extension
5. manifest service
6. catalog service
7. installation service
8. runtime service
9. sync service
10. `Extensions` 聚合字段

原因：

- installation 依赖 catalog / manifest
- runtime 依赖 installation
- sync 依赖 installation / runtime

---

## 6. 伪代码草案

```go
func NewServiceContext(c config.Config, opts ...Option) *ServiceContext {
    db, err := openDatabase(c)
    if err != nil {
        panic(err)
    }

    if err := autoMigrate(db); err != nil {
        panic(err)
    }

    extRepos := extensionrepo.NewBundle(db)
    manifestSvc := extensionmanifest.NewService()
    catalogSvc := extensioncatalog.NewService(extRepos.Catalog, extRepos.Release, manifestSvc)
    installationSvc := extensioninstallation.NewService(
        extRepos.Installation,
        extRepos.Release,
        extRepos.Event,
        manifestSvc,
    )
    runtimeSvc := extensionruntime.NewService(
        extRepos.Installation,
        extRepos.Binding,
        extRepos.Event,
        manifestSvc,
    )
    syncSvc := extensionsync.NewService(
        extRepos.Installation,
        extRepos.Binding,
        extRepos.Event,
    )

    ctx := &ServiceContext{
        ...
        Extensions: &ExtensionServices{
            Catalog:      catalogSvc,
            Manifest:     manifestSvc,
            Installation: installationSvc,
            Runtime:      runtimeSvc,
            Sync:         syncSvc,
        },
    }

    return ctx
}
```

---

## 7. autoMigrate 建议

第一阶段如果继续使用 `autoMigrate(db)`，则需要把扩展 model 纳入其中。

建议新增：

- `model.ExtensionCatalog{}`
- `model.ExtensionRelease{}`
- `model.ExtensionInstallation{}`
- `model.ExtensionRuntimeBinding{}`
- `model.ExtensionEvent{}`

如果后续转为更严格 migration-first，再逐步弱化 autoMigrate 的作用。

---

## 8. 旧服务兼容建议

### 8.1 `PlatformLoader`

短期仍可保留：

- `PlatformLoader *plat.Loader`

但要在注释中明确：

- 它是旧平台 runtime 兼容层
- 后续由 `official.external-platform` 安装实例替代主路径

### 8.2 Dashboard API

第一阶段新增 `api/extension`，不要马上重写现有 `api/platform`。

---

## 9. 第一阶段落地建议

第一批代码接入时：

1. 先加 `Extensions *ExtensionServices`
2. 先初始化空实现或最小实现
3. 再接入 `api/extension`
4. 不要一开始就改动现有所有 handler

---

## 10. 下一步

下一步需要补：

- `ServiceContext` 真实字段草案
- `autoMigrate` 需增加的 model 列表
- extension repo bundle 设计
