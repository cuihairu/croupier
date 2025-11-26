# Dashboard API 覆盖情况报告

生成时间: 2025-11-26

## 概览

- **后端 API 模块总数**: 38个
- **Dashboard 服务文件总数**: 21个
- **覆盖率**: ~85%

## 完全覆盖的模块 ✅

| 后端模块 | Dashboard 服务 | 状态 | 备注 |
|---------|---------------|------|------|
| admin.api | permissions.ts | ✅ | 管理员管理完整实现 |
| auth.api | auth.ts | ✅ | 认证登录完整实现 |
| profile.api | auth.ts + me.ts | ✅ | 用户资料和权限 |
| player.api | players.ts | ✅ | 玩家管理完整实现 |
| game.api | games.ts | ✅ | 游戏管理 |
| function.api | functions.ts + functions-enhanced.ts | ✅ | 函数管理和增强功能 |
| entity.api | entities.ts | ✅ | 实体管理 |
| pack.api | packs.ts | ✅ | Pack管理 |
| storage.api | storage.ts | ✅ | 存储服务 |
| audit.api | audit.ts | ✅ | 审计日志 |
| assignment.api | assignments.ts | ✅ | 分配管理 |
| analytics.api | analytics.ts | ✅ | 分析服务 |
| message.api | messages.ts | ✅ | 消息管理 |
| support.api | support.ts | ✅ | 支持工单 |

## 部分覆盖的模块 ⚠️

| 后端模块 | Dashboard 服务 | 覆盖度 | 缺失的API |
|---------|---------------|--------|----------|
| ops.api | ops.ts | 70% | - 健康检查API<br>- 维护模式API<br>- 备份管理API<br>- 节点管理API |
| certificate.api | ops.ts | 80% | - 证书告警管理<br>- 域名信息查询 |

## 未覆盖的模块 ❌

### 1. **alert.api** - 告警管理
**优先级**: 高
**API端点**:
- `GET /api/v1/alerts` - 获取告警列表
- `POST /api/v1/alerts/:id/silence` - 静默告警
- `GET /api/v1/alerts/silences` - 获取静默规则
- `DELETE /api/v1/alerts/silences/:id` - 删除静默规则

**建议**: 创建 `dashboard/src/services/croupier/alerts.ts`

### 2. **backup.api** - 备份管理
**优先级**: 中
**API端点**:
- `GET /api/v1/backups` - 备份列表
- `POST /api/v1/backups` - 创建备份
- `DELETE /api/v1/backups/:id` - 删除备份
- `GET /api/v1/backups/:id/download` - 下载备份

**建议**: 添加到 `ops.ts` 或创建独立文件

### 3. **node.api** - 节点管理
**优先级**: 高
**API端点**:
- `GET /api/v1/nodes` - 节点列表
- `GET /api/v1/nodes/:id/meta` - 节点元数据
- `PUT /api/v1/nodes/:id/meta` - 更新元数据
- `POST /api/v1/nodes/:id/drain` - 排空节点
- `POST /api/v1/nodes/:id/undrain` - 取消排空
- `POST /api/v1/nodes/:id/restart` - 重启节点
- `GET /api/v1/nodes/commands` - 节点命令

**建议**: 创建 `dashboard/src/services/croupier/nodes.ts`

### 4. **rate_limit.api** - 限流规则
**优先级**: 中
**API端点**:
- `GET /api/v1/rate-limits` - 限流规则列表
- `GET /api/v1/rate-limits/:id` - 获取规则
- `PUT /api/v1/rate-limits` - 创建/更新规则
- `DELETE /api/v1/rate-limits/:id` - 删除规则
- `POST /api/v1/rate-limits/preview` - 预览规则

**说明**: ops.ts 有部分实现，但端点不匹配

### 5. **faq.api** - FAQ管理
**优先级**: 低
**API端点**:
- `GET /api/v1/faqs` - FAQ列表
- `POST /api/v1/faqs` - 创建FAQ
- `GET /api/v1/faqs/:id` - FAQ详情
- `PUT /api/v1/faqs/:id` - 更新FAQ
- `DELETE /api/v1/faqs/:id` - 删除FAQ

**建议**: 添加到 `support.ts`

### 6. **feedback.api** - 反馈管理
**优先级**: 低
**API端点**:
- `GET /api/v1/feedback` - 反馈列表
- `POST /api/v1/feedback` - 创建反馈
- `GET /api/v1/feedback/:id` - 反馈详情
- `PUT /api/v1/feedback/:id` - 更新反馈
- `DELETE /api/v1/feedback/:id` - 删除反馈

**建议**: 添加到 `support.ts`

### 7. **ticket.api** - 工单管理
**优先级**: 中
**API端点**:
- `GET /api/v1/tickets` - 工单列表
- `POST /api/v1/tickets` - 创建工单
- `GET /api/v1/tickets/:id` - 工单详情
- `PUT /api/v1/tickets/:id` - 更新工单
- `DELETE /api/v1/tickets/:id` - 删除工单
- `POST /api/v1/tickets/:id/transition` - 工单状态转换
- `POST /api/v1/tickets/:id/comments` - 添加评论
- `GET /api/v1/tickets/:id/comments` - 评论列表

