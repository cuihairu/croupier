# Croupier 函数管理系统 - 完整架构总结

## 📋 文档导航

本项目包含3份详细的架构分析文档，建议按以下顺序阅读：

1. **FUNCTION_ARCHITECTURE.md** (24KB) - 核心架构总览
   - 系统分层架构
   - 关键组件介绍
   - Web前端系统
   - 数据流与调用链
   - 设计特性分析

2. **FUNCTION_COMPONENTS_DEEP_DIVE.md** (25KB) - 组件深度分析
   - 描述符加载器详解
   - 注册表系统深入
   - 函数包系统详解
   - 函数调用流程
   - Web前端实现细节
   - HTTP API详细定义
   - 安全与审计机制

## 🏗️ 核心架构速览

### 系统分层

```
Web UI (GmFunctions | Registry | Packs)
       ↓ HTTP REST API
HTTP Server (internal/app/server/http/)
       ↓
Descriptor Store | Registry Store | Pack System
       ↓
Agent (gRPC) → Game Functions
```

### 关键特性

| 特性 | 说明 | 位置 |
|------|------|------|
| 描述符驱动 | 单一JSON源驱动UI/验证/权限/审计 | `internal/function/descriptor/` |
| 多源聚合 | Legacy + Provider manifest统一管理 | `internal/platform/registry/` |
| 动态表单 | 从JSON Schema自动生成UI表单 | `web/src/pages/GmFunctions/` |
| 权限管理 | RBAC + 条件表达式的灵活权限 | `internal/security/rbac/` |
| 异步任务 | SSE实时流，进度报告 | `internal/app/server/http/server.go` |
| 数据可视化 | 多种Renderer，支持数据变换 | `web/src/plugin/registry.tsx` |
| 完整审计 | Trace ID关联，敏感字段掩码 | `internal/audit/` |

## 📁 目录结构关键路径

### 后端核心

```
internal/
├── function/descriptor/
│   ├── loader.go          # 描述符加载器
│   └── loader_test.go
├── platform/registry/
│   └── store.go          # 注册表存储
├── pack/
│   ├── manager.go        # 包管理器
│   └── typereg.go        # Protocol Buffer类型注册表
└── app/server/http/
    ├── server.go         # HTTP服务器主体
    └── ops_routes.go     # 运维路由
```

### 前端核心

```
web/src/
├── pages/
│   ├── GmFunctions/      # 函数调用工作台
│   ├── Registry/         # 代理和覆盖率仪表盘
│   └── Packs/            # 包管理界面
├── services/croupier/
│   ├── functions.ts      # 函数API
│   ├── registry.ts       # 注册表API
│   └── packs.ts          # 包API
└── plugin/
    └── registry.tsx      # 自定义Renderer注册表
```

### 包示例

```
packs/
├── prom/                 # Prometheus集成
├── player/               # 玩家管理
├── http/                 # 通用HTTP调用
├── grafana/              # Grafana集成
└── alertmanager/         # AlertManager集成

每个包包含:
├── manifest.json         # 包清单
├── descriptors/          # 函数描述符
└── ui/                   # UI Schema
```

## 🔑 核心概念

### 1. 描述符 (Descriptor)

**用途**: 完整定义一个函数的接口、权限、UI和输出

**关键字段**:
- `id`: 函数唯一标识 (e.g., "player.ban")
- `version`: 语义版本
- `category`: 分类 (player, item, economy)
- `risk`: 风险级别 (low|medium|high)
- `auth`: 权限配置 (permission, allow_if, require_approval)
- `params`: JSON Schema (请求参数定义)
- `outputs`: 输出视图定义 (views, layout, transforms)
- `semantics`: 语义信息 (mode, route, timeout)

**存储方式**:
- 文件存储: `packs/*/descriptors/*.json`
- 内存索引: `Server.descIndex[functionID]`

### 2. 注册表 (Registry)

**作用**: 管理Agent会话和函数覆盖率

**核心数据结构**:
```
agents: map[agentID] → AgentSession {
  AgentID, GameID, Env, RPCAddr, Functions, ExpireAt
}

provCaps: map[providerID] → ProviderCaps {
  ID, Version, Lang, SDK, Manifest, UpdatedAt
}
```

**关键特性**:
- 内存存储 (快速响应)
- 会话过期管理 (TTL)
- 覆盖率统计 (健康/总数)
- Provider manifest聚合

