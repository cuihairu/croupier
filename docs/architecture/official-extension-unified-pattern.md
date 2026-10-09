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

## 8. Provider 块（#66 P0 增量契约，2026-10-10 定稿）

外部平台接入型扩展（provider 插件）在 release manifest 上声明 `provider` 块。三层模型与完整设计见 [Provider 插件设计](../design/provider-plugin-design.md) §3；本节只定字段契约。

```json
{
  "provider": {
    "type": "openapi",
    "operations": ["day_report", "user_live", "order_list"],
    "permissions": { "operate": "external-platform.operate" }
  }
}
```

- **字段闭集**：恰好 `type` / `operations` / `permissions` 三键，任何其他键非法。守卫测试 `internal/core/extension/manifest` 的 `TestManifestProviderBlockFieldSet` 锁死，扩键须先过 binding spec 同构评审。
- `type`：driver 类型闭集起步 `openapi | webhook`（`webhook` 为 P2 预留）；缺省 `openapi`（与 `externalfunc.ParseProviderBinding` 现状一致）。
- `operations`：该 provider 暴露的方法闭集（非空字符串数组，不重复）；调用面 function ID 沿用 `external.<provider>.<method>`（`externalfunc.BuildFunctionID` 现状，key 经 `SanitizeKey` 归一）。
- `permissions`：可选覆盖，键⊆`read`/`operate`/`admin`；缺省按 §3 三层权限（`<domain>.read/operate/admin`），不声明即继承。
- **与 binding spec 同构**：本块与 runtime binding 的 `spec_json` 字段集（`provider/type/operations/enabled/config`）同构——安装时从 manifest 派生默认绑定，运营可在 installation 上覆写 `config`。两侧字段集变更必须同步评审，防止漂移。
- 解析入口：`manifest.ParseProviderBlock`（无 provider 块返回零值，未知键/类型越出闭集/operations 形态错/permissions 层级越界报错）。

> 该契约适用于新增 provider 型扩展；先于本模式的连接器条目
> （`official.external-platform`）随 P1 批次补声明，不回溯改造其余四业务扩展。
