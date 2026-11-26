# SUI-046 代表模板走查证据（2026-11-26）

线上栈：deploy-host `192.168.5.5` 平台栈（croupier-server :18780 + croupier-postgres），scope `X-Game-ID: demo-game` / `X-Env: demo`，admin 会话。源码 HEAD `06040b3a2`（走查期间无代码变更，仅新增回归测试 `web/src/pages/PageStudio/CompositeEditor/__tests__/crudTemplateWalkthrough.test.ts`，2/2 绿）。

| 文件 | 内容 | 走查步骤 |
| --- | --- | --- |
| `sui046-crud-player.json` | `GET /api/v1/component-templates/crud--player` 完整响应（树 + digest `f375e468…8244` + updatedAt 2026-11-16） | 取代表模板 |
| `sui046-create-payload.json` | 客户端链产物：真实 `instantiateTemplateDetailed`（插入）→ 真实 `compileTree`（发布 payload）的 sections + componentTemplates 快照（jest 走查测试捕获） | 插入 + 发布 payload |
| `sui046-published-spec.json` | accept-and-publish 后 `GET /api/v1/pages/sui046-crud-walkthrough`：status=published、5 sections、bindings 分配（player.get/delete 输入 `/id` ← form `/id`；delete `requireConfirm:true`）、componentTemplates 快照落库 | 发布落库验证 |
| `sui046-execute-list.json` | `POST /api/v1/console/pages/sui046-crud-walkthrough/bindings/player.list/execute`（body `{}`）：200 + 10 行玩家数据（requestId 76d21502…、traceId 37046bf7…） | 执行①表格 autoRun |
| `sui046-execute-get.json` | 同页 `bindings/player.get/execute`（body `{"context":{"form":{"id":"20001"}}}`）：200 + 玩家 20001 详情（requestId 7194e745…）。行操作 `params {{row.id}}` 经 context.form 解析 | 执行②行操作详情 |
| `sui046-tpl-v1.json` / `sui046-tpl-v2.json` | 自建模板 `sui046-freshness-tpl` 两次内容；Create 落 digest `af504827…a81f`，PUT 更新后 digest `dcec4634…7b5a` | 模板更新 |
| `sui046-fresh-page.json` | 用旧 digest `af504827…` 实例化的 mini 页 payload，发布后页面快照与模板当前 digest 失配（`computeStaleTemplateNames` 触发条件） | 更新提醒 |
| `sui046-templates-stale.json` | `GET /api/v1/component-templates?pageSize=100` 全量：72 条模板，68/68 builtin 族全部 `stale:true`（发现 F1：postgres jsonb 键序归一 vs Go marshal 字典序的 raw 字节比较恒假） | 服务端 stale 误报取证 |

关键响应摘录（未单独落盘的）：

- 创建提案：`POST /api/v1/versioning/pages/composite` → `{"pageKey":"sui046-crud-walkthrough","pageType":"composite","proposalKey":"composite--sui046-crud-walkthrough","published":false,"quality":"basic"}`（demo 环境发布分级为人工接受）。
- 接受并发布：`POST /api/v1/proposals/composite--sui046-crud-walkthrough/accept-and-publish` → `{"pageKey":"sui046-crud-walkthrough","draftRevision":1,"publishedVersion":1}`。
- 模板更新提醒触发条件：页面快照 digest `af5048279a5724a8cae373a3360bba67122502caa581be774b9604b4a404a81f` ≠ 模板当前 digest `dcec46344a6303a9276bfc29771ec7ddfbb5c520339480f4f975753b51177b5a`（判定逻辑由 `templateFreshness.test.ts` 覆盖）。
- 执行偶发超时记录：player.get 首次执行 `{"error":"internal_error","message":"context deadline exceeded"}`，未改参数重试即 200（dispatch 偶发超时，见走查文档 F3）。
- 清理：两页与自建模板均经 API 删除并复核 404；`DELETE` 路径为 `/api/v1/versioning/pages/:pageKey`（`/api/v1/pages/:pageKey` 无 DELETE 路由，误用返回 gin 默认 404 文本）。
