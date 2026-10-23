---
title: 游戏运营业务模板示例数据
---

# 游戏运营业务模板示例数据

## 1. 定位与边界

五个默认游戏运营模板（玩家工作台、邮件与补偿、封禁管理、订单核查、活动配置）共用一份浏览器内存示例数据，用于模板预览、交互联调和验收演示。

- 独立示例环境：`gameId=croupier-demo`、`environment=local-demo`。
- 示例操作身份：`accountId=gm-demo`，逻辑权限为 `console.read`、`gameops.demo.read`、`gameops.demo.write`。
- 该身份只描述示例页面中的操作者与权限意图，**不会创建真实登录账号、JWT、后端 scope 或权限记录**；访问模板路由仍受当前控制台会话与既有鉴权约束。
- 示例数据不写入真实 `page_specs`、`gameId/env` 菜单或游戏业务表；刷新页面或点击重置会恢复初始 seed。

## 2. 启动与访问

```bash
cd web
pnpm start
```

使用现有控制台登录态打开以下入口：

| 模板       | 路由                                   | 主要示例能力                                                   |
| ---------- | -------------------------------------- | -------------------------------------------------------------- |
| 玩家工作台 | `/console/templates/player-manage`     | 玩家查询/详情、新建/编辑/删除、封禁、充值、邮件、批量封禁/解封 |
| 邮件与补偿 | `/console/templates/mail-compensation` | 邮件/补偿任务、审批状态、失败重试、补偿数量与总量              |
| 封禁管理   | `/console/templates/ban-management`    | 封禁记录、创建、解封、延期、批量解封                           |
| 订单核查   | `/console/templates/order-review`      | 订单与发货凭据、对账差异、缺发货补发幂等                       |
| 活动配置   | `/console/templates/activity-config`   | 活动版本、发布、上线/下线、回滚                                |

模板页顶部横幅显示当前示例 `gameId/env/accountId` 与默认菜单分类，并提供「重置示例数据」入口。

## 3. 数据规模与边界用例

- 3 个区服、26 名玩家，含长昵称（大于 40 字符）。
- 30 笔订单，覆盖 `credited/pending/missing/not_needed` 发货状态与 `match/missing_credit/missing_order` 对账结论。
- 12 条封禁记录，覆盖 `active/expired/lifted`。
- 邮件/补偿任务覆盖审批中、部分失败、执行中、完成等状态；活动覆盖多版本、多时区与多业务状态。
- 玩家、订单、封禁、任务、活动与操作记录通过 `playerId/serverId/orderNo/approvalId/operationId` 相互关联，可验证跨页选择、筛选、分页和写后回读。

## 4. 重置与测试

- 页面入口：模板页顶部「重置示例数据」。
- 代码入口：`resetGameOpsDemoRuntime()`，原地替换共享 store 中全部集合。
- 回归测试：`cd web && npx jest src/components/PageRenderer/templates --runInBand`。
- 真实游戏数据接入时整体替换 demo 执行器，示例数据不参与正式执行链；真实环境验收需单独记录，不能以本示例通过替代。
