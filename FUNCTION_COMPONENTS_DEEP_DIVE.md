# Croupier 函数管理系统 - 组件深度分析

## 一、描述符加载器详细实现

### 1.1 核心加载逻辑 (loader.go)

```go
// 关键特性：递归扫描，JSON解析，ID验证

type Descriptor struct {
    ID        string         `json:"id"`        // 唯一标识 (必需)
    Version   string         `json:"version"`   // 语义版本
    Category  string         `json:"category"`  // 分类 (player, item, economy)
    Risk      string         `json:"risk"`      // 风险级别: low|medium|high
    Auth      map[string]any `json:"auth"`      // 权限配置
    Params    map[string]any `json:"params"`    // JSON Schema (请求参数)
    Semantics map[string]any `json:"semantics"`// 语义: mode, route, timeout
    Transport map[string]any `json:"transport"`// 传输配置
    Outputs   map[string]any `json:"outputs"`  // 输出定义 (views, layout)
    UI        map[string]any `json:"ui"`       // UI配置
}

// LoadAll(dir) 实现流程：
func LoadAll(dir string) ([]*Descriptor, error) {
    var out []*Descriptor
    
    // 步骤1：使用filepath.WalkDir递归遍历
    err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
        
        // 步骤2：过滤规则
        if d.IsDir() { return nil }                    // 跳过目录
        if filepath.Ext(path) != ".json" { return nil }// 只处理.json
        if filepath.Base(filepath.Dir(path)) == "ui" { // 排除ui子目录
            return nil
        }
        
        // 步骤3：读取并预检查
        b, err := os.ReadFile(path)
        if err != nil { return err }
        
        // 步骤4：快速检查 (是否有"id"字段)
        var probe map[string]any
        if err := json.Unmarshal(b, &probe); err != nil {
            return nil // 忽略解析错误
        }
        v, ok := probe["id"]
        if !ok || v == nil || (v.(string) == "") {
            return nil // ID不存在或为空
        }
        
        // 步骤5：完整解析
        var desc Descriptor
        if err := json.Unmarshal(b, &desc); err != nil {
            return nil // 解析错误时跳过
        }
        
        // 步骤6：最终验证 (ID必须非空)
        if desc.ID == "" {
            return nil
        }
        
        out = append(out, &desc)
        return nil
    })
    
    return out, err
}
```

### 1.2 描述符的关键字段解析

#### auth 字段 (权限配置)
```json
{
  "auth": {
    "permission": "player.ban",        // 权限码 (如未指定则为 function:{id})
    "allow_if": "roles.includes('admin')",  // 条件表达式
    "require_approval": true,          // 是否需要审批
    "two_person_rule": true            // 两人规则
  }
}
```

#### params 字段 (JSON Schema参数)
```json
{
  "params": {
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "type": "object",
    "properties": {
      "player_id": { "type": "string", "minLength": 1 },
      "reason": { "type": "string" },
      "duration": { "type": "integer", "minimum": 1 }
    },
    "required": ["player_id", "reason"],
    "additionalProperties": false
  }
}
```

#### semantics 字段 (语义配置)
```json
{
  "semantics": {
    "mode": "write",              // read|write|batch
    "route": "targeted",          // lb|broadcast|targeted|hash
    "timeout": "30s",             // 超时时间
    "idempotency": "required"     // 幂等性要求
  }
}
```

#### outputs 字段 (输出和可视化)
```json
{
  "outputs": {
    "views": [
      {
        "id": "result",
        "type": "json",
        "renderer": "json.view",
        "show_if": "$.success",           // 条件显示
        "transform": {
          "expr": "$.data",              // JSONPath表达式
          "template": { /* 变换规则 */ } // 模板渲染
        }
      }
    ],
    "layout": { "type": "grid", "cols": 2 }
  }
}
```

---

## 二、注册表系统详解

### 2.1 AgentSession 生命周期

