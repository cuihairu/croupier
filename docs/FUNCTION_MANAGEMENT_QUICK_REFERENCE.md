# Croupier 函数管理系统 - 快速参考指南

## 关键要点 (30 秒速览)

| 问题 | 当前状态 | 建议改进 | 优先级 |
|-----|---------|---------|--------|
| 菜单结构 | 函数功能分散在 2 个菜单下 | 统一为"函数管理"菜单 | 高 |
| 页面设计 | GmFunctions 单页面 650+ 行代码 | 拆分为 5 个专注页面 | 中 |
| 权限模型 | 粗粒度（functions:read 等） | 细粒度（function:{id}:invoke 等） | 中 |
| 数据流 | 多页面独立加载，无缓存 | 全局 Context + 共享缓存 | 低-中 |

---

## 新菜单结构速览

```
函数管理 (FunctionManagement) ← 新增统一菜单
├─ 函数目录 (Catalog)        ← 搜索、浏览、版本管理
├─ 函数调用 (Invoke)         ← 重构的 GmFunctions
├─ 函数分配 (Assignments)    ← 增强的分配管理
├─ 实例管理 (Instances)      ← 从 Registry 拆分
└─ 函数包 (Packs)            ← 增强的包管理
```

---

## 5 个新页面概览

### 1️⃣ 函数目录 (Catalog)
**目的：** 发现和浏览函数
```
搜索 + 分类 + 版本 + 状态
      ↓
列表/卡片/详情视图
      ↓
快速操作：[调用] [分配]
```

**关键功能：**
- 全文搜索（名称、描述、分类）
- 高级过滤（版本、状态、实例数）
- 三视图切换
- 批量操作

---

### 2️⃣ 函数调用 (Invoke) 
**目的：** 执行函数
```
选择函数 + 填参数 + 调用 + 查看结果
                           + 历史记录
```

**关键功能：**
- 参数表单（简化/高级模式）
- 调用历史集成
- 结果展示（自定义 renderer）
- 重新运行功能

---

### 3️⃣ 实例管理 (Instances)
**目的：** Agent 和覆盖率管理
```
Agent 列表 → 实例分布 → 覆盖率分析
         ↓
    健康检查 → 告警 → 批量操作
```

**关键功能：**
- Agent 生命周期管理
- 覆盖率表格 + 可视化
- 健康检查和告警
- CSV 导出

---

### 4️⃣ 函数分配 (Assignments)
**目的：** 权限和分配管理
```
选择游戏/环境 → 编辑函数白名单 → 设置细粒度权限
                                ↓
                    添加角色分配、时间窗口、审计
```

**关键功能：**
- 基础白名单
- 按角色分配（新增）
- 时间窗口控制（新增）
- 变更历史（新增）

---

### 5️⃣ 函数包 (Packs)
**目的：** 包管理和版本控制
```
查看清单 → 导入/导出 → 查看包内容
                     ↓
                  版本历史 → 灰度发布
```

**关键功能：**
- 包清单展示
- 导入导出功能
- 包内容详情（新增）
- 版本历史（新增）
- 灰度发布（新增）

---

## 5 个通用组件

### 组件清单

| 组件 | 功能 | 依赖 |
|-----|------|------|
| **FunctionFormRenderer** | JSONSchema 表单 | Ant Design Form |
| **FunctionListTable** | 表格 + 搜索 + 排序 | Ant Design Table |
| **RegistryViewer** | 注册表展示 | 自定义 |
| **FunctionCallHistory** | 历史时间线 | 自定义 |
| **FunctionDetailPanel** | 详情面板 | 自定义 |

### 使用矩阵

```
           FunctionForm  FunctionList  Registry  History  Detail
Catalog         ✓            ✓                            ✓
Invoke          ✓            ✓                    ✓
Assignments     ✓            ✓
Instances                     ✓            ✓
Packs                                             ✓       ✓
Approvals       ✓            ✓
Dashboard                                  ✓      ✓       ✓
```

---

## 新增 API 端点 (10 个)

| 端点 | 类型 | 用途 |
|-----|------|------|
| `/api/functions/summary` | GET | 汇总信息（一次请求） |
| `/api/function_calls` | GET | 查询历史 |
| `/api/function_calls/{id}` | GET | 单次详情 |
| `/api/function_calls/{id}/rerun` | POST | 重新运行 |
| `/api/agents` | GET | Agent 列表 |
| `/api/agents/{id}/functions` | GET | Agent 上的函数 |
| `/api/coverage/analysis` | GET | 覆盖率分析 |
| `/api/packs/{id}/contents` | GET | 包内容 |
| `/api/packs/{id}/versions` | GET | 版本历史 |
| `/api/packs/{id}/canary` | POST | 灰度发布 |

---

## 数据模型扩展

### FunctionDescriptor (扩展)

```typescript
{
  id: string;
  version: string;
  
  // 新增：元数据
  metadata?: {
    category: string;
    tags: string[];
    description: string;
    creator: string;
    created_at: string;
    updated_at: string;
    deprecation_notice?: string;
  };
  
  // 新增：运行时信息
  runtime?: {
    instances: number;
    agents: string[];
    last_called?: string;
    success_rate?: number;
    avg_latency_ms?: number;
  };
}
```

