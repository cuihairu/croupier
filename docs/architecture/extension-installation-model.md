# Extension Installation Model

更新时间：2026-03-15
状态：草案

本文件定义扩展安装实例的核心数据模型，用于替代当前以静态 YAML 为主的配置方式。

---

## 1. 设计目标

安装实例必须能够表达：

- 安装了哪个扩展
- 使用哪个 release
- 安装到哪里
- 属于哪个业务范围
- 当前是否启用
- 当前健康状态
- 当前配置和密钥引用
- 当前绑定了哪些 capability / function / provider
- 生命周期事件与错误记录

要求：

- Server 为主数据源
- Agent 只缓存自己需要的运行时副本
- 安装实例必须可审计、可升级、可回滚

---

## 2. 核心概念

### 2.1 Catalog

商店目录项，只描述“可安装什么”。

### 2.2 Release

一个扩展的某个可安装版本。

### 2.3 Installation

某个 release 被安装到某个 scope / target 的实例。

### 2.4 Binding

安装后生成的运行时绑定，例如：

- function
- provider
- page
- workflow
- task

### 2.5 Runtime State

运行时状态，包括：

- synced
- degraded
- failed
- reconciling

---

## 3. 表结构草案

### 3.1 `extension_catalog`

用途：商店目录。

建议字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | varchar | 主键，例如 `official.analytics` |
| `name` | varchar | 短名称 |
| `display_name` | varchar | 展示名 |
| `vendor` | varchar | 发布方 |
| `kind` | varchar | `official` / `community` / `private` |
| `summary` | text | 摘要 |
| `icon_url` | varchar | 图标 |
| `homepage_url` | varchar | 首页 |
| `status` | varchar | `active` / `hidden` / `deprecated` |
| `latest_version` | varchar | 最新版本 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |

### 3.2 `extension_release`

用途：具体版本。

建议字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint | 主键 |
| `extension_id` | varchar | 关联 catalog |
| `version` | varchar | 版本号 |
| `release_channel` | varchar | `stable` / `beta` / `experimental` |
| `manifest_json` | json | 完整 manifest 快照 |
| `package_ref` | varchar | 包引用 |
| `checksum` | varchar | 校验值 |
| `min_core_version` | varchar | 最低核心版本 |
| `changelog` | text | 变更说明 |
| `published_at` | datetime | 发布时间 |

唯一约束建议：

- `extension_id + version`

### 3.3 `extension_installation`

用途：安装实例，最关键。

建议字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint | 主键 |
| `installation_key` | varchar | 业务唯一键 |
| `extension_id` | varchar | 扩展 ID |
| `release_version` | varchar | 安装版本 |
| `scope_type` | varchar | `global/workspace/game/env/node-group/node` |
| `scope_id` | varchar | 对应 scope ID |
| `target_type` | varchar | `server/agent/hybrid` |
| `target_id` | varchar | 节点或逻辑目标 |
| `status` | varchar | 生命周期状态 |
| `enabled` | boolean | 是否启用 |
| `desired_state` | varchar | 期望状态 |
| `config_json` | json | 配置快照 |
| `secret_refs_json` | json | 密钥引用 |
| `last_error` | text | 最近错误 |
| `installed_by` | varchar | 安装人 |
| `installed_at` | datetime | 安装时间 |
| `updated_at` | datetime | 更新时间 |

建议状态：

- `pending`
- `installing`
- `installed`
- `enabling`
- `enabled`
- `disabling`
- `disabled`
- `upgrading`
- `degraded`
- `failed`
- `uninstalling`
- `uninstalled`

### 3.4 `extension_capability`

用途：安装实例暴露的 capability。

建议字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint | 主键 |
| `installation_id` | bigint | 关联安装实例 |
| `capability_key` | varchar | 例如 `platform.management` |
| `operation_key` | varchar | 例如 `invoke` |
| `display_name` | varchar | 展示名 |
| `enabled` | boolean | 是否启用 |
| `visible` | boolean | 是否显示在 UI |
| `spec_json` | json | capability 定义快照 |

### 3.5 `extension_runtime_binding`

用途：运行时绑定结果。

建议字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint | 主键 |
| `installation_id` | bigint | 安装实例 |
| `binding_type` | varchar | `function/provider/page/workflow/task` |
| `binding_key` | varchar | 唯一键 |
| `target_ref` | varchar | 目标引用 |
| `spec_json` | json | 绑定定义 |
| `status` | varchar | `pending/active/failed/removed` |
| `last_error` | text | 最近错误 |
| `updated_at` | datetime | 更新时间 |

### 3.6 `extension_health`

用途：扩展实例健康状态。

建议字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint | 主键 |
| `installation_id` | bigint | 安装实例 |
| `status` | varchar | `healthy/degraded/unhealthy/unknown` |
| `message` | text | 状态信息 |
| `checked_at` | datetime | 检查时间 |
| `details_json` | json | 扩展细节 |

### 3.7 `extension_event`

用途：生命周期事件与审计辅助。

建议字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint | 主键 |
| `installation_id` | bigint | 安装实例 |
| `event_type` | varchar | `install/enable/disable/upgrade/reconcile/error` |
| `level` | varchar | `info/warn/error` |
| `message` | text | 说明 |
| `payload_json` | json | 事件快照 |
| `created_by` | varchar | 操作者 |
| `created_at` | datetime | 时间 |

### 3.8 `secret_binding`

用途：扩展实例对密钥的引用。

建议字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint | 主键 |
| `installation_id` | bigint | 安装实例 |
| `secret_key` | varchar | manifest 中的密钥名 |
| `secret_ref` | varchar | 核心密钥引用 |
| `required` | boolean | 是否必填 |
| `updated_at` | datetime | 更新时间 |

---

## 4. 安装流程状态机

### 4.1 安装

`pending -> installing -> installed -> enabling -> enabled`

### 4.2 停用

`enabled -> disabling -> disabled`

### 4.3 升级

`enabled -> upgrading -> reconciling -> enabled`

失败时：

- 回到 `degraded`
- 记录 `last_error`
- 生成 `extension_event`

### 4.4 卸载

`disabled -> uninstalling -> uninstalled`

---

## 5. Scope 与 Target

### 5.1 Scope

Scope 表达“业务归属”，不等于部署位置。

支持：

- `global`
- `workspace`
- `game`
- `env`
- `node-group`
- `node`

### 5.2 Target

Target 表达“运行位置”。

支持：

- `server`
- `agent`
- `hybrid`

例子：

- 一个第三方平台扩展可以 `scope=game/env`
- 但 `target=agent`

这表示它属于某个游戏环境，但实际由某个 Agent 执行。

---

## 6. 与当前 YAML 的关系

历史上存在：

- `configs/platforms.yaml`
- Agent 本地 `providers.yaml`

当前状态：

- `configs/platforms.yaml` 主入口已移除
- 安装主数据源统一为 installation + runtime binding
- 本地文件路径仅用于测试夹具，不作为生产配置入口

补充：

- 默认生产路径应为 installation + runtime binding。
- YAML 回退应通过显式迁移开关控制，避免无感回流。

不能继续作为正式主数据源。

正式主数据源必须是数据库中的 installation 模型。

---

## 7. 第一批落地建议

第一批只需要真正建这些表：

- `extension_catalog`
- `extension_release`
- `extension_installation`
- `extension_runtime_binding`
- `extension_event`

其余表可以先简化为 JSON 字段或延后。

---

## 8. 下一步

下一步需要补充：

- SQL 草案
- GORM model 草案
- installation service 接口
- Agent sync payload 草案
