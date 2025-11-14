# Croupier 项目函数管理系统架构分析

## 一、核心架构概览

Croupier实现了一个**描述符驱动(Descriptor-Driven)** 的函数管理系统，由以下关键层级组成：

```
┌─────────────────────────────────────────────────────────────┐
│                      Web UI 前端                             │
│  GmFunctions | Registry | Packs 页面                         │
└────────────────────┬────────────────────────────────────────┘
                     │ HTTP REST API
┌────────────────────▼────────────────────────────────────────┐
│                   HTTP 服务器                                │
│  Server (internal/app/server/http/)                          │
│  ├─ Function API (/api/descriptors, /api/invoke)            │
│  ├─ Registry API (/api/registry)                            │
│  ├─ Packs API (/api/packs/*)                                │
│  └─ Provider Capabilities (/api/providers/*)                │
└────────────────────┬────────────────────────────────────────┘
                     │
    ┌────────────────┼────────────────┐
    │                │                │
┌───▼──────┐  ┌──────▼─────┐  ┌──────▼──────┐
│Descriptor│  │  Registry  │  │  Pack Mgmt  │
│ Store    │  │  Store     │  │  System     │
│          │  │(AgentSesSn)│  │             │
└──────────┘  └────────────┘  └─────────────┘
```

---

## 二、关键组件详解

### 2.1 描述符管理系统 (Descriptor System)

#### 位置
- **核心加载逻辑**: `internal/function/descriptor/loader.go`
- **Proto定义**: 
  - `proto/croupier/function/v1/function.proto` (Function Service)
  - `proto/croupier/control/v1/control.proto` (Control/Registration)

#### 核心结构
```go
// Descriptor 是简化的函数描述符模型，用于UI/验证
type Descriptor struct {
    ID        string         `json:"id"`        // 函数标识，如 "player.ban"
    Version   string         `json:"version"`   // 语义版本，如 "1.2.0"
    Category  string         `json:"category"`  // 分组，如 "player", "item"
    Risk      string         `json:"risk"`      // 风险等级 "low|medium|high"
    Auth      map[string]any `json:"auth"`      // 权限配置
    Params    map[string]any `json:"params"`    // JSON Schema 参数定义
    Semantics map[string]any `json:"semantics"`// 语义信息 (mode, route, timeout)
    Transport map[string]any `json:"transport"`// 传输配置
    Outputs   map[string]any `json:"outputs"`  // 输出定义 (views/layout)
    UI        map[string]any `json:"ui"`       // UI相关配置
}
```

#### 加载机制
```go
// LoadAll(dir) 递归扫描目录，加载所有 *.json 描述符文件
// 过滤规则：
// 1. 排除 "ui" 子目录下的JSON文件
// 2. 必须有非空的 "id" 字段
// 3. 返回有序的 []*Descriptor 切片
```

#### 关键特性
- **动态加载**: 运行时从文件系统加载描述符
- **JSON Schema 验证**: 使用 JSON Schema 验证函数参数
- **多源聚合**: 支持legacy描述符 + Provider manifest统一视图

---

### 2.2 注册表系统 (Registry System)

#### 位置
- **核心存储**: `internal/platform/registry/store.go`
- **HTTP接口**: `internal/app/server/http/server.go:7248` (`/api/registry`)

#### 核心数据结构
```go
// AgentSession 代表已注册的代理实例
type AgentSession struct {
    AgentID   string                  // 代理唯一标识
    GameID    string                  // 游戏作用域
    Env       string                  // 环境 (prod/stage/test)
    RPCAddr   string                  // 代理可达的 gRPC 地址
    Version   string                  // 代理版本
    Region    string                  // 区域信息
    Zone      string                  // 可用区
    Labels    map[string]string       // 自定义标签
    Functions map[string]FunctionMeta // 函数能力列表
    ExpireAt  time.Time               // 会话过期时间
}

// ProviderCaps 代表 Provider manifest 快照
type ProviderCaps struct {
    ID        string    // Provider ID
    Version   string    // Provider版本
    Lang      string    // 编程语言 (go, python, java...)
    SDK       string    // SDK名称
    Manifest  []byte    // 原始JSON清单
    UpdatedAt time.Time // 更新时间
}

// FunctionMeta 描述函数在代理上的能力
type FunctionMeta struct {
    Enabled bool // 函数是否启用
}
```