### FunctionAssignment (扩展)

```typescript
{
  game_id: string;
  env: string;
  functions: string[];              // 基础白名单
  
  role_assignments?: {              // 新增：角色分配
    [role: string]: string[];
  };
  
  time_windows?: {                  // 新增：时间控制
    enabled: string[];
    disabled: string[];
  };
  
  change_history?: Array<{          // 新增：审计
    timestamp: string;
    actor: string;
    operation: 'add'|'remove'|'enable'|'disable';
    function_id: string;
  }>;
}
```

---

## 实施三阶段

### 第 1 阶段（2-3 周） - 基础设施
- [ ] 更新菜单配置
- [ ] 创建 5 个新页面目录
- [ ] 创建后向兼容重定向
- [ ] 新增 3 个 API 端点

**交付：新菜单上线 + 基础函数目录 + 实例管理分离**

### 第 2 阶段（3-4 周） - 核心增强
- [ ] 重构 Invoke 页面
- [ ] 集成调用历史 API
- [ ] 增强 Assignments 管理
- [ ] 增强 Packs 管理

**交付：UI 重构完成 + 历史功能上线 + 分配增强**

### 第 3 阶段（1-2 月） - 高级功能
- [ ] 版本对比工具
- [ ] 细粒度权限实现
- [ ] 可视化监控
- [ ] 性能优化

**交付：完整的版本管理 + 权限系统 + 监控**

---

## 路由变更

### 新路由

```
/functions/catalog          函数目录
/functions/invoke           函数调用
/functions/invoke/:id       调用详情
/functions/assignments      函数分配
/functions/instances        实例管理
/functions/packs            函数包
/functions/:id              函数详情
/functions/:id/history      调用历史
/functions/:id/compare      版本对比
```

### 旧路由重定向

```
/game/functions          → /functions/invoke
/game/assignments        → /functions/assignments
/game/packs              → /functions/packs
/operations/registry     → /functions/instances
```

---

## 权限扩展

### 新权限定义

```
resource 粒度：
├─ function:{id}:read              读特定函数
├─ function:{id}:invoke            调用特定函数
├─ function:{id}:view_history      查看历史
├─ assignments:{game_id}:read      读特定游戏分配
├─ assignments:{game_id}:write     修改特定游戏分配
├─ registry:read                   读注册表
└─ packs:manage                    管理包

角色绑定：
├─ game_operator          调用已分配的函数
├─ game_admin             修改分配关系
├─ function_developer     上传/更新包
├─ ops_engineer           Registry + Instances
└─ system_admin           完全权限
```

---

## 关键数据结构

### FunctionManagementState (前端全局状态)

```typescript
{
  // 基础
  selectedGame: string;
  selectedEnv: string;
  
  // 缓存
  descriptors: Map<string, FunctionDescriptor>;
  assignments: Map<string, string[]>;
  registry: RegistryData;
  agents: Agent[];
  recentCalls: FunctionCall[];
  
  // UI
  searchText: string;
  filters: FunctionFilter;
  viewMode: 'list' | 'grid' | 'detail';
}
```

---

## 性能指标目标

| 指标 | 当前 | 目标 | 方法 |
|-----|------|------|------|
| 页面加载时间 | 2s | 1.5s | 缓存 + API 优化 |
| API 调用次数 | 3-5 | 1 | /api/functions/summary |
| 搜索响应 | 500ms | 200ms | 前端搜索 + 缓存 |
| 历史查询 | - | 300ms | 新 API 端点 |

---

## 常见问题 (FAQ)

### Q1: 新菜单上线后，旧链接会失效吗？
**A:** 不会。所有旧链接都会保持重定向，确保用户无缝迁移。

### Q2: 需要迁移现有数据吗？
**A:** 不需要。新页面和旧页面共享后端数据，无需迁移。

### Q3: 会影响现有的 API 吗？
**A:** 不会。所有旧 API 保持兼容，只是新增了 API 端点。

### Q4: 权限模型变化对现有权限配置有影响吗？
**A:** 新权限与旧权限兼容，现有权限配置保持有效。

### Q5: 什么时候能看到新菜单？
**A:** 第 1 阶段完成后（预计 2-3 周），可以看到新菜单结构和基础页面。

---

## 关键链接

- 📄 **详细分析：** FUNCTION_MANAGEMENT_ARCHITECTURE_ANALYSIS.md
- 📊 **对比图表：** FUNCTION_MANAGEMENT_COMPARISON.txt
- 📋 **执行摘要：** FUNCTION_MANAGEMENT_EXECUTIVE_SUMMARY.md
- 🚀 **这份指南：** FUNCTION_MANAGEMENT_QUICK_REFERENCE.md

---

## 需要帮助？

1. **理解现状？** 看对比图表
2. **了解改进方案？** 看执行摘要
3. **深入技术细节？** 看详细分析
4. **快速查阅？** 看这份快速参考

**负责人：** 架构评审团队
**更新时间：** 2024-11-13
**版本：** 1.0

