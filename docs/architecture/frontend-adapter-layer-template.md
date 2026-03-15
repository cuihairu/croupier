---
title: 前端 Adapter 分层模板
icon: layer-group
order: 33
category:
  - 系统架构
tag:
  - frontend
  - adapter
---

# 前端 Adapter 分层模板（Dashboard）

更新时间：2026-03-15

## 1. 目录模板

```text
dashboard/
  src/
    services/
      api/
        extensions.ts
        platform.ts
      adapters/
        extensions.ts
        platform.ts
      contracts/
        extensions.ts
      errors/
        mapper.ts
        codes.ts
    pages/
      Extensions/
        Store/
        Installations/
```

## 2. 分层职责

- `services/api/*`：请求发送、DTO 类型。
- `services/adapters/*`：DTO -> ViewModel 映射。
- `services/contracts/*`：由 OpenAPI 生成的类型。
- `services/errors/*`：错误码与提示映射。
- `pages/*`：仅消费 ViewModel，不直接做字段兼容判断。

## 3. 文件模板

## 3.1 `services/api/extensions.ts`

- `listCatalog(params)`
- `getCatalogDetail(id)`
- `listReleases(id)`
- `listInstallations(params)`
- `install(payload)`
- `enable(id)` / `disable(id)` / `upgrade(id,payload)` / `uninstall(id)`
- `listEvents(id, params)`

## 3.2 `services/adapters/extensions.ts`

- `toCatalogCard(dto): CatalogCardVM`
- `toInstallationRow(dto): InstallationRowVM`
- `toEventRow(dto): EventRowVM`

## 3.3 `services/errors/mapper.ts`

- `mapBackendError(error): UiError`
- 识别：
  - `extension_already_installed`
  - `dependency_blocked`
  - `missing_dependency`
  - `version_mismatch`
  - `dependency_cycle`

## 4. 最小接入顺序

1. 先接 `Store` 页面（catalog + install）。
2. 再接 `Installations` 页面（list + enable/disable/upgrade/uninstall）。
3. 最后接 `Events` 与 `Health` 细节页。

## 5. 验收口径

- 页面不再直接读取后端原始字段差异。
- 后端新增非必填字段不会引发页面逻辑变更。
- 错误码展示统一由 mapper 管理。