#### 注册表存储设计
```go
// Store 在内存中保持轻量级代理注册表状态
type Store struct {
    mu       sync.RWMutex
    agents   map[string]*AgentSession      // agent_id -> session
    provCaps map[string]ProviderCaps        // provider_id -> caps
}
```

#### 关键操作
```
UpsertAgent(session)          // 插入或更新代理会话
UpsertProviderCaps(caps)      // 插入或更新Provider能力
ListProviderCaps()            // 列出所有Provider能力
BuildUnifiedDescriptors()     // 合并所有Provider的manifest
```

#### 覆盖率计算
```go
// 注册表为每个 game_env 构建覆盖率统计：
Coverage {
    GameEnv    string              // "game_id|env"
    Functions  map[string]FuncCov  // 函数ID -> 健康/总数
    Uncovered  []string            // 未覆盖的函数列表
}
```

---

### 2.3 函数包系统 (Pack System)

#### 位置
- **包管理器**: `internal/pack/manager.go`
- **类型注册表**: `internal/pack/typereg.go`
- **HTTP路由**: `internal/app/server/http/server.go:3205-3316`

#### 包结构
```
packs/
├── prom/                          # 示例包: Prometheus集成
│   ├── manifest.json              # 包清单 (函数列表, plugins)
│   ├── descriptors/               # 函数描述符
│   │   ├── prom.query.json        # 查询函数描述符
│   │   └── prom.query_range.json
│   └── ui/                        # UI Schema和配置
│       ├── prom.query.schema.json
│       └── prom.query.uischema.json
├── player/                        # 示例包: 玩家管理
│   ├── manifest.json
│   ├── descriptors/
│   │   ├── player.ban.json
│   │   └── player.unban.json
│   └── ui/
├── http/                          # 示例包: 通用HTTP调用
├── grafana/                       # 示例包: Grafana集成
└── alertmanager/                  # 示例包: AlertManager集成
```

#### 包Manifest结构
```json
{
  "functions": [
    { "id": "prom.query", "version": "1.0.0", "category": "prom" },
    { "id": "prom.query_range", "version": "1.0.0", "category": "prom" }
  ],
  "web_plugins": [
    "web-plugin/echarts_plugin.js"
  ]
}
```

#### 描述符结构示例 (prom.query.json)
```json
{
  "id": "prom.query",
  "version": "1.0.0",
  "category": "prom",
  "risk": "low",
  "auth": { "permission": "prom.query" },
  "params": {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "type": "object",
    "properties": {
      "expr": { "type": "string" },
      "time": { "type": "string", "format": "date-time" }
    },
    "required": ["expr"]
  },
  "semantics": { "mode": "query", "route": "lb", "timeout": "30s" },
  "outputs": {
    "views": [
      { "id": "json", "type": "json", "renderer": "json.view" },
      { "id": "table", "type": "table", "renderer": "table.basic", 
        "show_if": "$.data.result",
        "transform": { /* data transformation rules */ }
      }
    ]
  }
}
```

#### 包管理操作
```go
// ComponentManager 负责函数组件的CRUD
InstallComponent(path)      // 安装新包 (验证依赖)
UninstallComponent(id)      // 卸载包 (检查反向依赖)
EnableComponent(id)         // 启用包
DisableComponent(id)        // 禁用包
ListInstalled()             // 列出已安装的包
ListByCategory(cat)         // 按分类列表
```

#### 类型注册表 (TypeRegistry)
```go
// 用于动态的Protocol Buffer编解码
LoadFDS(bytes)              // 加载 FileDescriptorSet
LoadFDSFromDir(dir)         // 从目录加载所有 *.pb 文件
JSONToProtoBin(typeFQN)     // JSON → Protocol Buffer二进制
ProtoBinToJSON(typeFQN)     // Protocol Buffer二进制 → JSON
```

#### 包导出/导入
```
GET  /api/packs/list        # 列出包信息 (manifest, 计数, ETag)
POST /api/packs/import      # 导入新包
GET  /api/packs/export      # 导出所有包为 tar.gz
POST /api/packs/reload      # 重新加载包
```

