# 前端 API 分层重构任务清单

## 🎯 目标
- 所有网络访问统一通过 `src/services` 封装，页面/组件不直接拼接 `/api/*`
- SSE、下载、上传、普通 fetch 走统一 helper，自动注入 token / scope header
- 引入 lint/CI 约束，防止再次出现裸 `fetch`

## 📌 近期交付
1. **服务层梳理**
   - [ ] 列出 dashboard 中所有直接 `fetch` / `EventSource` / `window.location` 的调用点
   - [ ] 为缺少 service 的 API 新建 `services/croupier/*` 模块或 hooks
   - [ ] 为 SSE、文件导出/上传等特殊场景提供统一 helper
2. **组件迁移**
   - [ ] ComponentManagement / VirtualObjectManager / EntityComposer 仅调用 services
   - [ ] GM Functions 与 Ops 模块（Jobs、Backups、Alerts…）迁移到 services
   - [ ] Support / Account / MessagesBell 等页面或组件引用新 helper
3. **公共工具**
   - [ ] 新增 `services/core/request.ts`（或 util），封装 `fetchJSON`、`eventSourceFactory`
   - [ ] 集中处理 game/env/token header 注入逻辑
   - [ ] 将 `apiUrl` helper 合并入公共 request 工具
4. **质量保障**
   - [ ] 添加 ESLint 规则：禁止直接在源代码中出现 `fetch('/api` 或 `new EventSource('/api)`
   - [ ] 为关键 service 编写单元/集成测试

## 🛠️ 后续计划
- 迁移后逐步引入 React Query / ahooks useRequest 统一数据层
- 下载/上传/导出接口支持签名校验与错误回调
- 在 CI 中加入 lint + jest 检查，阻止回归
