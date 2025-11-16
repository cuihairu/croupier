# 游戏后台角色权限体系设计

## 角色层级架构

```
超级管理员 (super_admin)
├── 系统管理员 (admin)
├── 开发人员 (developer)
├── 测试人员 (tester)
├── 运维人员 (ops)
├── 数据分析师 (analyst)
└── 客服人员 (support)
```

## 权限域说明

| 权限域 | 说明 | 示例权限 |
|--------|------|----------|
| `system:*` | 系统级操作 | `system:config`, `system:restart`, `system:backup` |
| `user:*` | 用户管理 | `user:create`, `user:update`, `user:delete`, `user:view` |
| `game:*` | 游戏配置管理 | `game:config`, `game:deploy`, `game:restart` |
| `player:*` | 玩家管理 | `player:query`, `player:update`, `player:ban` |
| `function:*` | 函数管理 | `function:register`, `function:deploy`, `function:test` |
| `job:*` | 任务管理 | `job:create`, `job:view`, `job:cancel`, `job:retry` |
| `audit:*` | 审计查看 | `audit:view`, `audit:export` |
| `monitor:*` | 监控数据 | `monitor:view`, `monitor:alert` |
| `data:*` | 数据分析 | `data:query`, `data:export`, `data:report` |
| `support:*` | 客服功能 | `support:ticket`, `support:chat` |

## 角色详细权限

### 1. 超级管理员 (super_admin)
**权限**：`["*"]` - 所有权限

**职责**：
- 系统最高权限持有者
- 紧急情况下的故障恢复
- 重要配置变更的最终审批
- 新角色和权限的创建

**使用场景**：
- 系统初始化配置
- 重大故障处理
- 安全事件响应

### 2. 系统管理员 (admin)
**权限**：
```json
[
  "system:config", "system:restart", "system:backup",
  "user:*", "game:*", "function:*",
  "audit:view", "monitor:view"
]
```

**职责**：
- 日常系统管理和维护
- 用户账号和权限管理
- 游戏环境配置管理
- 系统备份和恢复

**使用场景**：
- 新用户账号创建
- 角色权限分配调整
- 游戏配置更新
- 定期系统维护

### 3. 开发人员 (developer)
**权限**：
```json
[
  "function:register", "function:update", "function:test", "function:view",
  "job:create", "job:view", "job:cancel",
  "game:config:read", "player:query", "monitor:view"
]
```

**职责**：
- 新功能开发和部署
- 功能测试和调试
- 代码质量保证
- 技术文档编写

**使用场景**：
- 注册新的游戏功能函数
- 测试功能逻辑和性能
- 查看玩家数据进行调试
- 监控功能运行状态

### 4. 测试人员 (tester)
**权限**：
```json
[
  "function:test", "function:view",
  "job:create", "job:view",
  "player:create:test", "player:query",
  "game:config:read"
]
```

**职责**：
- 功能测试执行
- Bug验证和报告
- 测试用例设计
- 质量保证流程

**使用场景**：
- 执行功能测试用例
- 创建测试用户和数据
- 验证Bug修复结果
- 性能和压力测试

### 5. 运维人员 (ops)
**权限**：
```json
[
  "system:monitor", "system:restart",
  "job:view", "job:cancel", "job:retry",
  "monitor:*", "audit:view",
  "function:deploy", "game:deploy"
]
```

**职责**：
- 系统运行状态监控
- 故障诊断和处理
- 性能优化和调优
- 部署和发布管理

**使用场景**：
- 监控系统性能指标
- 处理系统告警和故障
- 执行功能发布部署
- 故障恢复和回滚

### 6. 数据分析师 (analyst)
**权限**：
```json
[
  "data:*", "player:query", "player:export",
  "audit:view", "monitor:view",
  "job:create:readonly"
]
```

**职责**：
- 游戏数据分析和挖掘
- 用户行为分析
- 运营数据报告
- 业务指标监控

**使用场景**：
- 生成日常运营报告
- 分析玩家行为模式
- 导出数据进行深度分析
- 制作数据可视化图表

### 7. 客服人员 (support)
**权限**：
```json
[
  "player:query", "player:update:basic",
  "support:ticket", "audit:view:player"
]
```

**职责**：
- 玩家问题处理
- 客服工单管理
- 玩家信息查询和更新
- 客服质量保证

**使用场景**：
- 查询玩家账号信息
- 处理玩家投诉和建议
- 更新玩家基础信息
- 查看玩家操作历史

## 权限控制机制

### 1. 基于角色的权限继承
- 用户通过角色获得权限
- 支持多角色分配
- 角色权限可以动态调整

### 2. 权限检查流程
```
请求 → JWT认证 → 提取用户角色 → RBAC权限检查 → 业务逻辑执行
```

### 3. 审计和监控
- 所有权限操作都会记录审计日志
- 敏感操作支持双人审批机制
- 实时权限使用情况监控

## 配置文件说明

### RBAC配置 (`configs/rbac.game-roles.json`)
定义了所有角色及其对应的权限列表。

### 用户配置 (`configs/users.game-roles.json`)
定义了预设的用户账号及其角色分配。

### 使用方法
1. 将配置文件复制到相应位置
2. 根据实际需要调整权限配置
3. 重启服务生效

## 安全建议

1. **最小权限原则**：只分配完成工作所需的最小权限
2. **定期权限审查**：定期检查和清理不必要的权限
3. **强密码策略**：所有账号使用强密码和双因子认证
4. **权限分离**：避免单人拥有过多权限，实施职责分离
5. **审计监控**：建立完善的权限使用审计和监控机制