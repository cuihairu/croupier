# Croupier 系统函数管理架构分析报告

## 执行摘要

Croupier 的函数管理系统采用了**分布式、描述符驱动**的架构，支持多渠道的函数发现、注册、分配和调用。当前实现存在**菜单结构分散**、**功能复用不足**、**用户体验需要统一**等问题。

---

## 一、现状分析

### 1.1 函数管理相关页面盘点

| 页面 | 路由 | 功能 | 权限 |
|-----|------|------|------|
| **GM Functions** | `/game/functions` | 列出所有函数描述符，支持参数填写、函数调用和任务管理 | `canFunctionsRead` |
| **Registry** | `/operations/registry` | 查看已注册的 Agent、函数、分配关系、覆盖率 | `canRegistryRead` |
| **Assignments** | `/game/assignments` | 管理游戏/环境下分配的函数列表 | `canAssignmentsRead` |
| **Packs** | `/game/packs` | 查看函数包清单、导出/导入、重新加载 | `canPacksRead` |

### 1.2 菜单结构现状

```
游戏管理 (GameManagement)
├── 游戏环境 (GamesEnvs)
├── 实体管理 (Entities)
├── 函数管理 (GmFunctions) ← /game/functions
├── 功能分配 (Assignments)  ← /game/assignments
└── 功能包管理 (Packs)      ← /game/packs

游戏运营 (Operations)
├── 审批管理 (Approvals)
├── 审计日志 (Audit)
├── 操作日志 (OperationLogs)
├── 服务注册表 (Registry)    ← /operations/registry
└── 服务列表 (Servers)
```

**问题：**
- 函数相关的管理功能分散在"游戏管理"和"游戏运营"两个不同菜单下
- Registry 在"运营"菜单，但与函数分配、包管理紧密相关
- 缺乏统一的"函数管理"顶级菜单

### 1.3 功能页面深度分析

#### GmFunctions (`/game/functions`)
**核心功能：**
- 列出所有函数描述符 (`listDescriptors()`)
- 根据游戏/环境过滤函数（调用 `fetchAssignments()`)
- 支持3种表单渲染模式：Enhanced UI、Form-Render、Legacy
- 支持4种路由策略：lb、broadcast、targeted、hash
- 实时函数实例查询 (`listFunctionInstances()`)
- 同步调用 + 异步任务支持
- 输出视图渲染（支持自定义 renderer 插件）

**问题：**
- 单页面承载功能过多，UI 复杂度高
- 缺少批量操作、搜索、分类等高级功能
- 没有函数版本管理或历史对比
- 输出视图插件系统复杂度高，文档缺失

#### Registry (`/operations/registry`)
**核心功能：**
- 显示已注册 Agent（按游戏/环境过滤）
- 函数覆盖率分析（已覆盖 vs 未覆盖 vs 部分覆盖）
- 支持按前缀分组、多种排序方式
- CSV 导出功能（支持细粒度导出）

**问题：**
- 菜单位置不直观
- 没有与函数调用页面的联动
- 数据展示主要是表格，缺乏可视化

#### Assignments (`/game/assignments`)
**核心功能：**
- 为游戏/环境配置允许的函数列表
- 空列表表示允许所有函数

**问题：**
- 仅支持白名单机制，没有细粒度权限控制
- 没有与函数描述符的强绑定
- 缺乏变更历史记录

#### Packs (`/game/packs`)
**核心功能：**
- 显示函数包清单（manifest）
- 导出/导入函数包（tar.gz 格式）
- 重新加载包内容
- 显示 ETag 用于版本管理

**问题：**
- 功能简单，主要是展示和导出
- 没有包内容详情视图（如包内包含的函数列表）
- 没有版本历史管理

---

## 二、后端 API 分析

### 2.1 核心 API 端点