```go
// 注册流程
Agent.Register() 
    → Server.ControlService.Register(RegisterRequest{
        AgentID: "agent-001",
        GameID: "game1",
        Env: "prod",
        RPCAddr: "10.0.1.1:19090",
        Functions: [
            { id: "player.ban", version: "1.0.0" },
            { id: "player.mute", version: "1.0.0" }
        ]
    })
    → Server.Registry.UpsertAgent(AgentSession{
        AgentID: "agent-001",
        GameID: "game1",
        Env: "prod",
        RPCAddr: "10.0.1.1:19090",
        Version: "v1.2.3",
        Functions: {
            "player.ban": { Enabled: true },
            "player.mute": { Enabled: true }
        },
        ExpireAt: time.Now().Add(30s) // TTL
    })
    ← RegisterResponse{ SessionID, ExpireAt }

// 心跳流程
Agent.Heartbeat(HeartbeatRequest{
    AgentID: "agent-001",
    SessionID: "sess-xxx"
}) 
    → Server.Registry.UpdateExpire("agent-001", 30s)
```

### 2.2 覆盖率计算 (Coverage)

```go
// 覆盖率统计逻辑

type FuncCov struct {
    Healthy int  // 健康的代理数
    Total   int  // 总代理数
}

type Coverage struct {
    GameEnv   string               // "game1|prod"
    Functions map[string]FuncCov   // 按函数统计
    Uncovered []string             // 未覆盖的函数
}

// 计算过程：
func BuildCoverage() Coverage {
    // 1. 遍历所有代理和函数
    for _, agent := range agents {
        for funcID := range agent.Functions {
            // 2. 统计健康/总数
            if now.Before(agent.ExpireAt) {
                coverage[funcID].Healthy++
            }
            coverage[funcID].Total++
        }
    }
    
    // 3. 识别未覆盖的函数
    for _, desc := range descriptors {
        if _, exists := coverage[desc.ID]; !exists {
            uncovered = append(uncovered, desc.ID)
        }
    }
    
    return coverage
}
```

### 2.3 Provider Manifest 聚合

```go
// BuildUnifiedDescriptors() 流程

func (s *Store) BuildUnifiedDescriptors() map[string]interface{} {
    unified := map[string]interface{}{
        "providers":  make(map[string]interface{}),
        "functions":  make([]interface{}, 0),
        "entities":   make([]interface{}, 0),
        "operations": make([]interface{}, 0),
    }
    
    // 遍历所有Provider
    for providerID, provCaps := range s.provCaps {
        
        // 解析Manifest JSON
        var manifest map[string]interface{}
        json.Unmarshal(provCaps.Manifest, &manifest)
        
        // 添加Provider信息
        providers[providerID] = map[string]interface{}{
            "id": provCaps.ID,
            "version": provCaps.Version,
            "lang": provCaps.Lang,
            "sdk": provCaps.SDK,
            "updated_at": provCaps.UpdatedAt,
        }
        
        // 合并functions
        if functions, exists := manifest["functions"]; exists {
            unified["functions"] = append(unified["functions"], functions...)
        }
        
        // 合并entities
        if entities, exists := manifest["entities"]; exists {
            unified["entities"] = append(unified["entities"], entities...)
        }
        
        // 合并operations
        if operations, exists := manifest["operations"]; exists {
            unified["operations"] = append(unified["operations"], operations...)
        }
    }
    
    return unified
}
```

---

## 三、函数包系统详解

### 3.1 包文件结构规范

```
my-game-pack/
│
├── manifest.json
│   {
│     "name": "Game Pack",
│     "version": "1.0.0",
│     "functions": [
│       { "id": "player.ban", "version": "1.0.0", "category": "player" },
│       { "id": "item.create", "version": "1.0.0", "category": "item" }
│     ],
│     "dependencies": ["analytics-pack"],
│     "web_plugins": ["web-plugin/player-ui.js"],
│     "author": "game-team",
│     "license": "MIT"
│   }
│
├── descriptors/
│   ├── player.ban.json
│   │   {
│   │     "id": "player.ban",
│   │     "version": "1.0.0",
│   │     "risk": "high",
│   │     "auth": { "permission": "player.ban", "require_approval": true },
│   │     "params": { /* JSON Schema */ },
│   │     "outputs": { /* 输出定义 */ }
│   │   }
│   ├── item.create.json
│   └── item.delete.json
│
├── ui/
│   ├── player.ban.uischema.json
│   │   {
│   │     "fields": {
│   │       "player_id": { "label": "Player ID", "placeholder": "Enter ID" },
│   │       "reason": { "label": "Ban Reason", "widget": "textarea" },
│   │       "duration": { "label": "Duration (days)" }
│   │     },
│   │     "ui:groups": [
│   │       { "title": "Basic Info", "fields": ["player_id", "reason"] },
│   │       { "title": "Options", "fields": ["duration"] }
│   │     ],
│   │     "ui:layout": { "type": "tabs", "cols": 1 }
│   │   }
│   ├── player.ban.schema.json
│   │   { /* 额外的JSON Schema用于UI验证 */ }
│   └── item.create.uischema.json
│
└── web-plugin/
    └── player-ui.js  # 自定义UI插件
```