### 3. 函数包 (Pack)

**结构**: manifest + descriptors + ui + web-plugins

**管理操作**:
- `InstallComponent`: 安装包 (验证依赖)
- `UninstallComponent`: 卸载包 (检查反向依赖)
- `EnableComponent`/`DisableComponent`: 启用/禁用
- `ListInstalled`/`ListByCategory`: 列表查询

**版本控制**: ETag体现包内容版本

### 4. 函数调用路由

| 路由模式 | 说明 | 用途 |
|----------|------|------|
| `lb` | 负载均衡 | 默认，自动选择健康实例 |
| `broadcast` | 广播 | 调用所有实例 |
| `targeted` | 指定 | 指定特定服务ID |
| `hash` | 一致性哈希 | 基于hashKey分发 |

## 🔗 数据流

### 同步调用流程

```
1. 用户选择函数 → 填充参数 → 点击Invoke
2. GmFunctions 页面 → POST /api/invoke
3. Server 处理:
   - 认证 (JWT/mTLS)
   - 参数验证 (JSON Schema)
   - 权限检查 (RBAC + allow_if)
   - 维护状态检查
   - 审计日志
4. 转发至 Agent (gRPC)
5. Agent 执行函数
6. 返回结果 → 前端渲染
```

### 异步任务流程

```
1. POST /api/start_job → 返回 job_id
2. GET /api/stream_job?id=jobId → SSE连接
3. Agent 后台执行，定期报告:
   - progress (0-100)
   - log (消息)
   - done/error (最终状态)
4. Server 转发事件至前端
5. 前端实时显示进度
```

## 🛡️ 权限与安全

### RBAC权限模型

```
权限查询链:
1. 函数自定义权限 desc.Auth["permission"]
   否则默认: "function:{functionID}"

2. 权限匹配:
   - "game:{gameID}:function:{functionID}" (游戏级)
   - "function:{functionID}" (全局)
   - "game:{gameID}:*" (游戏通配符)
   - "*" (超级权限)

3. 条件表达式 allow_if:
   allow_if: "roles.includes('admin') && env == 'prod'"

4. 审批流程 (require_approval):
   create → pending → approved/rejected → execute

5. 两人规则 (two_person_rule):
   请求者 + 批准者 + 审计记录
```

### 参数验证

使用JSON Schema进行多层次验证:
- 类型检查 (string, integer, object, array)
- 长度限制 (minLength, maxLength)
- 数值范围 (minimum, maximum)
- 正则匹配 (pattern)
- 枚举检查 (enum)
- 递归验证 (nested objects)

### 审计日志

```json
{
  "action": "invoke",
  "user": "admin-user",
  "function_id": "player.ban",
  "timestamp": "ISO8601",
  "trace_id": "unique-id",
  "game_id": "game1",
  "env": "prod",
  "payload_snapshot": "masked",  // 敏感字段已掩码
  "result": "success|failure"
}
```

## 🎨 Web前端

### 三种表单渲染模式

| 模式 | 特点 | 场景 |
|------|------|------|
| Enhanced | show_if, required_if, 分组, 选项卡 | 推荐，复杂表单 |
| Form-Render | 独立库，复杂schema支持好 | 超复杂表单 |
| Legacy | 基础Ant Design Form | 简单表单 |

### 结果可视化

支持多种Renderer:
- `json.view`: JSON树形展示
- `table.basic`: 基础表格
- `echarts.bar`: 柱状图
- `echarts.line`: 折线图
- `custom.*`: 自定义渲染器

支持数据变换:
- JSONPath表达式
- 模板渲染
- forEach循环
- 字段映射

## 📊 关键API端点

### 描述符相关

```
GET /api/descriptors              # 获取所有描述符
GET /api/descriptors?detailed=true # 详细模式 (含provider)
GET /api/ui_schema?id=function_id # 获取UI Schema
```

### 函数调用

```
POST /api/invoke              # 同步调用
POST /api/start_job           # 异步启动
GET  /api/stream_job?id=...   # SSE流监听
POST /api/cancel_job          # 取消任务
GET  /api/function_instances  # 列出实例
```

### 注册表

```
GET /api/registry             # 获取代理和覆盖率信息
```

### 包管理