| 端点 | 方法 | 功能 | 返回值 |
|-----|-----|------|--------|
| `/api/descriptors` | GET | 获取函数描述符列表 | `FunctionDescriptor[]` |
| `/api/descriptors?detailed=true` | GET | 获取详细信息（包含 provider manifests） | 组合对象 |
| `/api/invoke` | POST | 同步调用函数 | 函数返回值 |
| `/api/start_job` | POST | 异步启动任务 | `{ job_id }` |
| `/api/cancel_job` | POST | 取消任务 | - |
| `/api/stream_job?id={job_id}` | GET/SSE | 流式获取任务事件 | 事件流 |
| `/api/function_instances` | GET | 获取函数实例列表 | `{ instances }` |
| `/api/registry` | GET | 获取注册表（Agent、函数、覆盖率） | 注册表对象 |
| `/api/assignments` | GET/POST | 获取/设置分配关系 | 分配对象 |
| `/api/packs/list` | GET | 列出函数包清单 | 清单对象 |
| `/api/packs/export` | GET | 导出函数包 | tar.gz 文件 |
| `/api/packs/import` | POST | 导入函数包 | - |
| `/api/packs/reload` | POST | 重新加载函数包 | - |

### 2.2 权限控制

```
函数相关权限：
├── function:<function_id>:invoke    (调用特定函数)
├── functions:read / functions:*     (读取函数列表)
├── registry:read                    (查看注册表)
├── assignments:read / write         (管理分配)
├── packs:read / reload / export     (包管理)
└── packs:export                     (导出包)
```

---

## 三、数据流分析

### 3.1 函数发现流程

```
1. Admin 创建游戏/环境
   ↓
2. Agent 注册函数到 Server
   (gRPC: ControlService.RegisterFunction)
   ↓
3. Server 维护 Registry
   (存储 Agent -> Function 映射)
   ↓
4. Web UI 查询
   GET /api/descriptors
   GET /api/registry
   GET /api/assignments
```

### 3.2 函数调用流程

```
web 用户填表 
   ↓
POST /api/invoke 或 POST /api/start_job
   ↓
Server 权限检查 (RBAC)
   ↓
Server 负载均衡器选择 Agent
   ↓
gRPC 调用 Agent.Invoke
   ↓
Agent 调用游戏服务函数
   ↓
返回结果 / 流式任务事件
```

### 3.3 分配管理流程

```
Web 选择游戏 + 环境 + 函数列表
   ↓
POST /api/assignments
   ↓
Server 保存白名单到存储
   ↓
GmFunctions 页面过滤展示
```

---

## 四、现存问题清单

### 4.1 架构层面

| 问题 | 严重程度 | 影响范围 |
|-----|---------|---------|
| **菜单结构分散** | 高 | 用户导航、功能发现 |
| **页面承载过多** | 中 | GmFunctions 页面 UI 复杂 |
| **权限模型过粗** | 中 | Assignments 只支持白名单 |
| **缺乏版本管理** | 低 | 函数版本追踪 |
| **缺乏关联视图** | 中 | 分散的管理界面 |

### 4.2 功能层面

| 页面 | 问题 | 优先级 |
|-----|------|--------|
| GmFunctions | 搜索/分类缺失、表单编辑器复杂、没有调用历史 | P1 |
| Registry | 菜单位置不直观、缺乏与调用的联动 | P2 |
| Assignments | 权限粒度太粗、缺乏变更记录 | P2 |
| Packs | 功能简单、缺乏包内容详情视图 | P3 |

### 4.3 UX 层面

| 问题 | 表现 |
|-----|------|
| **认知成本高** | 用户需要在多个菜单间切换 |
| **功能发现困难** | 没有统一的函数管理入口 |
| **缺乏批量操作** | 无法批量启用/禁用函数 |
| **缺乏搜索** | 函数列表无过滤能力 |
| **缺乏操作反馈** | 调用历史、修改记录不清晰 |

---

## 五、改进建议

### 5.1 菜单结构优化

**建议方案：统一的"函数管理"菜单**

```
函数管理 (FunctionManagement) [新增]
├── 函数目录 (Catalog)           [新页面，聚合函数列表和描述]
│   ├── 按分类浏览
│   ├── 搜索和过滤
│   ├── 版本管理
│   └── 权限管理
├── 函数调用 (Invoke)            [重构 GmFunctions]
│   ├── 快速调用（简化 UI）
│   ├── 高级调用（路由策略）
│   └── 调用历史
├── 函数分配 (Assignments)       [保留，增强]
├── 实例管理 (Instances)         [从 Registry 拆分]
│   ├── Agent 管理
│   ├── 覆盖率分析
│   └── 健康检查
└── 函数包 (Packs)               [保留，增强]
    ├── 包清单
    ├── 导入/导出
    └── 包内容详情
```