### 3.2 ComponentManager 操作流程

```go
// 安装流程
func (cm *ComponentManager) InstallComponent(componentPath string) error {
    // 1. 加载manifest
    manifest, err := cm.loadManifest(componentPath)
    if err != nil { return err }
    
    // 2. 检查依赖
    for _, dep := range manifest.Dependencies {
        if _, exists := cm.registry.Installed[dep]; !exists {
            return fmt.Errorf("missing dependency: %s", dep)
        }
    }
    
    // 3. 复制组件文件
    destDir := filepath.Join(cm.installedDir, manifest.Category, manifest.ID)
    err = cm.copyComponent(componentPath, destDir)
    if err != nil { return err }
    
    // 4. 更新registry
    cm.registry.Installed[manifest.ID] = manifest
    
    // 5. 保存registry到文件
    return cm.SaveRegistry()
}

// 禁用流程
func (cm *ComponentManager) DisableComponent(componentID string) error {
    manifest, exists := cm.registry.Installed[componentID]
    if !exists { return fmt.Errorf("component not found") }
    
    // 1. 检查反向依赖
    for id, m := range cm.registry.Installed {
        for _, dep := range m.Dependencies {
            if dep == componentID {
                return fmt.Errorf("component %s depends on %s", id, componentID)
            }
        }
    }
    
    // 2. 移动到disabled目录
    srcDir := filepath.Join(cm.installedDir, manifest.Category, componentID)
    destDir := filepath.Join(cm.disabledDir, manifest.Category, componentID)
    os.Rename(srcDir, destDir)
    
    // 3. 更新registry
    cm.registry.Disabled[componentID] = manifest
    delete(cm.registry.Installed, componentID)
    
    return cm.SaveRegistry()
}
```

### 3.3 TypeRegistry Protocol Buffer支持

```go
// TypeRegistry 用于动态Protocol Buffer编解码

type TypeRegistry struct {
    files *protoregistry.Files   // 文件描述符注册表
    types *protoregistry.Types   // 类型注册表
}

// 加载FileDescriptorSet
func (r *TypeRegistry) LoadFDS(b []byte) error {
    var fds descriptorpb.FileDescriptorSet
    if err := proto.Unmarshal(b, &fds); err != nil {
        return err
    }
    
    // 创建Files对象
    files, err := protodesc.NewFiles(&fds)
    if err != nil { return err }
    
    // 注册所有文件和类型
    files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
        r.files.RegisterFile(fd)
        return true
    })
    return nil
}

// JSON <-> Protocol Buffer 转换
func (r *TypeRegistry) JSONToProtoBin(typeFQN string, jsonBytes []byte) ([]byte, error) {
    // 查找类型
    d, err := r.files.FindDescriptorByName(protoreflect.FullName(typeFQN))
    if err != nil { return nil, err }
    
    // 获取Message描述符
    md := d.(protoreflect.MessageDescriptor)
    
    // 创建动态Message
    msg := dynamicpb.NewMessage(md)
    
    // JSON反序列化到Message
    if err := protojson.Unmarshal(jsonBytes, msg); err != nil {
        return nil, err
    }
    
    // 序列化为二进制
    return proto.Marshal(msg)
}
```

---

## 四、函数调用详细流程

### 4.1 同步调用 (POST /api/invoke)

```go
// 请求处理流程

POST /api/invoke
├─ 身份认证
│  └─ s.auth(c.Request) 
│     ├─ JWT令牌验证
│     ├─ mTLS证书验证
│     └─ 返回 (user, roles)
│
├─ 参数验证
│  ├─ JSON格式校验
│  └─ JSON Schema验证
│     └─ validation.ValidateJSON(desc.Params, payload)
│
├─ RBAC权限检查
│  ├─ 基础权限: "function:{functionID}"
│  ├─ 自定义权限: desc.Auth["permission"]
│  ├─ 作用域权限: "game:{gameID}:function:{functionID}"
│  └─ 通配符: "game:{gameID}:*"
│
├─ allow_if 表达式求值 (可选)
│  └─ evalAllowIf("roles.includes('admin')", policyContext)
│
├─ 维护状态检查
│  └─ isWriteBlocked(gameID, env)
│
├─ 审计日志
│  └─ s.audit.Log("invoke", user, functionID, {...})
│
└─ 转发至Agent
   ├─ 路由选择
   │  ├─ lb: 负载均衡选择
   │  ├─ broadcast: 所有实例
   │  ├─ targeted: 指定服务ID
   │  └─ hash: 一致性哈希(基于hashKey)
   │
   ├─ 建立gRPC连接
   │  └─ Agent.gRPC:19090
   │
   ├─ 调用Function Service
   │  └─ Invoke(InvokeRequest{...})
   │
   └─ 返回响应
      └─ HTTP 200 + 结果JSON
```