---

### 2.4 函数调用流程

#### API端点
```
POST /api/invoke              # 同步调用函数
POST /api/start_job           # 异步启动任务
GET  /api/stream_job?id=...   # SSE 流式获取任务事件
POST /api/cancel_job          # 取消任务
GET  /api/function_instances  # 列出函数实例
```

#### 调用请求结构
```go
type InvokeRequest struct {
    FunctionID       string            // 函数ID，如 "player.ban"
    Payload          any               // 请求负载 (JSON格式)
    IdempotencyKey   string            // 幂等性密钥
    Route            string            // 路由模式: lb|broadcast|targeted|hash
    TargetServiceID  string            // 目标服务ID (route=targeted时)
    HashKey          string            // 哈希键 (route=hash时)
}
```

#### 权限检查流程
```
1. 身份认证 (JWT/mTLS)
2. 获取RBAC角色
3. 权限查询：
   - 优先使用函数的自定义权限 (desc.Auth.permission)
   - 默认权限: "function:{function_id}"
   - 作用域权限: "game:{game_id}:function:{function_id}"
   - 通配符: "game:{game_id}:*"
4. allow_if 表达式求值 (可选)
5. 维护状态检查 (isWriteBlocked)
```

#### 参数验证
```go
// 使用 JSON Schema 验证请求负载
if d := s.descIndex[functionID]; d != nil && d.Params != nil {
    if err := validation.ValidateJSON(d.Params, payloadBytes) {
        // 返回 400 Bad Request
    }
}
```

#### 审计日志
```go
// 记录调用事件 (敏感字段已掩码)
s.audit.Log("invoke", user, functionID, map[string]string{
    "ip":              clientIP,
    "trace_id":        randHex(8),
    "game_id":         gameID,
    "env":             env,
    "payload_snapshot": masked, // 敏感数据已掩码处理
})
```

---

## 三、Web前端系统

### 3.1 页面结构

#### GmFunctions 页面 (`web/src/pages/GmFunctions/index.tsx`)
**功能**: 函数调用工作台

**核心功能**:
- 按游戏/环境选择函数
- 动态表单渲染 (3种模式: enhanced, form-render, legacy)
- 参数验证与填充
- 多种路由策略 (lb, broadcast, targeted, hash)
- 实时任务流监听 (SSE)
- 结果可视化 (views渲染)

**状态管理**:
```tsx
const [descs, setDescs]           = useState<FunctionDescriptor[]>([])
const [currentId, setCurrentId]   = useState<string>()
const [route, setRoute]           = useState<'lb'|'broadcast'|'targeted'|'hash'>('lb')
const [instances, setInstances]   = useState<AgentInstance[]>([])
const [jobId, setJobId]           = useState<string>()
const [uiSchema, setUiSchema]     = useState<UISchema>()
const [renderMode, setRenderMode] = useState<'form-render'|'enhanced'|'legacy'>('enhanced')
```

**表单渲染引擎**:
1. **Enhanced UI** (推荐): XUISchema 扩展支持 show_if, required_if, 分组, 选项卡
2. **Form-Render**: 独立库，支持复杂schema
3. **Legacy**: 基础Ant Design Form

#### Registry 页面 (`web/src/pages/Registry/index.tsx`)
**功能**: 代理和函数覆盖率仪表盘

**显示内容**:
- 活跃Agent列表 (包括IP, 版本, 函数数, 健康状态, 过期时间)
- 按函数统计覆盖率 (健康/总数)
- 未覆盖函数列表
- 按前缀分组显示

**过滤选项**:
- 仅显示未覆盖的函数
- 仅显示部分覆盖的函数
- 按游戏/环境筛选
- 多种排序策略

#### Packs 页面 (`web/src/pages/Packs/index.tsx`)
**功能**: 函数包管理

**功能**:
- 查看包Manifest信息
- 统计描述符和UI Schema数量
- ETag版本控制
- 重新加载包
- 导出为tar.gz文件
- 权限检查 (packs:reload, packs:export)

### 3.2 API服务层

