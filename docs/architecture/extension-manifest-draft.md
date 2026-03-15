# Extension Manifest Draft

更新时间：2026-03-15
状态：草案

本文件定义 Croupier 扩展系统的第一版 `manifest.yaml` 草案。

---

## 1. 设计目标

`manifest.yaml` 必须能够描述：

- 这是什么扩展
- 由谁发布
- 运行在哪里
- 依赖什么驱动
- 需要哪些配置和密钥
- 暴露哪些 capability / operation
- 提供哪些页面、函数、provider 绑定
- 如何健康检查
- 如何升级与回滚

要求：

- 宿主不需要为单个扩展再写专用安装逻辑
- 扩展必须可入库、可审计、可升级
- 扩展不直接越过核心边界

---

## 2. 顶层字段

建议字段如下：

| 字段 | 必填 | 说明 |
|---|---|---|
| `api_version` | 是 | manifest 版本，例如 `croupier.io/v1alpha1` |
| `kind` | 是 | 固定为 `Extension` |
| `metadata` | 是 | 基本元数据 |
| `spec` | 是 | 扩展定义 |

---

## 3. metadata

```yaml
metadata:
  id: official.external-platform
  name: external-platform
  display_name: External Platform
  vendor: croupier
  version: 1.0.0
  summary: Integrate third-party platforms through managed drivers
  description: |
    Adds installable third-party platform integration capabilities.
  icon: icons/external-platform.png
  homepage: https://example.invalid
  docs: README.md
  license: Apache-2.0
  tags:
    - official
    - integration
```

字段说明：

- `id`：全局唯一，建议 `vendor.name`
- `name`：短名称
- `display_name`：展示名称
- `vendor`：来源方
- `version`：当前 release 版本
- `icon`：图标资源
- `docs`：包内说明文档

---

## 4. spec

### 4.1 基础字段

```yaml
spec:
  release_channel: stable
  min_core_version: 2.0.0
  targets:
    - server
    - agent
  install_mode: scoped
  scope_types:
    - workspace
    - game
    - env
  runtime:
    driver: openapi-driver
    mode: managed
```

字段说明：

- `release_channel`：`stable` / `beta` / `experimental`
- `min_core_version`：最低核心版本
- `targets`：可安装目标，`server` / `agent` / `hybrid`
- `install_mode`：
  - `global`
  - `scoped`
  - `per-node`
- `scope_types`：
  - `global`
  - `workspace`
  - `game`
  - `env`
  - `node-group`
  - `node`
- `runtime.driver`：使用哪个驱动
- `runtime.mode`：
  - `managed`
  - `passive`
  - `discovery-only`

### 4.2 依赖

```yaml
  dependencies:
    core_capabilities:
      - function.invoke
      - secret.resolve
      - audit.write
    extensions: []
```

说明：

- 依赖核心能力，不直接依赖内部包路径
- 扩展间依赖保持显式

### 4.3 配置与密钥

```yaml
  config:
    schema: config.schema.json
    defaults:
      timeout: 10s
  secrets:
    schema: secrets.schema.json
    required:
      - api_token
```

### 4.4 权限

```yaml
  permissions:
    roles:
      - extension.external_platform.viewer
      - extension.external_platform.operator
      - extension.external_platform.admin
    operations:
      - platform.read
      - platform.invoke
      - platform.configure
```

### 4.5 Capability

```yaml
  capabilities:
    source: capabilities.yaml
```

`capabilities.yaml` 负责定义 capability / operation 结构。

### 4.6 UI

```yaml
  ui:
    navigation: navigation.yaml
    pages:
      - pages/list.page.yaml
      - pages/detail.page.yaml
    widgets: []
```

### 4.7 绑定

```yaml
  bindings:
    providers:
      - providers/platforms.provider.yaml
    functions:
      - functions/platform-call.function.yaml
    workflows: []
```

### 4.8 健康检查

```yaml
  healthcheck:
    type: http
    interval: 30s
    timeout: 5s
    path: /health
    severity_on_failure: degraded
```

允许类型：

- `internal`
- `http`
- `rpc`
- `custom-driver`

### 4.9 生命周期

```yaml
  lifecycle:
    on_install:
      - validate_config
      - reconcile_bindings
    on_enable:
      - sync_to_agents
    on_disable:
      - revoke_bindings
    on_upgrade:
      - migrate_config
      - rolling_reconcile
```

### 4.10 升级策略

```yaml
  upgrade:
    strategy: rolling
    allow_downgrade: false
    require_backup: true
```

### 4.11 可见性

```yaml
  visibility:
    marketplace: true
    default_install: false
    hidden: false
```

---

## 5. 完整示例

```yaml
api_version: croupier.io/v1alpha1
kind: Extension
metadata:
  id: official.external-platform
  name: external-platform
  display_name: External Platform
  vendor: croupier
  version: 1.0.0
  summary: Install third-party platform integrations
  description: |
    Managed platform integration package powered by runtime drivers.
  icon: icons/external-platform.png
  docs: README.md
  license: Apache-2.0
  tags:
    - official
    - integration
spec:
  release_channel: stable
  min_core_version: 2.0.0
  targets:
    - server
    - agent
  install_mode: scoped
  scope_types:
    - workspace
    - game
    - env
  runtime:
    driver: openapi-driver
    mode: managed
  dependencies:
    core_capabilities:
      - function.invoke
      - secret.resolve
      - audit.write
    extensions: []
  config:
    schema: config.schema.json
    defaults:
      timeout: 10s
  secrets:
    schema: secrets.schema.json
    required:
      - api_token
  permissions:
    roles:
      - extension.external_platform.viewer
      - extension.external_platform.operator
      - extension.external_platform.admin
    operations:
      - platform.read
      - platform.invoke
      - platform.configure
  capabilities:
    source: capabilities.yaml
  ui:
    navigation: navigation.yaml
    pages:
      - pages/list.page.yaml
      - pages/detail.page.yaml
    widgets: []
  bindings:
    providers:
      - providers/platforms.provider.yaml
    functions:
      - functions/platform-call.function.yaml
    workflows: []
  healthcheck:
    type: internal
    interval: 30s
    timeout: 5s
    severity_on_failure: degraded
  lifecycle:
    on_install:
      - validate_config
      - reconcile_bindings
    on_enable:
      - sync_to_agents
    on_disable:
      - revoke_bindings
    on_upgrade:
      - migrate_config
      - rolling_reconcile
  upgrade:
    strategy: rolling
    allow_downgrade: false
    require_backup: true
  visibility:
    marketplace: true
    default_install: false
    hidden: false
```

---

## 6. 当前约束

本版 manifest 明确不支持：

- 任意宿主内动态代码执行
- 扩展自带独立权限体系
- 直接引用核心内部包路径
- 直接修改核心数据库结构

---

## 7. 下一步

下一步需要补充：

- `capabilities.yaml` 规范
- `config.schema.json` 示例
- `providers/*.yaml` 绑定规范
- manifest JSON Schema
