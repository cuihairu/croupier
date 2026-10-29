# 2026-10-29 架构、SVG 与 TODO 整理记录

源码基线 `0d64918c`。起始工作区干净，未发现适用 AGENTS.md，已阅读 CLAUDE.md。本轮按用户要求只做架构/文档/SVG 检查，不编译整个程序，不修改业务代码、不部署、不提交推送。

执行间隙曾观察到本轮文档/脚本被外部还原，同时出现 Web 在途修改。已恢复本轮文档交付，保留 Web 修改并保存其文件摘要核对；没有恢复或覆盖这些业务代码。根 TODO 随后新增的四条审核发现也已保留并规范为 REVIEW-001–004，未以在途修复勾选完成。

## 交付

- 双语 runtime-topology.md：进程/模块/库/插件/外部依赖边界、连接方向/协议/触发/认证/数据/交互，发现/路由/调度/存储与失效，启动/失败/恢复/停机。
- 双语竖向总览与六组独立流程 SVG（注册恢复、同步调用、异步任务、CDC 告警、插件、停机），共 16 个文件，包含兼容移动端路径。互斥分支同 ALT 框内以 OR 分隔；总览圆点才表示线相连。
- README、中英文站点首页/架构总览统一资源，详细文档提供 SVG 全图链接；手机图容器横向滚动，避免把全部文字缩小到不可读。
- 修正旧 HA 文档 mTLS 误标和“状态不丢”承诺；session-lifecycle 明确为目标模型；同步 CLAUDE 的运行事实，保留约束规则。
- TODO 共 45 项：41 项架构/业务剩余交付与 4 项新增审核复验。每项稳定 ID、优先级、类型、依赖、交付验收和条件；没有新勾选。旧记录全文保留在 todo-history-2026-10-29.md，仅把复选框转为历史文字，避免双份活动任务。

## 关键事实与缺口（本轮未修复）

