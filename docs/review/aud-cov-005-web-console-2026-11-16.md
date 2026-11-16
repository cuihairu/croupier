# AUD-COV-005 Web Console 审计报告 (2026-11-16)

## 结论摘要

- 总体判定：**有条件通过**；核心高危链路均有服务端授权兜底，但 DevOps 新面板存在明确的连接生命周期缺陷，需修复后交付。
- 发现计数：P0 0 项，P1 4 项，P2 7 项，P3 2 项。
- 专项核查确认终端输出经 xterm 消费，未发现 XSS 汇点；`fetchJSON` 请求自动携带认证与 scope 头。
- 静态验证：`tsc` 通过，ESLint 0 错误；DevOps/请求/scope 相关 5 个测试套件、31 个用例全部通过。
- 服务端授权实现未在本次范围内复核，涉及跨层结论均已在边界中标注。

## 发现清单

### P1 DevOps 终端 SSE 异常帧导致运行时崩溃

- 位置：`web/src/pages/Ops/Agents/DevopsTerminal.tsx:88`、`web/src/pages/Ops/Agents/DevopsTerminal.tsx:97`
- 问题：`output` 和 `end` 监听器直接调用 `JSON.parse(event.data)`；缺少 `try/catch`。畸形或非 JSON 帧会抛出未捕获异常，中断正常输出处理或结束流程。
- 触发场景：代理/中继返回格式异常的 SSE 帧，或网络中间设备注入非标准数据。
- 建议修复：仿照 `useRealtimeStream.ts:165` 的模式，在事件回调内用 `try/catch` 包裹解析逻辑。

### P1 DevOps 终端 useEffect 依赖不完整

- 位置：`web/src/pages/Ops/Agents/DevopsTerminal.tsx:151`
- 问题：effect 内部使用 `message`、`sessionId`、`onStatus` 等闭包值，但依赖数组仅含 `[sessionId]`，并手动禁用了 exhaustive-deps 警告。
- 触发场景：父组件传入新的 `message` 或 `onStatus` 引用时，终端仍持有旧引用，行为不一致。
- 建议修复：将实际使用的所有响应式值纳入依赖数组，或用 `useRef` 透传（参照 `DevopsSessionsPanel.tsx:98` 的 `useScopeReload` 模式）。

### P1 DevOps 面板无请求取消/竞态防护

- 位置：`web/src/pages/Ops/Agents/DevopsSessionsPanel.tsx:62`、`web/src/pages/Ops/Agents/DevopsSessionsPanel.tsx:73`
- 问题：`load` 和 `loadAgentOptions` 的异步结果直接 `setState`，无 `AbortController` 或版本计数器。快速切换 scope 时旧请求可能晚于新请求完成，覆盖新数据。
- 触发场景：用户在列表加载完成前切换顶栏游戏/环境。
- 建议修复：参照 `web/src/pages/Ops/Agents/index.tsx:63` 使用 `requestId` 计数器模式。

### P1 静态 JSON 输入提交前未校验

- 位置：`web/src/pages/Ops/Schedules/index.tsx:199`
- 问题：`payload: values.payload ? JSON.parse(values.payload) : {}` 在提交回调中直接解析，非法 JSON 会抛出未捕获异常阻断提交流程。
- 触发场景：用户输入 `{invalid}` 并提交创建调度任务。
- 建议修复：解析移入 try 块或表单校验规则，失败时在表单下方展示字段级错误（参照 `web/src/pages/Extensions/Store/index.tsx:158` 的现有模式）。

### P2 权限页删除动作异常未捕获

- 位置：`web/src/pages/Permissions/UsersV2/index.tsx:241`、`web/src/pages/Permissions/RolesV2/index.tsx:112`
- 问题：`remove()` 异步调用 `deleteAdmin`/`deleteRole` 后直接 `getMessage()?.success()`，无 `try/catch`。失败时成功提示仍会展示。
- 触发场景：网络异常或后端返回错误时，`Popconfirm.onConfirm` 返回的 Promise 被 antd 吞掉，用户看到错误的"已删除"提示。
- 建议修复：参照 `web/src/pages/Functions/DetailTabs.tsx:671` 添加 `try/catch` 和错误提示。

### P2 原始异常消息直接展示

