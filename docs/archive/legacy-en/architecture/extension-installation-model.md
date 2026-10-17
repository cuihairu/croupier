# Extension Installation Model

更新时间：2026-09-28
状态：已收口（对齐实现；#46 专项）

本文件定义扩展安装实例的核心数据模型。2026-03 草案提出 8 表与 12 态状态机；
本版按实际落地收口：**5 表已建**（AutoMigrate baseline），生命周期简化为
4 状态同步流转，capability / health / secret_binding 三表按 V1 简化决策不建
（依据与回补路径见 §4/§5）。原草案的「Server 主数据源 + Agent 运行时副本 +
可审计可升级可回滚」目标全部成立且已实现。

---

## 1. 设计目标（不变）

- 安装了哪个扩展（extension + release）
- 使用哪个 release（版本可切换 → 升级/回滚同一机制）
- 安装到哪里（scope 业务归属 + target 运行位置）
- 当前是否启用（enabled + status/desiredState）
- 当前配置和密钥引用（config/secretRefs JSON）
- 当前绑定了哪些运行时对象（runtime bindings）
- 生命周期事件与错误记录（events）

要求：

- Server 为主数据源 ✅（extension 表全部在 meta 库，Server 独写）
- Agent 只缓存自己需要的运行时副本 ✅（§7 RuntimeSnapshot）
- 安装实例必须可审计、可升级、可回滚 ✅（events 全程记账；升级/回滚=版本切换）

## 2. 核心概念

| 概念          | 落地                                                                                                              |
| ------------- | ----------------------------------------------------------------------------------------------------------------- |
| Catalog       | `extension_catalogs` 行：只描述「可安装什么」。**V1 只读**（repo 无写路径），写路径=批次链（契约基线 §7.3）       |
| Release       | `extension_releases` 行：某扩展的可安装版本，`manifest_json` 为完整 manifest 快照                                 |
| Installation  | `extension_installations` 行：某 release 安装到某 scope/target 的实例                                             |
| Binding       | `extension_runtime_bindings` 行：安装产生的运行时绑定（function/provider/page/…），随安装/升级/reconcile 全量替换 |
| Runtime State | 未建独立状态——绑定 `status` 字段（active/failed/…）+ 安装 `status/enabled` 已覆盖 V1 语义                         |

## 3. 表结构（实际落地）

> GORM 模型在 `internal/model/extension_*.go`，baseline AutoMigrate
> （`internal/model/migration.go`）；表名复数、时间戳 `*AtUnix` int64、
> JSON 列为 `JSON` 类型——与平台模型约定一致。

### 3.1 `extension_catalogs`

| 列                                      | 说明                                                  |
| --------------------------------------- | ----------------------------------------------------- |
| `extension_id`                          | 业务唯一键（uniqueIndex），如 `official.notification` |
| `name` / `display_name` / `vendor`      | 展示三元组                                            |
| `kind`                                  | `official` / `community` / `private`                  |
| `summary` / `icon_url` / `homepage_url` | 展示                                                  |
| `status`                                | `active` / `hidden` / `deprecated`                    |
| `latest_version`                        | 最新版本（展示用；权威在 releases）                   |

### 3.2 `extension_releases`

| 列                                                            | 说明                                                          |
| ------------------------------------------------------------- | ------------------------------------------------------------- |
| `extension_id` + `version`                                    | 联合索引（版本存在性校验依赖）                                |
| `release_channel`                                             | `stable` / `beta` / `alpha`                                   |
| `manifest_json`                                               | 完整 manifest 快照（pages/capabilities/config schema 的来源） |
| `package_ref` / `checksum` / `min_core_version` / `changelog` | 发布物                                                        |
| `published_at_unix`                                           | 发布时间                                                      |

### 3.3 `extension_installations`

| 列                                                  | 说明                                                      |
| --------------------------------------------------- | --------------------------------------------------------- |
| `installation_key`                                  | 业务唯一键（uniqueIndex，由 extension+scope+target 派生） |
| `extension_id` / `release_version`                  | 安装内容                                                  |
| `scope_type` / `scope_id`                           | 业务归属：`global                                         | game  | env     | node-group | node`——**行级标记，不分库**（扩展表在 meta 库，不在 per-game 库；与 database-per-game 的关系见 §8） |
| `target_type` / `target_id`                         | 运行位置：`server                                         | agent | hybrid` |
| `status` / `desired_state` / `enabled`              | 生命周期（§4）                                            |
| `config_json` / `secret_refs_json`                  | 配置快照与密钥引用                                        |
| `last_error` / `installed_by` / `installed_at_unix` | 审计                                                      |

### 3.4 `extension_runtime_bindings`

| 列                                | 说明                                      |
| --------------------------------- | ----------------------------------------- |
| `installation_id` + `binding_key` | 联合唯一                                  |
| `binding_type`                    | `function/provider/page/workflow/task` 等 |
| `target_ref` / `spec_json`        | 绑定目标与定义快照                        |
| `status` / `last_error`           | 绑定级运行时状态                          |

写路径：`ReplaceForInstallation`（全量替换）——install/upgrade/reconcile 后
重建，uninstall 清空。

