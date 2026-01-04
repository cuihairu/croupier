---
title: 权限控制
icon: shield
order: 4
category:
  - 核心概念
tag:
  - 权限
  - RBAC
  - ABAC
---

# 权限控制

Croupier 实现**零信任安全**模型，提供 RBAC/ABAC 混合权限控制、操作审批和完整审计链。

## 目录

[[toc]]

## 权限模型

### 整体架构

```mermaid
graph TB
    User[用户]
    Role[角色]
    Permission[权限]
    Function[函数]

    User -->|拥有| Role
    Role -->|分配| Permission
    Permission -->|保护| Function

    ABAC[ABAC 策略]
    Function -->|动态检查| ABAC
    User -->|属性匹配| ABAC
```

### RBAC 模型

RBAC (Role-Based Access Control) 基于角色的访问控制。

```
用户 (User)
  ↓ 拥有
角色 (Role)
  ↓ 分配
权限 (Permission)
  ↓ 保护
函数 (Function)
```

### ABAC 模型

ABAC (Attribute-Based Access Control) 基于属性的访问控制。

```javascript
// 评估表达式
user.roles.includes('admin') || (game_id === 'my-game' && env === 'dev')
```

## 用户和角色

### 用户定义

```json
{
  "user_id": "user_123",
  "username": "admin",
  "email": "admin@example.com",
  "roles": ["admin", "operator"],
  "attributes": {
    "department": "operations",
    "level": 5
  }
}
```

### 角色定义

```json
{
  "role_id": "admin",
  "name": "管理员",
  "permissions": [
    "*.*"  // 所有权限
  ]
}

{
  "role_id": "gm",
  "name": "游戏管理员",
  "permissions": [
    "player.*",
    "item.*",
    "guild.*"
  ]
}

{
  "role_id": "viewer",
  "name": "查看者",
  "permissions": [
    "player.view",
    "item.view"
  ]
}
```

## 权限定义

### 权限格式

```
{entity}.{operation}

示例：
- player.ban      # 封禁玩家
- player.view     # 查看玩家
- item.create     # 创建物品
- item.delete     # 删除物品
- *.*             # 所有权限（谨慎使用）
```

### 游戏作用域

权限可以限定到特定游戏：

```
game:{game_id}:{permission}

示例：
- game:my-game:player.ban    # 仅 my-game 的封禁权限
- game:test-game:*.*         # test-game 的所有权限
```

## 函数权限配置

### 基础权限

```json
{
  "id": "player.ban",
  "auth": {
    "permission": "player.ban"
  }
}
```

### ABAC 表达式

```json
{
  "id": "player.ban",
  "auth": {
    "permission": "player.ban",
    "allow_if": "has_role('admin') || has_role('senior_gm')"
  }
}
```

### 可用表达式

| 函数 | 说明 | 示例 |
|------|------|------|
| `has_role(role)` | 检查角色 | `has_role('admin')` |
| `has_permission(perm)` | 检查权限 | `has_permission('player.view')` |
| `==` | 等于 | `env == 'prod'` |
| `!=` | 不等于 | `env != 'dev'` |
| `&&` | 逻辑与 | `has_role('admin') && env == 'prod'` |
| `\|\|` | 逻辑或 | `has_role('admin') \|\| has_role('gm')` |
| `()` | 分组 | `(a \|\| b) && c` |

### 可用变量

| 变量 | 类型 | 说明 |
|------|------|------|
| `user` | object | 当前用户信息 |
| `user.roles` | array | 用户角色列表 |
| `game_id` | string | 目标游戏 ID |
| `env` | string | 环境 (dev/staging/prod) |
| `function_id` | string | 被调用的函数 ID |

## 审批流程

### 双人规则

高风险操作需要双人审批：

```json
{
  "id": "player.ban",
  "auth": {
    "permission": "player.ban",
    "two_person_rule": true,
    "approval": {
      "enabled": true,
      "threshold": 2  // 需要两人审批
    }
  }
}
```

### 审批流程

```mermaid
stateDiagram-v2
    [*] --> 待审批: 提交调用
    待审批 --> 已通过: 足够审批数
    待审批 --> 已拒绝: 拒绝
    待审批 --> 已取消: 超时
    已通过 --> 执行中: 开始执行
    执行中 --> 已完成: 执行成功
    执行中 --> 已失败: 执行失败
    已完成 --> [*]
    已失败 --> [*]
    已拒绝 --> [*]
    已取消 --> [*]
```

### 审批 API

```bash
# 创建审批请求
POST /api/approvals
{
  "function_id": "player.ban",
  "payload": {...},
  "reason": "玩家使用外挂"
}

# 获取待审批列表
GET /api/approvals?state=pending

# 审批通过
POST /api/approvals/{id}/approve

# 审批拒绝
POST /api/approvals/{id}/reject
{
  "reason": "证据不足"
}
```

## 审计日志

### 审计事件

所有操作都会记录审计日志：

```json
{
  "audit_id": "audit_20241201_001",
  "timestamp": "2024-12-01T10:30:00Z",
  "user": {
    "user_id": "user_123",
    "username": "admin"
  },
  "action": "function.invoke",
  "game_id": "my-game",
  "env": "prod",
  "function_id": "player.ban",
  "payload_preview": {
    "player_id": "***",
    "duration": 24
  },
  "result": "success",
  "approval_id": "approval_123",
  "ip": "192.168.1.100",
  "ip_region": "中国 上海"
}
```

### 敏感字段脱敏

```yaml
# 配置脱敏字段
server:
  audit:
    sensitive_fields:
      - "password"
      - "token"
      - "secret"
      - "api_key"
```

### 审计链防篡改

每条审计记录包含哈希值：

```json
{
  "audit_id": "audit_001",
  "hash": "sha256(prev_hash + content)",
  "prev_hash": "sha256(...)"
}
```

## 风险等级

### 风险分级

| 等级 | 说明 | 审批要求 |
|------|------|----------|
| `low` | 低风险 | 无需审批 |
| `medium` | 中风险 | 可选审批 |
| `high` | 高风险 | 强制双人审批 |

### 风险配置

```json
{
  "id": "player.ban",
  "ui": {
    "risk_level": "high",
    "risk_warning": "高风险操作，封禁后玩家无法登录",
    "confirm_message": "确认封禁玩家？"
  }
}
```

## 限流保护

### 函数级限流

```json
{
  "id": "player.ban",
  "semantics": {
    "rate_limit": "10rps",
    "concurrency": 5
  }
}
```

### 用户级限流

```json
{
  "semantics": {
    "user_rate_limit": "5rps",
    "user_burst": 10
  }
}
```

## 最佳实践

### 1. 最小权限原则

```json
// ❌ 不推荐：过度授权
{
  "roles": ["admin"]  // 拥有所有权限
}

// ✅ 推荐：按需授权
{
  "roles": ["gm"],  // 仅游戏管理权限
  "permissions": ["player.ban", "player.view"]
}
```

### 2. 环境隔离

```json
{
  "auth": {
    "allow_if": "env == 'dev' || has_role('admin')"
  }
}
```

### 3. 审批配置

```json
{
  "auth": {
    "approval": {
      "enabled": true,
      "threshold": "has_role('admin') ? 1 : 2",
      "approvers": ["admin", "senior_gm"],
      "timeout": "24h"
    }
  }
}
```

### 4. 审计保留

```yaml
server:
  audit:
    enabled: true
    retention_days: 365
    backup_enabled: true
```

## 相关文档

- [函数管理](./function-management.md)
- [配置管理](../configuration.md)
- [安全配置](../operations/security.md)