| 事实                                                                              | 源码证据                                                                                                                                  | 后续任务                                                                                                     |
| --------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| 单模块化 Server、四类独立 Agent，core 是包，SDK 嵌入游戏进程                      | Makefile；cmd/*-agent；agents；internal/app/agent；sdks                                                                                   | 保留拓扑，不拆新服务                                                                                         |
| 审批装配内存，SQLStore 未启用；audit signer nil                                   | internal/svc/service_context.go:342、350；internal/platform/approvals/sql_store.go                                                        | ARCH-001/002、007/008                                                                                        |
| 集群互联明文/hello role，部分失败 standalone                                      | cmd/server/cluster.go；internal/cluster/transport.go                                                                                      | ARCH-003–005                                                                                                 |
| DB binding 校验与物理名计算双来源，开库忽略 context                               | internal/svc/game_middleware.go:validateGameScope；internal/db/router/router.go:GameDB；service_context.go:newGameRouter                  | ARCH-006/009/010                                                                                             |
| TaskRunner 仅进程内 taskId map；上报错误被忽略                                    | agents/sidecar/task_runner.go:Start/run                                                                                                   | ARCH-015 已收口（2026-10-30）：重复 Start 幂等、每 taskId 代数守卫、有界事件重试；D3 重启对账触发归 ARCH-018 |
| worker 批 Append 后可 XACK，尚未批 Send；重入队 XADD 未核错误                     | internal/analytics/worker/worker.go:appendEventRow/appendPaymentRow/processMessage                                                        | ARCH-011 已解决（延迟 ACK，at-least-once）；故障复现仍 NOT RUN，归 ARCH-012                                  |
| worker 主程序取消后只等待 200ms，无 final flush join                              | cmd/analytics-worker/main.go                                                                                                              | 纳入 ARCH-012                                                                                                |
| ProviderDrain 仅日志和空 ACK，不摘流/等在途                                       | internal/agent/tcp_local_listener.go:handleDrain                                                                                          | ARCH-016                                                                                                     |
| Server 先等待 TCP 连接，后 HTTP/rootCancel；外层 30s；未调用 ServiceContext.Close | cmd/server/root.go:366–410；internal/server/tcp_listener.go:Close；internal/svc/service_context.go:Close                                  | ARCH-017/018                                                                                                 |
| devops 具有 SSH Broker/Hub/记录基础，无面板中继                                   | agents/devops/runner/runner.go；agents/devops/session                                                                                     | AGENT-002/003                                                                                                |
| 扩展 HTTP puller 没有 Authorization/scope，而目标受 protected 与读权限保护        | internal/app/agent/extension_sync_puller.go:fetch；internal/handler/routes.go:181–184；internal/api/extension/service.go:AgentSyncPayload | ARCH-021；直接接入不能称闭环，外部代理行为未核实                                                             |
| 注册 10s 仅 SyncAttempts 包装；DialAndRegister/SyncOnce 继承调用/传输预算         | core/register/registration.go；core/register/loops.go                                                                                     | ARCH-022                                                                                                     |
| synthetics 通知无应用 ACK，断线失败仅日志，不具备 capture spool 重放              | agents/synthetics/runner/runner.go:execute；agents/controlconn/controlconn.go:SendProbeEvent                                              | PROBE-002 真环境验收                                                                                         |

行号仅作导航，结论以命名函数/实际装配为准。真实部署配置、游戏账号/权威契约、第三方凭据、故障注入与压测环境尚缺，相关任务保持 NOT RUN。官方资料及版本核查日期为 2026-10-29，链接位于双语正文；不把上游协议文档当作本仓已实现证据。

## 复查方式

```bash
python3 scripts/docs/generate-architecture.py
python3 scripts/docs/check-architecture.py
pnpm --dir docs build
CROUPIER_PLAYWRIGHT_MODULE=/path/to/playwright node scripts/docs/check-architecture.cjs
git diff --check
```

浏览器使用已有 Playwright/Chromium，截图保存在 /tmp/croupier-architecture-review。静态检查只覆盖本轮架构正文、SVG、首页资源和 TODO DAG，不宣称全站无死链；VitePress 仍保留 ignoreDeadLinks。最终验证结果另记于下方。

## 本轮文件清单

- `CLAUDE.md`
- `README.md`
- `README.zh.md`
- `docs/.vitepress/config.mjs`
- `docs/.vitepress/theme/ArchitectureDiagram.vue`
- `docs/architecture/data-flow.md`
- `docs/architecture/index.md`
- `docs/architecture/layers.md`
- `docs/architecture/runtime-topology.md`
- `docs/architecture/server-ha-multi-instance.md`
- `docs/architecture/session-lifecycle.md`
- `docs/index.md`
- `docs/public/architecture/events-en.svg`
- `docs/public/architecture/events-zh.svg`
- `docs/public/architecture/execution-en.svg`
- `docs/public/architecture/execution-zh.svg`
- `docs/public/architecture/plugin-en.svg`
- `docs/public/architecture/plugin-zh.svg`
- `docs/public/architecture/registration-en.svg`
- `docs/public/architecture/registration-zh.svg`
- `docs/public/architecture/shutdown-en.svg`
- `docs/public/architecture/shutdown-zh.svg`
- `docs/public/architecture/system-en-mobile.svg`
- `docs/public/architecture/system-en.svg`
- `docs/public/architecture/system-zh-mobile.svg`
- `docs/public/architecture/system-zh.svg`
- `docs/public/architecture/task-en.svg`
- `docs/public/architecture/task-zh.svg`
- `docs/review/architecture-review-2026-10-29.md`
- `docs/review/todo-history-2026-10-29.md`
- `docs/zh/architecture/data-flow.md`
- `docs/zh/architecture/index.md`
- `docs/zh/architecture/layers.md`
- `docs/zh/architecture/runtime-topology.md`
- `docs/zh/architecture/server-ha-multi-instance.md`
- `docs/zh/architecture/session-lifecycle.md`
- `docs/zh/index.md`
- `scripts/docs/check-architecture.cjs`
- `scripts/docs/check-architecture.py`
- `scripts/docs/generate-architecture.py`
- `todo.md`

## 最终验证结果

- `pnpm --dir docs build` 通过；存量 Sass legacy API、chunk size、haproxy/conf 高亮提示不阻断构建。日志：`/tmp/croupier-doc-refresh-build.log`。
- `python3 scripts/docs/check-architecture.py` 通过：16 张 SVG XML、限定范围文档/源码路径、首页资源一致性、45 个唯一任务编号及无环依赖。
- Chromium 实际渲染通过：全部 SVG 的文本边界/重叠与总览连线穿字检查；中英文首页/总览/详细页在 1440px、390px 共 12 个页面，图片均加载、完整 SVG 链接一致、页面无横向溢出。截图：`/tmp/croupier-architecture-review`；另人工查看总览和流程图截图。
- SVG 生成器重复运行内容一致；旧 TODO 原文从 HEAD 比对一致（仅复选框展示改为历史标签）；`node --check scripts/docs/check-architecture.cjs`、`git diff --check` 通过。
- 执行间隙新增的 5 个 Web 文件在此前摘要检查时与发现时一致；最终检查观察到以下文件又发生变化：`web/src/components/PageRenderer/ResourcePageRenderer.tsx`, `web/src/components/PageRenderer/templates/playerManage.tsx`。本轮没有写入任何 Web 业务文件，保留其当前内容；新增四条 TODO 规范为复验任务，未勾选。
- 未运行全程序编译、全仓测试、部署、真实外部依赖/故障/容量验证；这些属于 TODO 条件明确的 NOT RUN 项，文档检查不代替运行保证验收。
- 改动保留于工作区，未提交或推送。文档交付备份：`/tmp/croupier-doc-delivery.tar`。

## 后续代码复审

用户随后要求重新审核整体代码/待办及游戏后台稳定性，并授权审核后提交推送。见 [代码与 TODO 对齐复审](./code-todo-alignment-2026-10-29.md)，当前清单由 45 项经过 62 项再更新为 75 项（包含用户补充的核心/外部分析边界）；本记录的“未提交/推送”和 45 项统计仅描述前轮结束状态。后续提交不包含 5 个仍在工作区的 Web 业务文件。