#### 函数服务 (`web/src/services/croupier/functions.ts`)
```typescript
listDescriptors()                    // GET /api/descriptors
invokeFunction(id, payload, opts)    // POST /api/invoke
startJob(id, payload, opts)          // POST /api/start_job
cancelJob(jobId)                     // POST /api/cancel_job
fetchJobResult(id)                   // GET /api/job_result
listFunctionInstances(params)        // GET /api/function_instances
```

#### 注册表服务 (`web/src/services/croupier/registry.ts`)
```typescript
fetchRegistry()  // GET /api/registry
                 // 返回: { agents, functions, assignments, coverage }
```

#### 包服务 (`web/src/services/croupier/packs.ts`)
```typescript
listPacks()      // GET /api/packs/list
reloadPacks()    // POST /api/packs/reload
// 导出通过直接URL: /api/packs/export
```

### 3.3 UI Schema 系统

#### UISchema 结构
```typescript
type UISchema = {
  fields?: Record<string, XUISchemaField>;
  'ui:layout'?: { type?: 'grid' | 'tabs'; cols?: number };
  'ui:groups'?: Array<{ title?: string; fields: string[] }>;
};

type XUISchemaField = {
  label?: string;
  placeholder?: string;
  widget?: 'textarea' | 'date' | 'time' | 'select';
  enum?: string[];
  'x-enum-labels'?: Record<string, string>;
  show_if?: string;           // 条件显示: "$.field == 'value'"
  required_if?: string;       // 条件必填: "$.other_field"
};
```

#### 数据获取
```typescript
// UI Schema 通过单独API获取
fetch(`/api/ui_schema?id=${encodeURIComponent(functionId)}`)
  .then(resp => resp.json())
  .then(json => setUiSchema(json.uischema || json.uiSchema))
```

---

## 四、后端HTTP服务器实现

### 4.1 服务器初始化 (`internal/app/server/http/server.go`)

**关键字段**:
```go
type Server struct {
    descs           []*descriptor.Descriptor          // 描述符列表
    descIndex       map[string]*descriptor.Descriptor // 快速查找索引
    packDir         string                            // 包目录路径
    reg             *registry.Store                   // 注册表存储
    rbac            rbac.Policy                       // RBAC策略
    audit           audit.Store                       // 审计日志
    statsProv       StatsProvider                     // 统计信息
    // ... 其他字段
}
```

### 4.2 核心API路由 (server.go)

#### 描述符 API
```go
GET /api/descriptors
    // 返回所有函数描述符
    // ?detailed=true 时返回 { legacy_descriptors, provider_manifests }
```

#### 调用 API
```go
POST /api/invoke
    Request: {
        function_id: string,
        payload: any,
        idempotency_key?: string,
        route?: 'lb'|'broadcast'|'targeted'|'hash',
        target_service_id?: string,
        hash_key?: string
    }
    Response: any (函数的返回值)
```

#### 任务 API
```go
POST /api/start_job
    Request: 同 /api/invoke
    Response: { job_id: string }

GET /api/stream_job?id=<job_id>
    SSE 流: { type, message, progress, payload }
    事件: progress|log|done|error

POST /api/cancel_job
    Request: { job_id: string }
```

#### 注册表 API
```go
GET /api/registry
    Response: {
        agents: Agent[],
        functions: Function[],
        assignments?: Record<game_env, string[]>,
        coverage?: Coverage[]
    }
```

#### 包 API
```go
GET /api/packs/list
    Response: { manifest, counts, etag, export_auth_required }

POST /api/packs/import
    上传新的包 (tar.gz)

GET /api/packs/export
    Response: tar.gz (所有包)

POST /api/packs/reload
    重新加载包目录
```

#### Provider API
```go
POST /api/providers/capabilities
    上传 Provider manifest (HTTP接口)

GET /api/providers/descriptors
    列出所有 Provider 能力

GET /api/providers/entities
    聚合所有 Provider 的 entities
```

---

## 五、数据流与调用链

### 5.1 同步函数调用流程

```
User (Web UI)
    ↓ [选择函数，填充参数]
GmFunctions Page
    ↓ POST /api/invoke { function_id, payload }
Server.POST /api/invoke
    ├─ 身份认证 (auth)
    ├─ RBAC权限检查
    ├─ 参数验证 (JSON Schema)
    ├─ 审计日志
    ├─ 负载均衡/路由 (lb|broadcast|targeted|hash)
    └─ 转发至 Agent gRPC
Agent
    ├─ 查找本地注册的函数
    ├─ 执行函数
    └─ 返回结果
Server
    ├─ 接收响应
    └─ 返回客户端
GmFunctions Page
    ├─ 解析响应
    ├─ 应用视图变换 (transform)
    └─ 渲染结果
```