### 3.5 `extension_events`

`installation_id` + `event_type`（install/enable/disable/upgrade/uninstall/
reconcile/health_check/error…）+ `level` + `message` + `payload_json` +
`created_by`。所有生命周期动作经 `appendEvent` 记账，事件查询走
`GET .../installations/:id/events`。

## 4. 生命周期状态机（实际）

草案 12 态未采用（中间态无落库消费方）。实际为 **4 状态同步流转**，
`status` 与 `desired_state` 恒同步写入：

```
install    → status=installed   desired=disabled  enabled=false
enable     → status=enabled     desired=enabled   enabled=true
disable    → status=disabled    desired=disabled  enabled=false
upgrade    → release_version=新  status=enabled（见下方已知边界）
uninstall  → status=uninstalled desired=uninstalled enabled=false + bindings 清空
```

- 动作同步完成（无 pending/installing 等中间态）；失败即整动作回滚并记
  `last_error`/error 事件，不落半态。
- **已知边界**：core 层 `Upgrade` 无条件置 `status=enabled` 但不动 `enabled`
  布尔——禁用态升级后出现 `status=enabled && enabled=false` 的字段不同步。
  实施批次修复（status 应保留原值），文档如实记录不粉饰。

## 5. 升级与回滚

- **升级 = 版本切换**：校验链（target release 存在 → 依赖图校验
  `missing_dependency/dependency_cycle/version_mismatch` → 现有 config 对新
  版本 config-schema 校验）→ 换 `release_version` → bindings 待 reconcile
  重建 → `upgrade` 事件。
- **回滚 = upgrade 到旧版本**，不设独立 rollback 端点（同一校验链，语义等价
  且避免第二套状态机）。同版本重复升级返回 `409 conflict`。
- **健康检查**：V1 为轻量语义判定（enabled→healthy / disabled→disabled /
  卸载→uninstalled）+ `health_check` 事件，无探测、无落库（响应即所得）。
  草案的 `extension_health` 表回补路径：若引入真实探测（如经 agent 隧道
  ping capability），再按草案 §3.6 建表——届时走编号迁移。
- **capability / secret_binding 两表**同理暂缓：capabilities 由 bindings +
  manifest 推导（`/capabilities` 端点聚合返回 details），secrets 以
  `secret_refs_json`（引用而非明文）挂在安装行上，独立表仅在需要行级审计/
  轮换时回补。

## 6. 与 YAML 的关系（已收口）

- `configs/platforms.yaml` 主入口已移除；安装主数据源=数据库 installation 模型。
- 本地 `providers.yaml` 文件路径仅存于测试夹具；兼容配置端点
  （`internal/api/extension` Compat* 家族）属 URL 形态兼容而非数据源回退，
  随契约收口批次删除（契约基线 §3）。

## 7. Agent 运行时副本（实际机制）

```
Server                                    Agent
extensionsync.BuildAgentPayload  ←HTTP轮询←  ExtensionSyncPuller(30s)
  按 target_type/target_id 过滤              GET /api/v1/agents/:id/extensions
  → installations + bindings 视图            → ExtensionRuntime.ApplyPayload
                                             → 内存 RuntimeSnapshot
                                             （version/generatedAt/lastApply*
                                               /installations[]）
```

- 副本是**内存快照**（`internal/app/agent/extension_runtime.go`），Agent 重启
  后首拉重建，不落盘——满足「只缓存自己需要的运行时副本」。
- 同步走 HTTP 轮询（非 TCP 隧道推送）：V1 保留，间隔可配；隧道推送为演进项。
- apply 结果统计（applied/removed/failed/lastError）回显在快照与
  `/installations/:id/reconcile` 响应中，构成安装模型的可观测闭环。
- `externalfunc` 桥：`*.external` 类扩展 ID 识别为外部平台扩展，函数发现/绑定
  走独立映射——边界见 `core-extension-mapping.md`。

## 8. Scope 与多游戏隔离

- 扩展五表在 **meta 库**，路由不在 scoped 组（无 GameDBMiddleware）——扩展是
  系统级资源，跨游戏共享 catalog/release。
- `scope_type=game|env` 的安装实例用行级 `scope_id` 标记归属，**不**触发
  per-game 分库；与平台「database-per-game」不冲突：分库隔离的是业务数据
  （玩家/工单/页面），扩展安装是控制面资源（同 users/roles 一层）。
- Agent 侧 payload 按 target 过滤（`matchesAgentTarget`），一个 Agent 只见
  分派给自己的安装实例。

## 9. 第一批落地（已完成，原草案 §7 对账）

- ✅ `extension_catalogs` / `extension_releases` / `extension_installations` /
  `extension_runtime_bindings` / `extension_events` 五表
- ✅ 安装生命周期 API 全套 + 依赖校验 + 卸载保护 + 事件记账
- ✅ Agent 运行时副本 + 轮询同步 + reconcile
- ⏸ catalog/release 写路径（admin CRUD + 官方扩展 seed）——**当前主缺口**，
  批次链见契约基线 §7
- ⏸ capability / health / secret_binding 独立表——按 §5 简化决策暂缓
