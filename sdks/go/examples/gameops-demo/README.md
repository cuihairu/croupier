# GameOps Demo Server（界面生成链 E2 批次：demo-server 基础）

`gameops-demo` 是界面生成链 E2「示例操作与异常状态闭环」的 `demo-server 基础`
（前端审查评定最大剩余 P1 的首块交付）：一个可运行的游戏侧服务，作为 E1 示例数据
的服务端化基座，供后续把 E1 模板执行器替换为 `executePageBinding` 的闭环片使用。

它把 E1 浏览器示例数据（`web/src/components/PageRenderer/templates/gameOpsDemoData.ts`）镜像为
进程内共享、可变的数据仓库，并通过 Croupier SDK→Agent→Server 的既有注册链暴露与
E1 模板 PageSpec 一致的 32 个函数契约外加任务态 `report.generate`（共 33 个；`FunctionContract`
字段：resource/operation/capability/execution/risk/input/output schema），供控制台的正式
页面生成/发布/执行链解析。

## 启动

```bash
cd sdks/go
go run ./examples/gameops-demo \
  -agent 127.0.0.1:19091 \
  -game-id croupier-demo \
  -env local-demo
```

函数契约会随 SDK→Agent 注册链上报到 Server；Agent 需先启动并指向同一 Server。
`-register-only` 可用于只注册冒烟，`Ctrl+C`/`SIGTERM` 优雅退出并关闭 SDK。

## 覆盖能力

- 32 个精确函数 ID：`player.*`、`mail_task.*`、`ban.*`、`order.*`、`activity.*`（与 E1 模板绑定一致）。
- 任务态函数：`report.generate`（`Execution: "task"`、`Capability: "task"`）——异步任务、延迟/结果未知、失败与幂等重试场景的可调用契约。
- 列表查询：统一 `current/pageSize/orderBy/orderDirection`，按资源暴露可选筛选字段。
- 状态闭环：封禁/解封/延期、任务创建/重试/取消、订单补发、活动发布/上线/结束/回滚。
- 操作写入共享数据源，随后 `*.get`/`*.list` 均可回读（write-then-read）。
- E1 边界：3 区服、26 玩家（含 >40 字符昵称）、30 订单、12 封禁、4 任务、8 活动、UTC 时间。
- 服务端幂等守卫：重复补发拒绝（`already_credited`）、无失败重试拒绝（`no_failed_targets`）。

## 边界（非完整 E2 闭环）

- 本例只交付**示例数据源 + 契约注册与执行器**，不创建游戏账号/真实权限，也不替代真实游戏验收。
- 预览与发布共用本数据源，需在已发布 demo PageSpec 上继续验证（本文件只承诺后端基础）。
- 异常/延迟/结果未知经 `report.generate` 复现（mode=task 立即返回任务 id，执行器等待 `delayMs` 后成功或失败，`fail` 标志与上下文取消生效）；待审批经高风险函数（`player.ban`/`ban.*`）的真实审批闸复现。控制台侧端到端走查（任务启动→流式完成、审批队列、幂等键重试）仍属人工验证。
- 重置入口目前仅进程内 `Reset()`（测试/嵌入方使用）；后续接 demo 页面时统一暴露控制台入口。

## 测试

```bash
cd sdks/go
go test ./examples/gameops-demo/... -count=1
```