### 4.2 异步任务 (POST /api/start_job)

```go
// 任务启动流程

POST /api/start_job
├─ 创建Job实例
│  ├─ job_id = uuid.New()
│  ├─ 初始状态: pending
│  └─ 保存到JobStore
│
├─ 返回job_id
│  └─ { job_id: "uuid" }
│
└─ Agent启动异步执行
   ├─ 后台Goroutine执行
   ├─ 定期报告进度
   │  ├─ type: "progress", progress: 0-100
   │  ├─ type: "log", message: "..."
   │  ├─ type: "done", payload: resultJSON
   │  └─ type: "error", message: "errorMsg"
   │
   └─ Server通过SSE转发

// 任务流监听 (GET /api/stream_job?id=jobId)
GET /api/stream_job?id={jobId}
├─ 打开SSE连接
├─ 监听事件队列
└─ 事件格式:
   ├─ event: progress
   │  └─ data: {"progress": 50}
   │
   ├─ event: log
   │  └─ data: {"message": "Processing item 1..."}
   │
   ├─ event: done
   │  └─ data: {"payload": {...}}
   │
   ├─ event: error
   │  └─ data: {"message": "Operation failed"}
   │
   └─ (连接关闭)
```

### 4.3 权限模型详解

```
权限查询链：
1. 检查函数的自定义权限 (desc.Auth["permission"])
   如果定义: perm = "custom.permission"
   否则: perm = "function:{functionID}"

2. 尝试匹配权限：
   ├─ s.can(user, roles, "game:{gameID}:{perm}")  // 游戏级权限
   ├─ s.can(user, roles, perm)                    // 全局权限
   ├─ s.can(user, roles, "game:{gameID}:*")       // 游戏通配符
   └─ s.can(user, roles, "*")                     // 超级权限

3. 条件表达式 allow_if：
   allow_if: "roles.includes('admin') && env == 'prod'"
   
   在policy context中求值：
   {
     User: "user123",
     Roles: ["player-admin", "dev"],
     GameID: "game1",
     Env: "prod",
     FunctionID: "player.ban"
   }

4. 审核/批准流程 (if require_approval)：
   ├─ 创建approval请求
   ├─ 等待批准者审批
   ├─ timeout: 1小时
   └─ 批准后执行或拒绝

5. 两人规则 (if two_person_rule)：
   ├─ 记录请求者
   ├─ 需要不同用户批准
   └─ 都记录在审计链中
```

---

## 五、Web前端实现细节

### 5.1 GmFunctions 页面的表单渲染

```tsx
// 三种渲染模式的关键差异

// 模式1: Enhanced UI (推荐)
// 特点: 支持 show_if, required_if, 分组, 选项卡
const renderXFormItems = (desc, ui, form) => {
    const groups = ui['ui:groups'] || [];
    const layoutType = ui['ui:layout']?.type || 'grid';
    
    if (layoutType === 'tabs') {
        return (
            <Tabs items={groups.map((g, gi) => ({
                key: String(gi),
                label: g.title,
                children: (
                    <Row gutter={12}>
                        {g.fields.map(key => (
                            <Col span={span}>
                                {renderXUIField(
                                    key,
                                    props[key],
                                    uiFields[key],
                                    values,
                                    form,
                                    [key],
                                    required.includes(key)
                                )}
                            </Col>
                        ))}
                    </Row>
                )
            }))} />
        );
    }
};

// 字段渲染支持的特性:
// 1. 条件显示 (show_if)
//    show_if: "$.status == 'active'"
//    evalExpr() 解析和求值表达式
//
// 2. 条件必填 (required_if)
//    required_if: "$.type == 'permanent'"
//    动态添加required验证规则
//
// 3. 字段类型支持
//    - string, integer, number, boolean
//    - date, time, datetime (date/time picker)
//    - enum (select dropdown)
//    - array (Form.List with add/remove)
//    - object (nested object)
//    - map (additionalProperties)
//
// 4. 验证规则 (JSON Schema)
//    - minLength, maxLength
//    - minimum, maximum
//    - pattern (正则表达式)

// 模式2: Form-Render
// 特点: 独立库，复杂schema支持好，但不支持条件显示
<FormRender
    schema={currentDesc.params}
    uiSchema={uiSchema?.fields || {}}
    formData={formData}
    onChange={setFormData}
    displayType="row"
    labelWidth={120}
/>

// 模式3: Legacy
// 特点: 基础Ant Design Form，功能最少
<Form form={form}>
    {renderFormItems(desc, ui, form)}
</Form>
```