```
GET  /api/packs/list          # 列出包信息
POST /api/packs/import        # 导入包
GET  /api/packs/export        # 导出所有包 (tar.gz)
POST /api/packs/reload        # 重新加载包
```

### Provider能力

```
POST /api/providers/capabilities   # 上传Provider manifest
GET  /api/providers/descriptors    # 列出Provider能力
GET  /api/providers/entities       # 列出Provider实体
```

## 🚀 扩展指南

### 创建新的函数包

```
1. 创建目录结构:
   my-pack/
   ├── manifest.json
   ├── descriptors/
   │   └── my_func.json
   └── ui/
       └── my_func.uischema.json

2. 定义manifest.json:
   {
     "functions": [
       { "id": "my.func", "version": "1.0.0" }
     ]
   }

3. 定义函数描述符:
   {
     "id": "my.func",
     "params": { /* JSON Schema */ },
     "outputs": { /* 视图定义 */ }
   }

4. (可选) 定义UI Schema:
   {
     "fields": { /* UI配置 */ },
     "ui:groups": [ /* 分组 */ ]
   }
```

### 注册自定义Renderer

```typescript
// web/src/plugin/registry.tsx
registerRenderer('my.renderer', (props) => {
  return <MyComponent data={props.data} options={props.options} />;
});
```

### 上传Provider Manifest

```
POST /api/providers/capabilities
{
  "provider": {
    "id": "my-sdk",
    "version": "1.0.0",
    "lang": "python",
    "sdk": "my-croupier"
  },
  "manifest_json": {
    "provider": { /* ... */ },
    "functions": [ /* ... */ ],
    "entities": [ /* ... */ ]
  }
}
```

## 📈 性能考虑

### 内存使用

- **描述符**: 按包加载，全部存入内存 (indexed by functionID)
- **注册表**: 内存存储，支持并发读写
- **Provider manifests**: 原始JSON存储，解析时反序列化

### 缓存策略

- **描述符**: 启动时加载，支持热重载
- **注册表**: 实时更新 (UpsertAgent)
- **包信息**: ETag版本控制

### 并发处理

- **注册表**: 使用RWMutex保护
- **HTTP请求**: Gin框架处理并发
- **gRPC调用**: 连接池复用

## 🔍 调试技巧

### 查看活跃代理

```
GET /api/registry
查看agents列表和健康状态
```

### 检查函数覆盖率

```
GET /api/registry
在coverage字段中查看未覆盖的函数
```

### 获取描述符详情

```
GET /api/descriptors
或
GET /api/descriptors?detailed=true (含provider)
```

### 验证包完整性

```
GET /api/packs/list
检查descriptors和ui_schema计数
```

### 查看审计日志

系统记录所有函数调用，包括:
- 请求用户和IP
- Trace ID关联
- 参数快照 (敏感字段已掩码)
- 执行结果

## 📚 相关文档

- **FUNCTION_ARCHITECTURE.md**: 完整的架构分析
- **FUNCTION_COMPONENTS_DEEP_DIVE.md**: 各组件深度实现
- **CLAUDE.md**: 项目开发指南
- **docs/providers-manifest.schema.json**: Provider manifest JSON Schema

## 🎯 快速开始

### 查看现有函数

```bash
# 访问GmFunctions页面
http://localhost:8080/pages/GmFunctions

# 或API查询
curl http://localhost:8080/api/descriptors
```

### 监控代理健康状态

```bash
# 访问Registry页面
http://localhost:8080/pages/Registry

# 或API查询
curl http://localhost:8080/api/registry
```

### 管理函数包

```bash
# 访问Packs页面
http://localhost:8080/pages/Packs

# 或API查询
curl http://localhost:8080/api/packs/list
```

## 📝 总结

Croupier的函数管理系统通过以下设计实现了高度灵活性和可扩展性:

1. **描述符驱动**: 单一数据源驱动整个生态
2. **多源聚合**: 统一管理legacy和现代Provider
3. **分层架构**: 清晰的责任分工
4. **权限集中化**: 灵活的RBAC + 条件表达式
5. **可视化友好**: 从Schema自动生成UI
6. **完整可观测性**: 审计、Trace ID、覆盖率统计

开发者只需定义JSON描述符，就能自动获得完整的UI、验证、权限管理和可视化能力！

---

**文档维护**: 2024-11-13
**项目**: Croupier Game Management Platform
**相关技术**: Go, React, Protocol Buffers, JSON Schema
