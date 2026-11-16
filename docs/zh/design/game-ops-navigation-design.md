---
title: 游戏运营后台导航与角色工作区设计（GUX-002）
---

# 游戏运营后台导航与角色工作区设计（GUX-002）

状态：todo GUX-002 设计交付，依据 `docs/design/game-operations-ux-review-2026-11-16.md`
（产品与导航小节）。本文只把**既有**能力重新分组为三个面向任务的工作区：不删除任何路由、
不新增任何后端权限、不改动任何角色定义。

## 1. 现状盘点

当前顶级导航组（`web/config/routes.ts`，umi 路由树即菜单树）：

| 分组              | 路径         | Access key                 | 背后权限（access.ts）                           |
| ----------------- | ------------ | -------------------------- | ----------------------------------------------- |
| Runtime Console   | `/console`   | `canConsoleRead`           | `console:read`、`pages:read`、`function:invoke` |
| Functions & Pages | `/functions` | `canFunctionsAndPagesRead` | functions/pages/resources/OpenAPI 读            |
| Approvals         | `/approvals` | `canAuditRead`             | `audit:read`                                    |
| Analytics         | `/analytics` | `canAnalyticsRead`         | `analytics:read` + feature flag                 |
| Development       | `/dev`       | `canDevRead`               | `dev:read`/`bugs:*` + feature flag              |
| Operations & SRE  | `/ops`       | `canOpsRead`               | `ops:read` + feature flag                       |
| Risk Control      | `/risk`      | `canOpsRead`               | 与 Ops 同键                                     |
| Support           | `/support`   | `canSupportRead`           | `support:read` + feature flag                   |
| System Management | `/system`    | `canSystemConfigRead`      | `system:config`、games 读                       |
| Access & Accounts | `/admin`     | `canPermissionManage` 等   | roles/users 读写                                |

审查指出的问题：函数、SDK、页面构建入口与玩家服务、风控、运维并列——所有角色面对同一棵
十组大树，工作上下文丢失。

## 2. 角色任务清单与权限映射

角色来自 `configs/roles.json`（17+ 预置角色）。代表性别权限集：

| 角色（roles）                                       | 任务                                                                       | 现有权限                                                                                                                                              | 当前导航可见性                                                                                         |
| --------------------------------------------------- | -------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| 运营 operations_manager / operations_lead           | 玩家查询、封禁/充值/邮件补偿、订单审核、活动配置、审批、风控告警、执行记录 | `function:invoke`、`functions:read`、`games:read`、`registry:read`、`resources:read`、`assignments:read`、`currency:read`、`item:read`、`wallet:read` | Console ✓；审批 **✗（缺 `audit:read`）**；风控 **✗（缺 `ops:read`）**；执行记录 ✓（挂在 Functions 下） |
| 客服 support_manager                                | 玩家查询、钱包/货币读、工单                                                | `currency:read`、`wallet:read`、`functions:read`、`games:read`、`registry:read`                                                                       | Console ✓；客服 **✗（缺 `support:read`）**                                                             |
| 开发接入 developer / frontend_developer / tech_lead | 函数/资源契约、OpenAPI、页面构建与发布、invoke 调试、SDK 分发              | `functions:read`、`pages:edit/read`、`registry:read`、`resources:read`、`uploads:write`、`function:invoke`                                            | Functions & Pages ✓；Console ✓；Dev **部分（feature flag）**                                           |
| 平台管理与运维 admin / super_admin / ops_manager    | 游戏/环境、账号角色、节点/服务器/agent、系统配置、扩展、证书               | `games:manage`、`system:config`、`ops:*`、`certificates:manage`、`roles:*`/`users:*`（admin）                                                         | System ✓；Ops ✓；Admin ✓                                                                               |

**权限缺口只记录、不在本项修**。上表运营/客服角色缺 `audit:read`（审批）、`ops:read`
（风控/节点）、`support:read`（工单）。改 `configs/roles.json` 属权限模型决策，归用户拍板；
GUX-002 只让工作区可见性从「实际存在的权限」推导。

## 3. 目标导航结构

三个工作区包住现有十个顶级组。**所有路径不变**——只重排菜单树，书签、文档链接、
逐路由 access key 全部保持有效：

```
运营工作台 Operations         （可见：canConsoleRead || canAuditRead ||
                              canSupportRead || canAnalyticsRead）
  Runtime Console  /console/...              7 个菜单节点 + 模板
  Approvals        /approvals
  Support          /support
  Risk Control     /risk
  Analytics        /analytics
  Execution Logs   /functions/execution-logs （入口副本；同一路由）
开发与接入 Dev & Integration  （可见：canFunctionsAndPagesRead || canDevRead）
  Functions & Pages /functions/...           （execution-logs 副本除外）
  Development       /dev/...
  SDK Distribution  /functions/sdk-distribution（入口副本）
平台管理与运维 Platform       （可见：canSystemConfigRead || canOpsRead ||
                              canPermissionManage || canExtensionsRead）
  System Management /system/...
  Operations & SRE  /ops/...
  Access & Accounts /admin/...
  Extensions        /system/extensions       （本就在 system 下；入口保留）
```

设计决策：

1. **重分组、不重路由**。umi 路由树驱动菜单；把现有组节点挂到三个工作区父节点下只改分组。
   任一子组可见则工作区父节点渲染（umi access 语义经派生键支持）。
2. **不造新权限**。工作区可见键是现有 access key 在 `web/src/access.ts` 里的并集，例如
   `canOperationsWorkspace = canConsoleRead || canAuditRead || canSupportRead || canAnalyticsRead`。
   admin 保持全量（现有 `*` 权限已授予全部键）。
3. **跨界入口，不复制页面**。执行记录与 SDK 分发保持单一路由、单份页面；第二个工作区得到
   指向同一路径的菜单项。不复制组件、不复制权限。
4. **作用域头部保持全局**。游戏/环境选择器对三个工作区都留在顶栏；高影响操作提交时重新
   展示实际作用域（既是既有模式，也在 GUX-012 强化）。
5. **落地页规则（供 GUX-003）**。登录后落到第一个可见工作区的首页：运营 → `/console/home`、
   开发 → `/functions/catalog`、平台 → `/system`。全部不可见时退回个人页。
6. **各工作区入口（供 GUX-003/008）**。每个工作区首页呈现该角色的任务（待办/异常/最近处理/
   常用入口）。Console 首页不再承载产品建设说明（审查观察 #1）；这类内容移到开发工作区，
   在那里是上下文而不是噪音。

## 4. 实现草图（不属于本交付）

- `web/config/routes.ts`：三个父节点（operations / dev-integration / platform）包住现有组；
  `/` 的 `redirect` 保持。
- `web/src/access.ts`：三个派生 `can*Workspace` 键。
- 语言文件：工作区标题（运营工作台 / 开发与接入 / 平台管理与运维）。
- 无服务端改动；无 `rbac.json`/`roles.json` 改动。

## 5. 边界

1. 本项是设计交付（GUX-002）；实现随 GUX-003+ 落地并走 DoD。
2. 角色权限缺口（§2）是平台决策——只记录，不裁决。
3. Risk Control 今天由 `ops:read` 把门，与 Operations & SRE 耦合；本设计保持该耦合
   （不造新权限）；解耦需用户批准的权限模型变更。
4. Feature flag（`analytics`/`dev`/`support`/`ops`）照旧叠加在权限之上。