### 5.2 路由策略选择

```tsx
// 路由选择UI
<Select
    value={route}
    onChange={(v) => setRoute(v)}
    options={[
        { label: 'LB (负载均衡)', value: 'lb' },
        { label: 'Broadcast (广播)', value: 'broadcast' },
        { label: 'Targeted (指定)', value: 'targeted' },
        { label: 'Hash (哈希)', value: 'hash' }
    ]}
/>

// 根据路由选择显示额外选项
{route === 'targeted' && (
    <Select
        placeholder="Select target service"
        value={targetService}
        onChange={setTargetService}
        options={instances.map(i => ({
            label: `${i.service_id} @ ${i.agent_id}`,
            value: i.service_id
        }))}
    />
)}

{route === 'hash' && (
    <Input
        placeholder="e.g. player_id"
        value={hashKey}
        onChange={(e) => setHashKey(e.target.value)}
    />
)}

// 调用时传递路由选项
invokeFunction(currentId, payload, {
    route,
    target_service_id: route === 'targeted' ? targetService : undefined,
    hash_key: route === 'hash' ? hashKey : undefined
})
```

### 5.3 结果可视化 (Views Rendering)

```tsx
// outputs.views 中定义的渲染规则

{currentDesc?.outputs?.views && currentDesc.outputs.views.length > 0 && (
    <div>
        {currentDesc.outputs.views.map(v => {
            // 1. 条件显示检查 (show_if)
            if (typeof v.show_if === 'string') {
                try {
                    const cond = applyTransform(lastOutput, { expr: v.show_if });
                    if (!cond || (Array.isArray(cond) && cond.length === 0)) {
                        return null;
                    }
                } catch {}
            }
            
            // 2. 应用数据变换 (transform)
            const data = applyTransform(lastOutput, v.transform);
            
            // 3. 查找并调用renderer
            const Renderer = getRenderer(v.renderer || v.type || 'json.view');
            if (!Renderer) return <div>No renderer: {v.renderer}</div>;
            
            // 4. 渲染组件
            return (
                <div key={v.id}>
                    <Renderer data={data} options={v.options} />
                </div>
            );
        })}
    </div>
)}

// 支持的Renderer
// - json.view: JSON树形展示
// - table.basic: 基础表格
// - echarts.bar: 柱状图
// - echarts.line: 折线图
// - custom.renderer: 自定义渲染器
```

### 5.4 数据变换规则 (Transform)

```json
{
  "transform": {
    "expr": "$.result.items",  // JSONPath 表达式
    "template": {              // 模板变换
      "forEach": {
        "path": "$.items",     // 迭代路径
        "template": {          // 每个元素的变换
          "name": "$.name",
          "value": { "number": "$.count" },
          "percent": { "divideBy": ["$.count", "$.total"] }
        }
      }
    }
  }
}
```

---

## 六、HTTP API 细节

### 6.1 GET /api/descriptors

```go
// 单一模式 (默认)
GET /api/descriptors
Response: [
    {
        "id": "player.ban",
        "version": "1.0.0",
        "category": "player",
        "params": { /* JSON Schema */ },
        "outputs": { /* 视图定义 */ }
    },
    ...
]

// 详细模式
GET /api/descriptors?detailed=true
Response: {
    "legacy_descriptors": [
        { /* 从packs/*/descriptors/加载 */ }
    ],
    "provider_manifests": {
        "providers": {
            "go-sdk": {
                "id": "go-sdk",
                "version": "1.0.0",
                "lang": "go",
                "updated_at": "2024-11-13T..."
            }
        },
        "functions": [ /* 聚合的函数 */ ],
        "entities": [ /* 聚合的实体 */ ],
        "operations": [ /* 聚合的操作 */ ]
    }
}
```