- 位置：`web/src/pages/Functions/DetailTabs.tsx:671`、`web/src/pages/Functions/Directory/useDirectoryPage.ts:213`
- 问题：`message.error(err instanceof Error ? err.message : String(err))` 直接输出 `err.message`，可能包含后端堆栈信息或内部路径。
- 触发场景：后端 500 错误的消息体含敏感上下文（数据库连接串、内部服务名等）。
- 建议修复：改用 `extractErrorMessage(err, intl.formatMessage(...))` 统一过滤。

### P2 Token 存储于 localStorage

- 位置：`web/src/services/core/http.ts:18`、`web/src/pages/User/Login/index.tsx:173`
- 问题：JWT 持久化在 `localStorage`，任何 XSS 汇点均可读取。
- 触发场景：若后续新增未转义的内容渲染点，攻击者可通过注入脚本窃取管理员凭证。
- 建议修复：改用 `httpOnly` cookie 或至少在会话结束时主动清除（当前退出登录已清除：`AvatarDropdown.tsx:46`）。

### P2 DevOps 面板页面静态文案含"未交付"注释

- 位置：`web/src/pages/Ops/Agents/index.tsx:36`、`web/src/pages/Ops/Agents/index.tsx:38`
- 问题：页面描述仍写"主机与会话数据面未交付，本页不提供入口"，但同文件已渲染 `<DevopsSessionsPanel/>` 提供会话入口，文案与功能不一致。
- 触发场景：用户阅读描述后误认为功能不可用。
- 建议修复：更新描述文案以反映当前已交付的会话能力。

### P2 分页 pageSize 未统一封顶

- 位置：`web/src/pages/MenuManagement/index.tsx:57`、`web/src/pages/Functions/ExecutionLogs/index.tsx:594`
- 问题：部分页面硬编码 `pageSize: 200` 或 `50` 直传后端，未做上限约束或提示，可能导致首屏数据量过大。
- 触发场景：数据源条目数极多时，单次请求传输量膨胀。
- 建议修复：设定合理上限（如 50），或在服务端做 cap。

### P2 跨页面 `window.open` 未统一走 safeUrl

- 位置：`web/src/pages/Permissions/UsersV2/index.tsx:401`
- 问题：`window.open` 拼接 URL 后直接打开，未经过 `safeExternalHref` 校验。虽然此处路径是硬编码前缀+用户名参数，实际注入风险低，但模式不统一。
- 触发场景：若未来参数来源变化，可能打开非预期协议链接。
- 建议修复：统一走 `safeUrl` 工具函数。

### P2 部分管理操作回执仅凭 UI 状态判断

- 位置：`web/src/pages/Ops/Schedules/index.tsx:330`、`web/src/pages/Dev/Tools/index.tsx:260`
- 问题：删除操作仅凭请求成功（无异常即视为成功）更新 UI，未验证响应体中的 `success` 或 `deleted` 字段。
- 触发场景：后端返回 200 但响应体标记部分失败时，前端仍显示全部成功。
- 建议修复：检查响应体的操作结果字段后再更新 UI。

### P3 中文硬编码文案未走 i18n

- 位置：`web/src/pages/Ops/Agents/DevopsSessionsPanel.tsx:146`、`web/src/pages/Ops/Agents/DevopsSessionsPanel.tsx:176`
- 问题：面板内的确认弹窗标题、按钮文案等直接写中文字符串，未使用 `intl.formatMessage` 或 `FormattedMessage`。
- 触发场景：非中文 locale 下用户仍看到中文文案。
- 建议修复：提取到 locale 文件并使用 `useIntl()`。

### P3 按钮 loading 状态未绑定请求

- 位置：`web/src/pages/Ops/Agents/DevopsSessionsPanel.tsx:227`
- 位置：`web/src/pages/Ops/Agents/DevopsSessionsPanel.tsx:169`
- 问题：「关闭」和「刷新」按钮点击后无 loading 状态，用户可能重复点击触发重复请求。
- 触发场景：网络延迟时用户连点按钮，产生并发请求竞态。
- 建议修复：为按钮增加 `loading` 状态并在请求期间禁用。

## 已检查未见问题的区域