**优势：**
- 集中化：所有函数操作都在一个菜单下
- 清晰化：按功能分页面，而非按对象
- 可扩展：易于添加版本管理、权限控制等功能

### 5.2 页面重构方案

#### 5.2.1 函数目录页面（新增）

```typescript
// 功能
- 全文搜索 (by name, description, category)
- 高级过滤 (category, version, assigned_to_game, health_status)
- 列表/卡片/详情三视图切换
- 批量操作 (enable/disable for game, export)
- 版本选择和对比

// 数据结构扩展
interface FunctionDescriptor {
  id: string;
  version: string;
  category: string;                // 分类
  description?: string;            // 描述
  params?: JSONSchema;
  auth?: Record<string, any>;
  outputs?: ViewDefinitions;
  metadata?: {
    created_at: string;
    created_by: string;
    updated_at: string;
    updated_by: string;
    tags?: string[];               // 标签
    deprecated?: boolean;          // 弃用标志
  };
}
```

#### 5.2.2 函数调用页面（重构）

**当前问题：** 承载了函数列表、选择、表单渲染、调用、历史等多个功能

**改进方向：**

```
页面分离模式：
├── 快速调用版（简化）
│  └── 函数选择 → 参数填写 → 调用 → 结果展示
├── 高级调用版（复杂）
│  └── + 路由策略、实例选择、幂等性 key、历史记录
└── API 接口增强
   └── 支持 tab 式/历史记录/对比

// 新增 API 端点
GET /api/function_calls?game_id=...&function_id=...&limit=20
  返回: { calls: [ { id, timestamp, user, params, result, status } ] }

GET /api/function_call/{id}
  返回: 单次调用的完整信息

POST /api/function_calls/{id}/rerun
  重新执行之前的调用
```

#### 5.2.3 实例管理页面（新增）

```
从 Registry 页面分离出来，专注于：
- Agent 生命周期管理（注册、心跳、过期）
- 函数实例覆盖率分析（表格 + 可视化）
- 健康检查和告警
- 批量操作（重启、升级、删除）

// 数据增强
GET /api/agents
  - 支持按游戏/环境/状态过滤
  - 返回实时健康状态

GET /api/agents/{agent_id}/functions
  - 该 Agent 上的函数实例列表
  - 每个函数的最后一次调用时间、成功率等

GET /api/coverage/analysis?game_id=...&env=...
  - 按分类统计覆盖率
  - 识别有风险的函数组合
```

#### 5.2.4 分配管理增强

```typescript
// 当前：简单白名单
// 改进：细粒度权限模型

interface FunctionAssignment {
  game_id: string;
  env: string;
  functions: string[];                    // 白名单
  
  // 新增：细粒度权限
  role_assignments?: {                    // 按角色分配
    [role: string]: string[];
  };
  
  // 新增：时间控制
  time_windows?: {
    enabled: string[];                    // 仅在指定时间可用的函数
    disabled: string[];                   // 禁用时间段
  };
  
  // 新增：审计
  change_history?: Array<{
    timestamp: string;
    actor: string;
    operation: 'add' | 'remove' | 'enable' | 'disable';
    function_id: string;
    reason?: string;
  }>;
}
```

#### 5.2.5 函数包增强

```
当前：只展示 manifest 和导出/导入

改进：
1. 包内容详情视图
   - 显示包内包含的函数列表
   - 版本历史
   - 依赖关系图

2. 包管理界面
   - 版本对比
   - 灰度发布（按环境/百分比）
   - 自动降级机制

3. 新增 API
   GET /api/packs/{pack_id}/contents
   GET /api/packs/{pack_id}/versions
   POST /api/packs/{pack_id}/canary?env=...&percentage=10
```

### 5.3 组件复用策略

#### 5.3.1 通用表单组件

```typescript
// 统一函数参数表单渲染
export interface FormRenderConfig {
  schema: JSONSchema;
  uiSchema?: UISchema;
  mode?: 'simple' | 'advanced';     // 简化/高级模式
  readOnly?: boolean;
  onSubmit?: (values: any) => void;
}

// 使用场处：
// - GmFunctions: 函数调用表单
// - Assignments: 参数预设配置
// - Approvals: 审批时的参数验证
```

#### 5.3.2 注册表展示组件

