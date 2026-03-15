# official.backup-advanced 迁移草案

更新时间：2026-03-15  
状态：Draft（Phase 7 预备）

## 1. 目标

将备份管理中的增强能力迁移为 `official.backup-advanced` 官方扩展，核心只保留基础备份模型、权限审计与扩展底座。

## 2. 当前范围

- 现有入口：`internal/api/backup/*` 与 `internal/api/ops` 的备份操作
- 现有能力：
  - 备份列表
  - 创建备份
  - 删除备份
  - 下载地址/文件

## 3. 迁移边界

核心保留：

- 备份基础数据模型与最小兼容 API
- 扩展安装、运行时、事件与健康检查底座
- 权限与审计

扩展迁移：

- 备份策略（定时、增量、跨区域）
- 存储后端策略与生命周期管理
- 高级页面与运维入口

## 4. 目标 binding 草案

- page:
  - `backups.overview`（`/backups`）
- capability:
  - `backups.management`（`list/create/delete/download`）
- function:
  - `backups.create`
  - `backups.delete`

## 5. 分步建议

1. 先落 runtime bindings 骨架和 create/delete 事件桥接（本轮完成）。
2. 把高级备份策略配置迁到 installation config。
3. 扩展化下载与远端存储策略，核心仅做兼容代理。
4. 稳定后再清理核心中仅为高级备份存在的耦合逻辑。

## 6. 风险

- 下载链路包含本地文件与远程地址两种模式，需要保持兼容行为。
- 备份任务与运维流程耦合，切换时要保证任务状态语义不变。
