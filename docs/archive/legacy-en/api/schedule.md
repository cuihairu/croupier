# 定时调度 API

> 状态：Current（cron 定时任务调度：五字段表达式、失败计数与死信）
> handler：`internal/api/schedule/handler.go`

路径前缀 `/api/v1/schedules`（登录态；运维权限）。

## 端点清单

| 方法   | 路径                            | 说明                                          |
| ------ | ------------------------------- | --------------------------------------------- |
| GET    | `/api/v1/schedules`             | 分页列表（`page/pageSize/gameId/env/status`） |
| POST   | `/api/v1/schedules`             | 创建                                          |
| GET    | `/api/v1/schedules/:id/runs`    | 执行记录                                      |
| PUT    | `/api/v1/schedules/:id/status`  | 启停（`active` / `paused`）                   |
| POST   | `/api/v1/schedules/:id/trigger` | 立即触发一次                                  |
| DELETE | `/api/v1/schedules/:id`         | 删除                                          |

## 响应契约（裸 payload，无 envelope；字段 lowerCamelCase）

### 创建

`POST` body：

```json
{
  "name": "每日凌晨清理",
  "cronExpr": "0 3 * * *",
  "functionId": "maintenance.cleanup",
  "gameId": "demo",
  "env": "production",
  "payload": { "days": 7 },
  "maxFailedRuns": 3
}
```

响应 `{ "item": { ...ScheduleItem } }`；cron 表达式非法返回
`400 { "error": "...", "message": "invalid cron expression" }`。

### 执行记录

`GET /:id/runs` → `{ "items": [ { "id": 1, "taskRunId": "tr-...",
"status": "success|failed|dead", "message": "", "slot": "2026-08-31T03:00",
"createdAt": "..." } ], "total": 10 }`

## 语义

- **五字段 cron**（分 时 日 月 周），时区为 server 本地时区
- **失败计数与死信**：连续失败达 `maxFailedRuns` 后调度进入
  dead 状态不再执行（需人工介入重新激活）
- **手动触发**不重置失败计数，也不占用 cron 槽位
- 管理页：运维中心「定时调度」（`/ops/schedules`）

## 已知边界

- 调度器随 server 启动运行（多实例 HA 下由 cluster owner 单点执行）
- 备份等平台自身定时任务建议托管于此（见
  [备份恢复](../operations/backup-restore.md)）