- **XSS**：全仓无 `dangerouslySetInnerHTML`/`innerHTML`/`document.write` 汇点；`MarkdownText`（`web/src/components/MarkdownText/index.tsx:42`）和 `safeUrl.ts` 提供了完善的 URL 协议白名单；DevOps 终端输出经 xterm 消费（`DevopsTerminal.tsx:34`），不是 DOM 直写。
- **认证与 scope 注入**：`web/src/services/core/http.ts:29-31` 和 `web/src/services/core/scope.ts:64-71` 统一注入 Authorization 和 X-Game-ID/X-Env 头。
- **类型安全**：非生成代码 0 处 `any`（排除 `services/generated/**` 和 `services/swagger/**`，ESLint 已配置忽略）；`tsc --noEmit` 0 错误。
- **危险操作确认**：`Permissions/UsersV2`、`Permissions/RolesV2`、`Dev/Tools`、`Ops/Schedules` 均有 Popconfirm/Modal.confirm。
- **路由权限**：DevOps 路由要求 `canOpsRead`（`web/config/routes.ts:496`），面板页面整体在登录后且需 ops 读取权限。
- **错误处理**：全局拦截器（`requestErrorConfig.ts:267`）提供统一 toast/notification/跳转；局部错误处理器均有 fallback。

## 审计边界

- 服务端授权实现（`internal/api/ops/`、`internal/api/` 目录）未在本次前端审计范围内复核。
- `.umi/`、`.umi-production/` 目录为 UmiJS 自动生成代码，已排除。
- `services/generated/**` 目录为 OpenAPI 生成代码，已排除；`services/swagger/**` 目录含 `any` 但标记为 ESLint 忽略，未纳入红线计数。
- Playwright E2E 测试未执行。
- 仅审计了前端代码，未审计后端 API 的 scope 校验逻辑是否与前端假设一致。
- 本报告基于 commit `43890bfc9` 和当前工作区状态（含未提交的 14 个变更文件）。

---

验证命令与结果：

- `pnpm --dir web run tsc` — 通过
- `pnpm --dir web exec eslint --ext .js,.jsx,.ts,.tsx src` — 0 错误，92 既有警告
- `pnpm --dir web exec jest --runInBand --cacheDirectory /dev/shm/croupier-audit-jest --no-cache src/pages/Ops/Agents/__tests__/devopsSessionsPanel.test.tsx src/pages/Ops/Agents/__tests__/devopsTerminal.test.tsx src/services/api/devopsSessions.test.ts src/services/core/http.test.ts src/hooks/__tests__/useScopeReload.test.ts` — 5 套件/31 用例全部通过

---

## 修复与复核附注（2026-11-16/17，接续报告正文）

### 处置记录

- P1×4 中 3 条当场修（一条一 commit）：
  - SSE 畸形帧处理 → `2f6d79d01`
  - Devops 终端 effect 依赖 → `95072aaeb`
  - Devops 会话面板陈旧响应守卫 → `c430996a2`
- 第 4 条（Schedules payload JSON）逐条复核为「已有 onFinish try/catch 兜底、缺表单项级校验」，降级 P2 并已修：`payloadRule` 字段级校验 + `pages.opsSchedules.form.payloadInvalid` 双语文案 + 纯函数直测用例 → `abc6f25a1`（`pnpm --dir web run tsc` 0 错误；`pnpm jest src/pages/Ops/Schedules --testPathPatterns index.test` 全绿）。
- P2/P3 共 9 条逐条对照当前 HEAD 复核：7 条确认属实落 todo（AUD-029..033、AUD-034/035 为 P3）；2 条核伪不收，见下。

### 复核核伪记录

- **P2「列表请求未做分页上限」（ExecutionLogs 半句不实）**：ExecutionLogs 页实际使用 `PAGE_SIZE=20` 常量分页，非无界拉取；同条目中 MenuManagement `pageSize: 200` 硬编码属实 → AUD-032。收窄后整条以 AUD-032 挂账，ExecutionLogs 半句剔除。
- **P2「权限页 try/catch 吞错」归类修正**：UsersV2/RolesV2 `remove()` 确有吞错（属实，AUD-029），但原报告同点位提到的字段校验缺口经复核为 onFinish 兜底已覆盖，不另设条目。
- **「公开配置端点跨 game 泄漏」为 Go API 侧发现**（见 `docs/review/aud-cov-001-go-api-2026-11-16.md` [P0]），不在本前端报告范围，已归 001 处置（AUD-037）。

### 本报告审计边界补充

- 报告基于 commit `43890bfc9` + 工作区；复核与修复基于 2026-11-17 HEAD（`abc6f25a1`）。行号引用以报告时点为准，修复 commit 信息以本附注为准。