```typescript
// 提取 Registry 的表格和统计逻辑
export interface RegistryViewProps {
  agents?: Agent[];
  functions?: Function[];
  coverage?: Coverage[];
  loading?: boolean;
  groupBy?: 'prefix' | 'none';
  filter?: {
    gameId?: string;
    env?: string;
    uncovered?: boolean;
    partial?: boolean;
  };
  onExport?: (type: 'csv' | 'json') => void;
}

// 使用场景：
// - Operations/Registry: 完整视图
// - Instances: 部分视图
// - Dashboards: 嵌入式组件
```

#### 5.3.3 函数列表组件

```typescript
// 支持多种展示模式
export interface FunctionListProps {
  descriptors: FunctionDescriptor[];
  mode?: 'list' | 'grid' | 'table';
  search?: string;
  filters?: FunctionFilter;
  selectable?: boolean;
  selected?: string[];
  onSelect?: (id: string, selected: boolean) => void;
  onInvoke?: (id: string) => void;
}

// 使用场景：
// - Catalog: 主列表视图
// - Assignments: 函数选择
// - Approvals: 函数搜索
```

### 5.4 数据模型扩展

```typescript
// 后端应该提供的新数据结构

interface EnrichedFunctionDescriptor {
  // 原有字段
  id: string;
  version: string;
  params?: JSONSchema;
  
  // 元数据
  metadata?: {
    category: string;
    tags: string[];
    description: string;
    creator: string;
    created_at: string;
    updated_at: string;
    deprecation_notice?: string;
  };
  
  // 运行时信息
  runtime?: {
    instances: number;                    // 有多少个实例
    agents: string[];                     // 哪些 Agent 上有
    last_called?: string;                 // 最后调用时间
    success_rate?: number;                // 成功率
    avg_latency_ms?: number;              // 平均延迟
  };
  
  // 权限信息
  permissions?: {
    assigned_games: string[];             // 分配给哪些游戏
    required_role?: string;               // 所需角色
  };
}
```

### 5.5 前端路由优化

```typescript
// 新的路由结构
{
  path: '/functions',
  name: 'FunctionManagement',
  icon: 'code',
  routes: [
    { path: '/functions', redirect: '/functions/catalog' },
    
    // 主页面
    { path: '/functions/catalog', name: 'Catalog', component: './Functions/Catalog' },
    { path: '/functions/invoke', name: 'Invoke', component: './Functions/Invoke' },
    { path: '/functions/invoke/:id', hideInMenu: true, component: './Functions/InvokeDetail' },
    { path: '/functions/assignments', name: 'Assignments', component: './Functions/Assignments' },
    { path: '/functions/instances', name: 'Instances', component: './Functions/Instances' },
    { path: '/functions/packs', name: 'Packs', component: './Functions/Packs' },
    
    // 详情页
    { path: '/functions/:id', hideInMenu: true, component: './Functions/Detail' },
    { path: '/functions/:id/history', hideInMenu: true, component: './Functions/History' },
    { path: '/functions/:id/compare', hideInMenu: true, component: './Functions/Compare' },
  ]
}

// 后向兼容重定向
{ path: '/game/functions', redirect: '/functions/invoke' },
{ path: '/game/assignments', redirect: '/functions/assignments' },
{ path: '/game/packs', redirect: '/functions/packs' },
{ path: '/operations/registry', redirect: '/functions/instances' },
```

---

## 六、实施路线图

### 阶段 1（短期，2-3 周）
1. 新增菜单配置和路由结构
2. 创建函数目录页面（基础功能）
3. 分离实例管理页面（从 Registry）
4. 增强权限检查和错误提示

### 阶段 2（中期，3-4 周）
1. 重构函数调用页面（UI 分离）
2. 增加调用历史 API 和展示
3. 增强分配管理（变更历史）
4. 函数包详情视图

### 阶段 3（长期，1-2 月）
1. 版本管理和对比功能
2. 细粒度权限模型
3. 可视化监控和告警
4. 性能分析和优化建议

---

## 七、总结

Croupier 的函数管理系统功能完整但**组织散乱**。通过统一的菜单结构、清晰的页面职责分工、增强的数据模型，可以显著提升用户体验和系统可维护性。

建议优先实施**菜单统一**和**功能目录页面**，作为后续优化的基础。