### 5.2 异步任务调用流程

```
User (Web UI)
    ↓ [点击 Start Job]
GmFunctions Page
    ↓ POST /api/start_job
Server.POST /api/start_job
    └─ 创建Job, 返回 job_id
GmFunctions Page
    ├─ 打开 EventSource: GET /api/stream_job?id={job_id}
    └─ 监听事件: progress|log|done|error
Agent (后台执行)
    ├─ 启动异步任务
    └─ 定期报告进度
Server
    ├─ 接收Agent进度事件
    └─ 通过SSE转发至客户端
GmFunctions Page
    ├─ 实时显示进度
    ├─ 收集日志消息
    ├─ 最终结果渲染
    └─ 关闭EventSource (done/error)
```

### 5.3 函数注册流程

```
Agent
    ├─ 启动时扫描本地函数
    └─ 调用 ControlService.Register
Server.ControlService.Register
    ├─ 验证Agent身份
    ├─ 保存到Registry
    │   └─ UpsertAgent(AgentSession{
    │       AgentID, GameID, Env, RPCAddr, Functions...
    │   })
    └─ 返回 session_id, expire_at
Agent
    ├─ 周期性心跳 Heartbeat
    └─ 保持会话活跃
```

---

## 六、关键设计特性

### 6.1 描述符驱动架构

**核心理念**:
- 单一数据源 (描述符) 驱动整个系统
- 包含参数定义、权限、UI配置、语义、输出规则
- 支持多源聚合 (legacy + Provider manifest)

**优势**:
1. **UI自动生成**: 从JSON Schema自动生成表单
2. **验证集中化**: 参数验证、权限检查由描述符驱动
3. **输出变换**: 通过transform规则自动转换函数输出
4. **版本管理**: 描述符版本化便于演化

### 6.2 多源聚合系统

**描述符来源**:
1. **Legacy描述符**: 从 `packs/*/descriptors/` 加载的JSON文件
2. **Provider Manifest**: 通过HTTP或Control Service上传的Provider清单
3. **统一视图**: `/api/descriptors?detailed=true` 返回两种格式

**Manifest示例**:
```json
{
  "provider": { "id": "go-sdk", "version": "1.0.0", "lang": "go" },
  "functions": [ /* 函数列表 */ ],
  "entities": [ /* 实体定义 */ ],
  "operations": [ /* 操作定义 */ ]
}
```

### 6.3 权限与审计

**权限模型**:
- RBAC (基于角色) + ABAC (基于属性)
- 函数级权限 + 游戏级作用域
- 条件表达式 (allow_if)

**审计追踪**:
- 每次调用都记录
- 敏感字段掩码处理
- Trace ID 关联

### 6.4 输出可视化

**Views系统**:
```json
{
  "outputs": {
    "views": [
      {
        "id": "chart",
        "type": "chart",
        "renderer": "echarts.bar",
        "show_if": "$.data",
        "transform": { /* 数据变换规则 */ }
      }
    ]
  }
}
```

**支持的Renderer**:
- `json.view` - JSON树形展示
- `table.basic` - 基础表格
- `echarts.bar` - ECharts柱状图
- `echarts.line` - ECharts折线图
- 可通过插件扩展

### 6.5 路由策略

```
route='lb'          → 负载均衡 (轮询/一致性哈希)
route='broadcast'   → 广播所有实例
route='targeted'    → 指定目标服务ID
route='hash'        → 基于hashKey的一致性哈希
```

---

## 七、数据存储设计

### 7.1 注册表存储 (内存)

```go
agents map[string]*AgentSession      // key: agent_id
provCaps map[string]ProviderCaps     // key: provider_id
```

**特点**:
- 内存存储 (快速, 无持久化)
- 会话过期管理 (TTL)
- 读写锁保护并发访问

### 7.2 包存储 (文件系统)

