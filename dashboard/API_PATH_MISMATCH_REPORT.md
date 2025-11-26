# API 路径不匹配问题报告

## 问题概述

Dashboard 中的许多 API 调用路径与后端定义不匹配，主要问题是：
1. 缺少 `/v1` 版本前缀
2. 路径格式不一致

## 需要修复的文件

### 1. analytics.ts ❌ **严重不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/analytics/overview` | `/api/v1/analytics/overview` | ❌ 缺少 v1 |
| `/api/analytics/retention` | `/api/v1/analytics/retention` | ❌ 缺少 v1 |
| `/api/analytics/realtime` | `/api/v1/analytics/realtime` | ❌ 缺少 v1 |
| `/api/analytics/realtime/series` | `/api/v1/analytics/realtime/series` | ❌ 缺少 v1 |
| `/api/analytics/behavior/events` | `/api/v1/analytics/behavior/events` | ❌ 缺少 v1 |
| `/api/analytics/behavior/funnel` | `/api/v1/analytics/behavior/funnel` | ❌ 缺少 v1 |
| `/api/analytics/behavior/paths` | `/api/v1/analytics/behavior/paths` | ❌ 缺少 v1 |
| `/api/analytics/behavior/adoption` | `/api/v1/analytics/behavior/adoption` | ❌ 缺少 v1 |
| `/api/analytics/behavior/adoption_breakdown` | `/api/v1/analytics/behavior/adoption/breakdown` | ❌ 缺少 v1 + 路径错误 |
| `/api/analytics/payments/summary` | `/api/v1/analytics/payments/summary` | ❌ 缺少 v1 |
| `/api/analytics/payments/transactions` | `/api/v1/analytics/payments/transactions` | ❌ 缺少 v1 |
| `/api/analytics/payments/product_trend` | `/api/v1/analytics/payments/product-trend` | ❌ 缺少 v1 + 下划线vs横杠 |
| `/api/analytics/levels` | `/api/v1/analytics/levels` | ❌ 缺少 v1 |
| `/api/analytics/levels/episodes` | `/api/v1/analytics/levels/episodes` | ❌ 缺少 v1 |
| `/api/analytics/levels/maps` | `/api/v1/analytics/levels/maps` | ❌ 缺少 v1 |
| `/api/analytics/attribution` | 后端不存在此端点 | ❌ API 不存在 |
| `/api/analytics/segments` | 后端不存在此端点 | ❌ API 不存在 |

### 2. entities.ts ❌ **不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/entities` | `/api/v1/entities` | ❌ 缺少 v1 |
| `/api/entities/validate` | `/api/v1/entities/:id/validate` 或 `/api/v1/schemas/validate` | ❌ 路径错误 |

### 3. games.ts ❌ **不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/games` | `/api/v1/games` | ❌ 缺少 v1 |

### 4. me.ts ❌ **不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/me/profile` | `/api/v1/profile` | ❌ 路径错误 |
| `/api/me/password` | `/api/v1/profile/password` | ❌ 路径错误 |
| `/api/me/games` | `/api/v1/profile/games` | ❌ 路径错误 |

### 5. messages.ts ❌ **不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/messages` | `/api/v1/messages` | ❌ 缺少 v1 |
| `/api/messages/read` | `/api/v1/messages/:id/read` | ❌ 缺少 v1 + 路径错误 |
| `/api/messages/unread_count` | `/api/v1/messages/unread-count` | ❌ 缺少 v1 + 下划线vs横杠 |

### 6. audit.ts ❌ **不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/audit` | `/api/v1/audit` | ❌ 缺少 v1 |

### 7. assignments.ts ❌ **不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/assignments` | `/api/v1/assignments` | ❌ 缺少 v1 |

### 8. registry.ts ❌ **不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/registry` | 后端无此端点，可能是 `/api/v1/ops/services` | ❌ |

### 9. packs.ts ❌ **不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/packs/list` | `/api/v1/packs` | ❌ 缺少 v1 + 路径错误 |
| `/api/packs/reload` | `/api/v1/packs/reload` | ❌ 缺少 v1 |

### 10. storage.ts ❌ **不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/upload` | `/api/v1/storage/signed-url` | ❌ 完全不匹配 |

### 11. support.ts ❌ **不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/support/tickets` | `/api/v1/support/tickets` | ❌ 缺少 v1 |
| `/api/support/faq` | `/api/v1/support/faq` | ❌ 缺少 v1 |
| `/api/support/feedback` | `/api/v1/support/feedback` | ❌ 缺少 v1 |

### 12. functions-enhanced.ts ⚠️ **部分不匹配**

| Dashboard 调用 | 后端定义 | 状态 |
|---------------|---------|------|
| `/api/v1/function-calls` | 后端无此端点 | ❌ |
| `/api/v1/function-instances` | 后端无此端点 | ❌ |
| `/api/v1/analytics/coverage` | 后端无此端点 | ❌ |
| `/api/v1/registry/services` | 后端无此端点，可能是 `/api/v1/ops/services` | ❌ |
| `/api/functions/*` | 应该是 `/api/v1/functions/*` | ❌ 缺少 v1 |

## 正确的 API ✅

以下文件的 API 路径是正确的：
- auth.ts ✅
- permissions.ts ✅
- players.ts ✅
- alerts.ts ✅
- nodes.ts ✅
- roles.ts ✅
- users.ts ✅
- functions.ts ✅（基础功能）

## 修复优先级

### P0 - 立即修复（核心功能）
1. **analytics.ts** - 分析功能完全不可用
2. **games.ts** - 游戏管理不可用
3. **me.ts** - 个人资料不可用
4. **messages.ts** - 消息功能不可用

### P1 - 高优先级
5. **entities.ts** - 实体管理
6. **audit.ts** - 审计日志
7. **assignments.ts** - 分配管理
8. **support.ts** - 支持工单

### P2 - 中优先级
9. **packs.ts** - Pack 管理
10. **registry.ts** - 注册表
11. **functions-enhanced.ts** - 增强功能
12. **storage.ts** - 存储服务

## 修复策略

1. **全局替换**: 批量添加 `/v1` 前缀
2. **路径标准化**: 统一使用横杠而非下划线
3. **移除不存在的 API**: 删除或注释掉后端不存在的端点
4. **文档同步**: 确保前后端 API 定义保持一致

## 估算影响

- **受影响的文件数**: 12个
- **需要修复的 API 调用**: 约50+处
- **预计修复时间**: 30-60分钟
- **风险等级**: 中（需要彻底测试）