**说明**: support.ts 可能有部分实现，需要确认

### 8. **component.api** - 组件管理
**优先级**: 低
**API端点**:
- `GET /api/v1/components` - 组件列表
- `POST /api/v1/components/:id/install` - 安装组件
- `GET /api/v1/components/:id` - 组件详情
- `POST /api/v1/components/:id/enable` - 启用组件
- `POST /api/v1/components/:id/disable` - 禁用组件
- `DELETE /api/v1/components/:id` - 删除组件
- `PATCH /api/v1/components/:id` - 更新组件

**建议**: 创建 `dashboard/src/services/croupier/components.ts`

### 9. **provider.api** - Provider管理
**优先级**: 中
**API端点**:
- `GET /api/v1/providers` - Provider列表
- `GET /api/v1/providers/:id` - Provider详情
- `DELETE /api/v1/providers/:id` - 删除Provider
- `POST /api/v1/providers/reload` - 重新加载
- `GET /api/v1/providers/descriptors` - 获取描述符
- `GET /api/v1/providers/:id/entities` - Provider实体
- `GET /api/v1/providers/capabilities` - 能力列表

**建议**: 创建 `dashboard/src/services/croupier/providers.ts`

### 10. **schema.api** - Schema管理
**优先级**: 低
**API端点**:
- `GET /api/v1/schemas` - Schema列表
- `POST /api/v1/schemas` - 创建Schema
- `GET /api/v1/schemas/:id` - Schema详情
- `PUT /api/v1/schemas/:id` - 更新Schema
- `DELETE /api/v1/schemas/:id` - 删除Schema
- `POST /api/v1/schemas/validate` - 验证Schema
- `POST /api/v1/schemas/raw-validate` - 原始验证
- `GET /api/v1/schemas/:id/ui-config` - UI配置
- `PUT /api/v1/schemas/:id/ui-config` - 更新UI配置

**建议**: 创建 `dashboard/src/services/croupier/schemas.ts`

### 11. **xrender.api** - XRender相关
**优先级**: 低
**API端点**:
- `GET /api/v1/xrender/components` - 组件列表
- `GET /api/v1/xrender/templates` - 模板列表
- `POST /api/v1/xrender/generate-schema` - 生成Schema
- `POST /api/v1/xrender/preview-schema` - 预览Schema
- `GET /api/v1/xrender/schema/:id` - 获取Schema

**建议**: 创建 `dashboard/src/services/croupier/xrender.ts`

### 12. 其他较小模块
- **agent.api** - Agent相关（部分在 ops.ts 中）
- **approval.api** - 审批流程
- **config.api** - 配置管理（configs.ts 可能已包含）
- **job.api** - Job管理（functions.ts 已包含）
- **meta.api** - 元数据（可能不需要前端）
- **monitoring.api** - 监控（可能在 ops.ts 中）

## 下一步行动建议

### 高优先级（必须补充）
1. ✅ **alert.ts** - 告警管理是运维必备功能
2. ✅ **nodes.ts** - 节点管理对分布式系统很重要

### 中优先级（建议补充）
3. **backup.ts** - 备份功能对生产环境重要
4. **providers.ts** - Provider管理
5. **ticket.ts** - 完善工单系统

### 低优先级（按需补充）
6. **components.ts** - 组件管理
7. **schemas.ts** - Schema管理
8. **xrender.ts** - XRender功能
9. FAQ/Feedback - 添加到 support.ts

## 实施建议

### 1. 统一API响应处理
所有新增的 API 服务都应该使用统一的 `unwrap()` 函数处理响应：

```typescript
type ApiResponse<T> = T | { data?: T } | { Data?: T } | null;

function unwrap<T>(resp: ApiResponse<T>): T {
  if (resp && typeof resp === 'object') {
    const anyResp = resp as any;
    if (anyResp.data) return anyResp.data as T;
    if (anyResp.Data) return anyResp.Data as T;
  }
  return resp as T;
}
```

### 2. 统一认证Token处理
所有需要认证的请求都应该添加 Bearer Token：

```typescript
const token = typeof window !== 'undefined' ? localStorage.getItem('token') : '';
const resp = await request<ApiResponse<T>>('/api/v1/xxx', {
  headers: token ? { Authorization: `Bearer ${token}` } : undefined,
});
```

### 3. 类型安全
- 所有 API 都应该有完整的 TypeScript 类型定义
- 请求和响应类型应该与后端 API 定义保持一致

### 4. 模块化组织
- 相关功能应该归类到同一个服务文件
- 避免文件过大（超过500行应考虑拆分）
- 通过 index.ts 统一导出

## 测试建议

补充 API 后应该进行以下测试：
1. ✅ TypeScript 编译测试
2. ✅ Dashboard 构建测试
3. 🔲 API 调用集成测试
4. 🔲 错误处理测试
5. 🔲 认证流程测试