### 6.2 GET /api/registry

```go
Response: {
    "agents": [
        {
            "agent_id": "agent-001",
            "game_id": "game1",
            "env": "prod",
            "rpc_addr": "10.0.1.1:19090",
            "ip": "10.0.1.1",
            "type": "agent",
            "version": "v1.2.3",
            "functions": 5,
            "healthy": true,
            "expires_in_sec": 25
        }
    ],
    "functions": [
        {
            "game_id": "game1",
            "id": "player.ban",
            "agents": 2
        }
    ],
    "assignments": {
        "game1|prod": ["player.ban", "player.mute", ...]
    },
    "coverage": [
        {
            "game_env": "game1|prod",
            "functions": {
                "player.ban": { "healthy": 2, "total": 3 },
                "player.mute": { "healthy": 2, "total": 2 }
            },
            "uncovered": ["item.delete"]
        }
    ]
}
```

### 6.3 GET /api/packs/list

```go
Response: {
    "manifest": {
        "functions": [
            { "id": "prom.query", "version": "1.0.0" },
            { "id": "prom.query_range", "version": "1.0.0" }
        ],
        "web_plugins": ["web-plugin/echarts_plugin.js"]
    },
    "counts": {
        "descriptors": 42,
        "ui_schema": 38
    },
    "etag": "sha256:abc123...",
    "export_auth_required": true
}
```

### 6.4 POST /api/providers/capabilities

```go
// Provider上传manifest (HTTP接口)

Request: {
    "provider": {
        "id": "go-sdk",
        "version": "1.0.0",
        "lang": "go",
        "sdk": "go-croupier"
    },
    "manifest_json": {
        "provider": { /* ... */ },
        "functions": [ /* ... */ ],
        "entities": [ /* ... */ ]
    }
}

Response: 204 No Content (成功)

// Manifest验证:
// 1. 使用 docs/providers-manifest.schema.json 验证
// 2. 大小限制: 10MB
// 3. 存储到Registry
```

---

## 七、安全与审计

### 7.1 参数验证流程

```
Request 到达 → JSON Schema验证
    ├─ type检查 (object, array, string, etc.)
    ├─ required字段检查
    ├─ minLength/maxLength验证
    ├─ minimum/maximum验证 (数值)
    ├─ pattern正则验证
    ├─ enum枚举值检查
    ├─ additionalProperties检查
    └─ 递归验证嵌套对象

JSON Schema示例:
{
    "type": "object",
    "properties": {
        "player_id": { "type": "string", "minLength": 1 },
        "amount": { "type": "integer", "minimum": 1, "maximum": 1000000 },
        "items": {
            "type": "array",
            "items": { "type": "string" },
            "minItems": 1
        }
    },
    "required": ["player_id"],
    "additionalProperties": false
}
```

### 7.2 审计日志内容

```go
// 审计记录
{
    "action": "invoke",
    "user": "admin-user",
    "function_id": "player.ban",
    "timestamp": "2024-11-13T10:30:45Z",
    "ip": "203.0.113.42",
    "trace_id": "abc123def456",
    "game_id": "game1",
    "env": "prod",
    "payload_snapshot": "{\"player_id\":\"***\",\"reason\":\"abuse\"}",  // 掩码处理
    "result": "success" | "failure",
    "error_message": "..." // 如果失败
}

// 敏感字段掩码规则:
// - 密码相关: 完全隐藏
// - 用户ID: 保留前缀，如 "user_***"
// - Token: 仅显示后6位
// - 金额: 仅显示位数，如 "*****"
```

---

## 总结：关键实现亮点

1. **描述符驱动**: 单一JSON源驱动UI、验证、权限、审计
2. **多源聚合**: Legacy + Provider manifest的统一管理
3. **灵活的权限模型**: RBAC + 条件表达式的组合
4. **丰富的可视化**: 多种renderer，支持数据变换
5. **完整的审计链**: Trace ID关联，敏感字段掩码
6. **异步任务支持**: SSE实时流，进度报告
7. **模块化包系统**: 依赖管理，版本控制

这个架构展现了高度的工程化思想，通过数据驱动和模块化设计，实现了灵活可扩展的函数管理平台。