```
packDir/
├── manifest.json           // 总清单 (所有包)
├── descriptors/            # 所有函数描述符
│   ├── prom.query.json
│   ├── player.ban.json
│   └── ...
└── ui/                     # 所有UI Schema
    ├── prom.query.uischema.json
    └── ...
```

**特点**:
- 文件系统存储 (持久化)
- 可以导出为tar.gz
- ETag版本控制

### 7.3 描述符索引 (内存)

```go
descIndex map[string]*descriptor.Descriptor  // key: function_id
```

**用途**:
- 快速查找函数描述符
- 参数验证、权限检查

---

## 八、现有页面与组件清单

### 前端页面
| 页面 | 路径 | 功能 |
|------|------|------|
| GmFunctions | `/pages/GmFunctions/index.tsx` | 函数调用工作台 |
| Registry | `/pages/Registry/index.tsx` | 代理和覆盖率仪表盘 |
| Packs | `/pages/Packs/index.tsx` | 包管理界面 |

### 服务模块
| 模块 | 路径 | 功能 |
|------|------|------|
| functions.ts | `/services/croupier/functions.ts` | 函数API |
| registry.ts | `/services/croupier/registry.ts` | 注册表API |
| packs.ts | `/services/croupier/packs.ts` | 包API |

### 核心库
| 库 | 位置 | 作用 |
|----|------|------|
| descriptor | `internal/function/descriptor/` | 描述符加载与管理 |
| registry | `internal/platform/registry/` | 代理注册表 |
| pack | `internal/pack/` | 包管理 (manager.go, typereg.go) |

---

## 九、关键接口定义

### 9.1 Proto Interfaces

```proto
// 函数调用服务
service FunctionService {
  rpc Invoke(InvokeRequest) returns (InvokeResponse);
  rpc StartJob(InvokeRequest) returns (StartJobResponse);
  rpc StreamJob(JobStreamRequest) returns (stream JobEvent);
  rpc CancelJob(CancelJobRequest) returns (StartJobResponse);
}

// 控制服务 (Agent注册)
service ControlService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Heartbeat(HeartbeatRequest) returns (HeartbeatResponse);
  rpc RegisterCapabilities(RegisterCapabilitiesRequest) 
    returns (RegisterCapabilitiesResponse);
}
```

### 9.2 Go Interfaces

```go
// 描述符加载器
func LoadAll(dir string) ([]*Descriptor, error)

// 注册表操作
func (s *Store) UpsertAgent(a *AgentSession)
func (s *Store) UpsertProviderCaps(c ProviderCaps)
func (s *Store) BuildUnifiedDescriptors() map[string]interface{}

// 包管理
func (cm *ComponentManager) InstallComponent(path string) error
func (cm *ComponentManager) UninstallComponent(id string) error
func (cm *ComponentManager) ListInstalled() map[string]*ComponentManifest
```

---

## 十、扩展点与定制化

### 10.1 自定义函数包
创建新包只需遵循目录结构:
```
my-pack/
├── manifest.json           # 声明包元数据
├── descriptors/            # 函数描述符
│   └── my_func.json
└── ui/                     # UI Schema
    └── my_func.uischema.json
```

### 10.2 自定义UI Renderer
```typescript
// 在 web/src/plugin/registry.tsx 中注册
registerRenderer('my.renderer', (props) => {
  return <MyCustomRenderer data={props.data} />;
});
```

### 10.3 自定义数据变换
```json
{
  "transform": {
    "expr": "$.results",           // 路径表达式
    "template": { /* 变换规则 */ } // 模板渲染
  }
}
```

---

## 总结

Croupier的函数管理系统通过以下设计实现了高度灵活性和可扩展性:

1. **描述符驱动**: 单一数据源驱动UI、验证、权限、审计
2. **多源聚合**: 支持legacy + Provider manifest统一管理
3. **分层架构**: Web UI → HTTP API → Registry → Agent → Function
4. **权限集中化**: RBAC + 条件表达式的灵活权限模型
5. **可视化友好**: 动态表单生成、输出转换、多种渲染器支持
6. **可观测性**: 完整的审计链、Trace ID、覆盖率统计

这个架构使得开发者可以只需定义JSON描述符, 就能自动获得完整的UI、验证、权限管理和输出展示能力。
