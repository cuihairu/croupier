# API 契约审计报告（2026-03-08）

范围：`croupier-dashboard` 前端服务层 vs `croupier/services/server/modules` 后端 API 定义。

## 已修复

1. `profile/password` 字段不一致

- 前端原调用：`{ current, password }`
- 后端契约：`{ oldPassword, newPassword }`
- 修复：前端 `me.ts` 已映射为 `oldPassword/newPassword`。

2. `profile` 更新字段别名不一致

- 前端编辑页使用 `display_name`
- 后端契约字段为 `nickname`
- 修复：前端 `me.ts` 已将 `display_name` 映射到 `nickname`。

3. `messages` 已读接口路径不一致

- 前端原调用：`POST /api/v1/messages/read`（批量）
- 后端契约：`POST /api/v1/messages/:id/read`
- 修复：前端 `messages.ts` 改为逐条调用 `/:id/read`。

4. `messages` 分页参数命名不一致

- 前端常用 `size`
- 后端契约定义 `pageSize`
- 修复：前端 `messages.ts` 对外兼容 `size`，请求时统一映射为 `pageSize`。

## 本轮新增能力（与契约兼容）

1. 个人中心增强

- 账号信息补全（ID、最近登录 IP）。
- 登录记录结构化展示（时间、结果、IP、属地、类型、UA）。
- “可申请权限”清单 + 申请弹窗（提交反馈工单，失败降级复制申请文案）。

2. Workspace 侧（前序已完成）

- 版本详情接口、时间过滤、回滚、导入导出、备份包、告警桥接等能力已与后端当前契约对齐。

## 仍需后续治理（不属于本次增量修复）

1. 仓库存在大量历史 TS 类型错误（非本次功能引入）

- `npx tsc --noEmit` 仍有多模块报错（Functions/ComponentManagement/Entities/Permissions 等）。
- 建议单独立项做“类型系统清债”，按模块逐批治理。

2. `Profile` 权限申请仍是“反馈工单通道”

- 当前后端无专用“权限申请单”接口。
- 建议新增专用 API（如 `/api/v1/profile/permission-requests`）并接审批流。

## 建议执行顺序

1. 先补“权限申请专用后端接口 + 审批流对接”。
2. 再按模块推进 TS 类型清债（优先 Permissions、Functions、Profile 相关路径）。
3. 最后补端到端回归（Profile/Workspace/Permissions 三条主链路）。
