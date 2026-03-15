# 官方扩展统一模式（权限 / 菜单 / 配置）

更新时间：2026-03-15  
状态：Draft（Phase 7 统一约束）

## 1. 目的

为 `official.alerting`、`official.notification`、`official.approval`、`official.backup-advanced` 提供统一的实现约束，避免每个扩展重复发明权限、菜单与配置模型。

## 2. 命名规则

- Extension ID：`official.<domain>`，例如 `official.notification`
- Capability Key：`<domain>.<subject>`，例如 `notifications.management`
- Operation：短动词集合（`list/get/create/update/delete/approve/reject/...`）
- Function Key：`<domain>.<operation>`，例如 `notifications.update`
- Page Key：`<domain>.overview` 或 `<domain>.<feature>`

## 3. 权限模型

每个扩展统一三层权限：

- `<domain>.read`
- `<domain>.operate`
- `<domain>.admin`

映射规则：

- `list/get/pages/capabilities` 绑定 `read`
- `create/update/delete/approve/reject/test` 绑定 `operate`
- `enable/disable/upgrade/config/secrets` 绑定 `admin`

要求：

- API 层只校验 capability/operation 对应权限，不写散乱业务权限字符串。
- 扩展 manifest/capabilities 必须声明每个 operation 对应权限。

## 4. 菜单与页面模型

统一页面声明字段：

- `title`
- `route`
- `icon`
- `order`
- `required_permission`

菜单约束：

- 扩展在 Dashboard 统一挂载到 `Operations > Extensions > <domain>`。
- 页面入口来自 runtime `page` bindings，不在核心前端硬编码业务路由。
- 所有扩展页面默认支持“未安装不可见”。

## 5. 配置模型

统一配置拆分：

- `config`：非敏感配置（installation config）
- `secrets`：敏感配置（secret binding）

Schema 规则：

- 所有配置项必须声明 `type`、`description`、`required`。
- 可选项有默认值时，必须声明 `default`。
- `enum` 选项必须给出合法值和含义。
- 配置升级需声明向后兼容策略（字段重命名、废弃窗口、默认值迁移）。

## 6. 事件与可观测性

每个扩展统一事件类型前缀：`<domain>_*`，例如：

- `alerts_silence`
- `notifications_update`
- `approvals_approve`
- `backups_create`

事件最小字段：

- `event_type`
- `level`
- `message`
- `payload`
- `created_by`

## 7. 迁移执行模板

每个业务模块迁移时必须按以下顺序：

1. 迁移草案文档（边界、binding、风险）
2. runtime binding 骨架
3. extension 事件桥接
4. config/secrets 切到 installation 模型
5. 前端页面切到 schema/runtime 驱动
6. 旧核心路径降级为兼容代理并最终移除
