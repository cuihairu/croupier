# GUX-017 标杆流程端到端验收（2026-11-17）

目标栈：本地三进程真栈（server 18790 sqlite / agent 19093 / demo-game 18093 + web dev 8000），发布页渲染 demo-game 活数据。验收驱动 `/tmp/gux017/acceptance.js`（Playwright），取证不记录任何凭据。

## 结果：20/20 步通过

驱动输出见 `acceptance-results.txt`。A1–A12 覆盖发布页玩家查询→详情→补偿→确认→回执全链，B1–B4 覆盖模板向导（内置内存演示数据，横幅如实可见），C1–C4 覆盖 1280×800 笔记本视口。

## 发布链闭环（spec 字段落库）

验收页面不是编辑器/预览侧，而是走完整提案→保存→发布链落到已发布 spec 后由发布页渲染：proposals 创建 → accept → save（保存后 draftRevision 自增）→ publish（必须用**保存后**的新 draftRevision，用旧值返回 409 草稿版本冲突）→ GET publishedVersion 回读。验收时的已发布版本：`player-manage` publishedVersion 3、`mail-compensation` publishedVersion 5。A1–A12 全部在发布页执行真实 binding（list/detail/retry_failed/create）读取 demo-game 活数据，即 spec 字段已落库并生效。

## 流程覆盖

| 阶段 | 步骤 | 要点 |
| --- | --- | --- |
| 玩家查询 | A1/A2 | 发布页列表渲染真数据；查询栏输入「玩家7号」→ 查询按钮过滤命中 20007（唯一行 + 已封禁标签，20001 不在列表） |
| 键盘操作 | A3/A4 | Tab 走焦（去空白匹配，7 次到「查看」）+ Enter 开详情抽屉，身份锚 20007；Esc 关闭（按可见性判定） |
| 任务列表 | A5 | task-mail-1002 部分失败可见 |
| 部分成功 | A6/A7 | 重试失败 → 确认弹窗 → 回执白名单（重试数/已送达/失败数）；任务状态刷新为已发送 |
| 业务错误 | A8 | 对无失败任务重试 → 「资源操作未完成」常驻面板 + `no_failed_targets` 原因 + **无回执** |
| fail-closed | A9 | 新建补偿指定玩家 0 人提交 → 错误 toast + 具体原因（指定玩家补偿必须选择至少一名玩家）+ 弹窗不关；取消后常驻错误面板 |
| 创建回执 | A10 | 改全服提交 → 回执白名单（ID + 审批单号） |
| 作用域切换 | A11/A12 | 切 default 空态 → 切回 demo-game/demo 数据恢复（不串） |
| 模板向导 | B1–B4 | 预览横幅（内置内存演示数据如实标注）→ 补偿任务信息 → 指定玩家×2（即时已选人数汇总）→ 核对摘要（人数/单价/总额）→ 必勾确认 → 已提交审批回执 |
| 视口 | C1–C4 | 1280×800 下列表/详情/任务/向导可用 |

## 截图索引

| 文件 | 步骤 |
| --- | --- |
| `a1-player-list-1440.png` | A1 列表 |
| `a2-player-search-banned-1440.png` | A2 过滤命中 |
| `a3-player-detail-drawer-1440.png` | A3 详情抽屉 |
| `a4-drawer-closed.png` | A4 Esc 关闭 |
| `a5-task-list-partial-1440.png` | A5 部分失败行 |
| `a6-retry-confirm-modal.png` / `a6-retry-receipt-1440.png` | A6 确认弹窗 / 回执 |
| `a8-no-failed-targets-error-1440.png` | A8 业务错误面板 |
| `a9-create-form-players.png` / `a9-create-failclosed-error.png` / `a9-create-failclosed-panel.png` | A9 fail-closed 三态 |
| `a10-create-receipt-approval-1440.png` | A10 创建回执 |
| `a11-scope-default-1440.png` / `a12-scope-back-demo-1440.png` | A11/A12 作用域切换 |
| `b1-wizard-preview-1440.png` | B1 模板预览横幅 |
| `b2-wizard-step1.png` | B2 步骤 1（任务信息表单） |
| `b3-wizard-step2.png` | B3 指定玩家×2 |
| `b4-wizard-review.png` / `b4-wizard-receipt-approval-1440.png` | B4 核对 / 审批回执 |
| `c1-player-list-1280.png` / `c2-player-detail-1280.png` / `c3-task-list-1280.png` / `c4-wizard-1280.png` | C1–C4 笔记本视口 |

## 验收发现并已修复（本项交付内容）

1. **业务错误按成功呈现**（`ResourceMutationFeedback`/`ResourcePageRenderer`）：demo-game 以 200 + `data.error` 返回业务错误，原实现渲染成功回执。改为 throw 判败：toast 通用文案 + 常驻错误面板（具体原因）+ 不渲染回执 + 弹窗不关。A8/A9 断言锁定。
2. **查询栏装饰化**（五个资源模板）：列表绑定 `assignments: []` 使查询栏纯装饰（点击查询列表恒为全量）。按各 list 函数真实契约补齐 filters 与 form 来源 assignments（player: nickname/status；mail_task: type/status；ban: nickname/reason/status；order: nickname/channel/reconciliation；activity: name/type/status）；契约不支持的列（身份 ID、无关枚举）移出查询栏。
3. **枚举标签不渲染**（`SchemaFormRenderer` + 模板）：rjsf v5 `optionsList` 只消费 `ui:options.enumNames`，schema 级 `enumNames`/`x-enum-options` 不生效，下拉露原始枚举值。模板改用平台 hints 契约 `x-enum-options`；SchemaFormRenderer 两条路径（fields 展示层、无 fields 的 createForm 兜底）归一到 `ui:options.enumNames`。
4. **fixture 漂移同步**：`scripts/e2e/fixtures/demo-game/player-manage.pagespec.json` 随模板修订重新导出，drift-lock 测试锁定。

## 已知边界（如实，未修）

- 发布页面包屑展示原始 category key（如 Mail）、页副标题展示 pageKey（如 `template--mail-compensation`）——观感问题，不影响功能，留待后续文案项。
- demo.db 经验收自身写入（task-mail-1002 → 已发送、新增一条全服补偿任务），属验收预期变更。
- 真实游戏验收沿 UI-001，本 fixture 验收不抵扣。
