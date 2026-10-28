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

## 5. 服务端示例基座（E2 `demo-server` 基础）

E2 闭环需要在共享数据源之上提供可调用函数契约与执行器，而非浏览器内存行。`sdks/go/examples/gameops-demo` 是本片第一步的服务端基座：

- 把本节的示例 seed 镜像为进程内可变 store（`newGameOpsStore()`/`Reset()`）。
- 通过 SDK→Agent→Server 既有注册链注册 E1 `PageSpec` 绑定引用的 32 个精确函数 ID（`player.*`、`mail_task.*`、`ban.*`、`order.*`、`activity.*`）外加任务态 `report.generate`（共 33 个），携带 `FunctionContract` 字段（`resource/operation/capability/execution/risk/inputSchema/outputSchema`）。
- 查询契约接受 `current/pageSize/orderBy/orderDirection` 与资源专属筛选字段；写操作写入共享 store 并可回读。
- 幂等守卫在服务端（已发货补发 → `already_credited`；无失败目标重试 → `no_failed_targets`）。
- E2 异常状态闭环：`report.generate` 是唯一 `Execution: "task"` 契约。以 mode=task 调用时服务端立即返回任务 id（结果未知直到完成）；执行器等待 `delayMs`（默认约 2s）后按 `fail` 标志成功或失败，并响应上下文取消。其 `runSeq` 统计实际执行次数，平台按幂等键去重（发生在执行器之前）时重试不改变它——即幂等重试场景的可观测证据。待审批场景经既有高风险函数（`player.ban`、`ban.*`）可达：其默认策略 `RequireApproval: true` 走真实审批闸。

```bash
make run-gameops-demo        # 默认 agent=127.0.0.1:19091 game=croupier-demo env=local-demo
cd sdks/go && go test ./examples/gameops-demo/... -count=1
```

边界：本基座尚未把 E1 模板执行器替换为 `executePageBinding`；剩余 E2 闭环（页面生成→发布→binding→异常状态走查）为后续片，须与真实游戏验收分别记录。
