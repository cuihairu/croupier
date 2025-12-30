# Croupier 虚拟对象(Virtual Object)架构完整分析

## 📚 执行摘要

Croupier 项目中的"虚拟对象"概念是一套完整的**对象驱动的组件化管理系统**，基于 JSON Schema 实现低代码/无代码管理界面生成。虚拟对象在系统中被称为 **Entity(实体)**，通过 **Resource(资源)** 进行 UI 层级的函数组合，最终打包为 **Component(组件)** 进行模块化管理。

---

## 1️⃣ 虚拟对象的核心定义

### 1.1 概念模型

在 Croupier 中，"虚拟对象"对应以下三个层级的概念：

| 层级 | 中文名 | 文件格式 | 作用 |
|------|--------|---------|------|
| **Entity** | 实体/虚拟对象 | `*.entity.json` | 定义业务对象的完整描述，包括数据结构、UI配置、操作映射 |
| **Function** | 函数 | `*.json` | 具体的业务操作实现，包含输入输出Schema、权限、语义 |
| **Resource** | 资源 | `*.resource.json` | UI层面的操作集合，将多个函数组合成完整的管理界面 |
| **Component** | 组件 | `manifest.json` | 功能模块的打包单位，包含entities、functions、resources |

### 1.2 Official Definition

根据 `docs/providers-manifest.md` 的定义：

> **entity（实体/虚拟对象）**：业务对象类型（可"虚拟"，仅有上下文/生命周期），含对象 schema 及一组操作（create/get/update/delete/custom…）。每个操作独立声明参数、权限、目标定位方式（如何找到某个对象实例）。

关键特征：
- **虚拟化**：可以是纯粹的上下文对象，不一定对应数据库表
- **多操作**：支持标准CRUD和自定义操作
- **目标定位**：通过 `target` 字段指定如何找到对象实例
- **权限隔离**：每个操作独立的权限声明

---

## 2️⃣ 虚拟对象的设计文档

### 2.1 Official Documents

#### 📄 `docs/providers-manifest.md`
```
路径: /Users/cui/Workspaces/croupier/docs/providers-manifest.md
大小: 111 行
内容: Provider Manifest 设计说明
```

核心内容：
- **Manifest 文件结构**：JSON格式，包含provider元信息、functions数组、entities数组
- **参数定义与校验**：首选JSON-Schema，支持x-ui扩展、x-mask敏感字段标记
- **虚拟对象操作定义**：
  - `op`：操作类型(create/get/update/delete/custom)
  - `target`：定位方式(field或jsonpath)
  - `request/response`：Schema或Proto FQN
  - `auth.require`：权限要求
  - `semantics`：限流、并发、幂等性等

#### 📄 `docs/providers-manifest.schema.json`
```
路径: /Users/cui/Workspaces/croupier/docs/providers-manifest.schema.json
大小: 167 行
内容: Manifest JSON Schema 验证规范
```

核心schema定义：
- `entity`：id, title, color, schema, operations[]
- `operation`：op, target, request, response, auth, semantics, transport, routing, ui

#### 📄 `ARCHITECTURE.md`
```
路径: /Users/cui/Workspaces/croupier/ARCHITECTURE.md
大小: 323 行
内容: 对象驱动系统的完整架构文档
```

三层架构：
```
┌─────────────────────────────┐
│   UI Resource Layer         │  ← 资源配置层
│  player.resource, ...       │
└─────────────────────────────┘
           ↓
┌─────────────────────────────┐
│   Entity Definition Layer    │  ← 实体定义层
│  player.entity, ...         │
└─────────────────────────────┘
           ↓
┌─────────────────────────────┐
│   Function Layer            │  ← 函数实现层
│  player.register, ...       │
└─────────────────────────────┘
```

---

## 3️⃣ 虚拟对象如何组合多个函数

### 3.1 函数绑定机制

每个函数通过 `entity` 字段绑定到虚拟对象：

```json
{
  "id": "player.register",
  "entity": {
    "name": "player",
    "operation": "create"
  },
  "params": { /* 输入 Schema */ },
  "result": { /* 输出 Schema */ },
  "auth": { /* 权限控制 */ }
}
```

### 3.2 Entity Definition 中的操作映射

Entity定义了哪些函数操作该对象：

```json
{
  "id": "player.entity",
  "type": "entity",
  "schema": { /* JSON Schema */ },
  "operations": {
    "create": ["player.register"],
    "read": ["player.profile.get"],
    "update": ["player.profile.update"],
    "delete": ["player.ban"],
    "list": ["player.list"],
    "unban": ["player.unban"]  // 自定义操作
  },
  "ui": {
    "display_field": "username",
    "title_template": "{username} ({nickname})",
    "avatar_field": "avatar_url",
    "status_field": "status"
  }
}
```

**关键点**：
- 每个操作可以绑定**单个函数**或**函数数组**
- 支持标准CRUD和自定义操作
- UI配置指定显示方式

### 3.3 Resource Definition 中的函数组合

Resource 在UI层面组合函数成完整的管理界面：

```json
{
  "id": "player.resource",
  "type": "resource",
  "entity": {
    "name": "player",
    "label": "玩家",
    "primary_key": "player_id",
    "display_field": "username"
  },
  "operations": {
    "create": {
      "function": "player.register",
      "label": "注册玩家",
      "icon": "UserAddOutlined"
    },
    "read": {
      "function": "player.profile.get",
      "label": "查看详情",
      "icon": "EyeOutlined"
    },
    "update": {
      "function": "player.profile.update",
      "label": "编辑资料",
      "icon": "EditOutlined"
    },
    "delete": {
      "function": "player.ban",
      "label": "封禁玩家",
      "icon": "StopOutlined",
      "danger": true
    },
    "list": {
      "function": "player.list",
      "label": "玩家列表",
      "icon": "TableOutlined"
    }
  },
  "ui": {
    "type": "pro-table",
    "layout": "table",
    "columns": [
      /* ProTable列定义 */
    ],
    "actions": {
      "toolbar": ["create"],
      "row": ["read", "update", "delete"]
    },
    "features": {
      "searchable": true,
      "pagination": true,
      "sortable": true,
      "filterable": true,
      "exportable": true
    }
  },
  "auth": {
    "permission": "player:manage",
    "allow_if": "has_role('gm') || has_role('admin')"
  },
  "semantics": {
    "cacheable": true,
    "cache_ttl": "5m"
  }
}
```

---

## 4️⃣ 现有的虚拟对象实现代码

### 4.1 后端实现（Go）

#### 📝 Entity 验证（`internal/validation/entity.go`）

```go
package validation

// ValidateEntityDefinition validates an entity definition structure
func ValidateEntityDefinition(entity map[string]any) []string {
    // 检查必需字段：id, type, schema
    // 验证JSON Schema结构
    // 验证operations映射
    // 验证UI配置
}

// validateJSONSchema 验证JSON Schema本身
// validateSchemaProperties 验证各属性定义
// validateOperations 验证操作映射
// validateUIConfig 验证UI配置
```

功能：
- 验证entity的id和type字段
- 验证schema是否为有效的JSON Schema
- 验证operations映射的操作名称和函数ID
- 验证UI配置的字段

#### 📝 Descriptor Loader（`internal/function/descriptor/loader.go`）

```go
package descriptor

// Descriptor is a simplified function descriptor model for UI/validation
type Descriptor struct {
    ID        string         `json:"id"`
    Version   string         `json:"version"`
    Category  string         `json:"category"`
    Risk      string         `json:"risk"`
    Auth      map[string]any `json:"auth"`
    Params    map[string]any `json:"params"`
    Semantics map[string]any `json:"semantics"`
    Transport map[string]any `json:"transport"`
    Outputs   map[string]any `json:"outputs"`
    UI        map[string]any `json:"ui"`
}

// LoadAll 从目录递归加载所有描述符
func LoadAll(dir string) ([]*Descriptor, error)
```

特点：
- 递归扫描目录加载所有.json文件
- 跳过ui子目录和无id字段的文件
- 构建统一的Descriptor结构供UI和校验使用

#### 📝 Component Manager（`internal/pack/manager.go`）

```go
package pack

// ComponentManager manages function components
type ComponentManager struct {
    dataDir      string
    installedDir string
    disabledDir  string
    registry     *ComponentRegistry
}

type ComponentManifest struct {
    ID           string              `json:"id"`
    Name         string              `json:"name"`
    Version      string              `json:"version"`
    Description  string              `json:"description"`
    Category     string              `json:"category"` // player, item, economy, social, etc.
    Dependencies []string            `json:"dependencies,omitempty"`
    Functions    []ComponentFunction `json:"functions"`
    Author       string              `json:"author,omitempty"`
    License      string              `json:"license,omitempty"`
}

// 主要方法：
// InstallComponent(componentPath) - 安装组件
// UninstallComponent(componentID) - 卸载组件
// EnableComponent(componentID) - 启用组件
// DisableComponent(componentID) - 禁用组件
// LoadRegistry() - 加载组件注册表
// SaveRegistry() - 保存组件注册表
```

### 4.2 HTTP API 实现

#### 📝 Entity 管理 API（go-zero `services/server/internal/handler/entity/*`）

| 端点 | 方法 | 权限 | 功能 |
|------|------|------|------|
| `/api/v1/entities` | GET | `entities:read` | 获取所有 entity 定义 |
| `/api/v1/entities` | POST | `entities:create` | 创建新 entity |
| `/api/v1/entities/:id` | GET | `entities:read` | 获取特定 entity |
| `/api/v1/entities/:id` | PUT | `entities:update` | 更新 entity 定义 |
| `/api/v1/entities/:id` | DELETE | `entities:delete` | 删除 entity |
| `/api/v1/entities/validate` | POST | `entities:read` | 验证 entity |
| `/api/v1/entities/:id/preview` | GET | `entities:read` | 预览 entity UI |

实现细节：
```go
// GET /api/v1/entities 扫描所有components目录
for _, entry := range entries {  // 遍历components
    descriptorsDir := filepath.Join(componentsDir, entry.Name(), "descriptors")
    for _, file := range descriptorFiles {
        if !strings.HasSuffix(file.Name(), ".entity.json") {
            continue
        }
        // 读取并解析entity定义
    }
}

// POST /api/v1/entities 保存到指定component
componentDir := filepath.Join("components", component)
descriptorsDir := filepath.Join(componentDir, "descriptors")
entityFile := filepath.Join(descriptorsDir, id+".entity.json")
os.WriteFile(entityFile, entityData, 0644)
```

#### 📝 Descriptor API（go-zero rest 示例）

> 说明：当前服务已从 Gin 迁移到 go-zero（`rest` + `httpx`）。以下为示例写法与现有路由风格一致。

```go
// GET /api/v1/functions/descriptors
server.AddRoutes([]rest.Route{
	{Method: http.MethodGet, Path: "/descriptors", Handler: function.DescriptorsHandler(serverCtx)},
}, rest.WithPrefix("/api/v1/functions"))

// GET /api/v1/providers/:id/entities（id 可为 "*" 聚合全部）
server.AddRoutes([]rest.Route{
	{Method: http.MethodGet, Path: "/:id/entities", Handler: provider.ProvidersEntitiesHandler(serverCtx)},
}, rest.WithPrefix("/api/v1/providers"))
```

---

## 5️⃣ 虚拟对象的配置和管理机制

### 5.1 文件组织结构

```
components/
├── player-management/          # 组件目录
│   ├── manifest.json           # 组件清单
│   └── descriptors/
│       ├── player.entity.json  # Entity定义
│       ├── player.resource.json # Resource定义
│       ├── player.register.json # Function定义
│       ├── player.profile.get.json
│       ├── player.profile.update.json
│       ├── player.ban.json
│       ├── player.unban.json
│       └── player.list.json
│
├── item-management/
│   ├── manifest.json
│   └── descriptors/
│       ├── item.entity.json
│       ├── item.resource.json
│       ├── item.create.json
│       ├── item.get.json
│       ├── item.list.json
│       ├── item.update.json
│       └── item.delete.json
│
├── economy-system/
│   ├── manifest.json
│   └── descriptors/
│       ├── currency.entity.json
│       ├── currency.resource.json
│       ├── wallet.entity.json
│       ├── wallet.resource.json
│       └── ...
│
└── entity-management/          # 虚拟对象管理系统本身
    ├── manifest.json
    └── descriptors/
        ├── entity.resource.json     # Entity的资源配置
        ├── entity.create.json       # 创建entity函数
        ├── entity.update.json
        ├── entity.preview.json
        └── schema.validate.json
```

### 5.2 Manifest 配置格式

#### 🔧 Component Manifest (`manifest.json`)

```json
{
  "id": "player-management",
  "name": "Player Management System",
  "version": "1.0.0",
  "description": "Core player operations...",
  "category": "player",
  "dependencies": [],
  "entities": [
    {
      "id": "player",
      "name": "Player",
      "description": "Player business object"
    }
  ],
  "functions": [
    {
      "id": "player.register",
      "version": "1.0.0",
      "enabled": true,
      "description": "Register a new player"
    },
    {
      "id": "player.profile.get",
      "version": "1.0.0",
      "enabled": true,
      "description": "Get player profile"
    }
  ],
  "author": "Croupier Team",
  "license": "MIT"
}
```

#### 🔧 Entity Definition (`*.entity.json`)

```json
{
  "id": "player.entity",
  "version": "1.0.0",
  "name": "Player Entity",
  "description": "Player business object definition",
  "type": "entity",
  "category": "player",
  "schema": {
    "type": "object",
    "properties": {
      "player_id": {
        "type": "string",
        "description": "Unique player identifier",
        "primary_key": true
      },
      "username": {
        "type": "string",
        "description": "Player username",
        "unique": true,
        "searchable": true
      },
      "status": {
        "type": "string",
        "enum": ["active", "banned", "suspended"],
        "filterable": true
      }
    },
    "required": ["player_id", "username", "email"]
  },
  "operations": {
    "create": ["player.register"],
    "read": ["player.profile.get"],
    "update": ["player.profile.update"],
    "delete": ["player.ban"],
    "list": ["player.list"],
    "unban": ["player.unban"]
  },
  "ui": {
    "display_field": "username",
    "title_template": "{username} ({nickname})",
    "avatar_field": "avatar_url",
    "status_field": "status"
  }
}
```

#### 🔧 Resource Definition (`*.resource.json`)

```json
{
  "id": "player.resource",
  "version": "1.0.0",
  "name": "Player Resource Management",
  "type": "resource",
  "entity": {
    "name": "player",
    "label": "玩家",
    "primary_key": "player_id"
  },
  "operations": {
    "create": {
      "function": "player.register",
      "label": "注册玩家",
      "icon": "UserAddOutlined"
    },
    "read": {
      "function": "player.profile.get",
      "label": "查看详情",
      "icon": "EyeOutlined"
    }
  },
  "ui": {
    "type": "pro-table",
    "columns": [
      {
        "dataIndex": "player_id",
        "title": "ID",
        "width": 100,
        "fixed": "left"
      },
      {
        "dataIndex": "username",
        "title": "用户名",
        "width": 120,
        "searchable": true
      }
    ],
    "actions": {
      "toolbar": ["create"],
      "row": ["read", "update", "delete"]
    }
  },
  "auth": {
    "permission": "player:manage",
    "allow_if": "has_role('gm') || has_role('admin')"
  },
  "semantics": {
    "cacheable": true,
    "cache_ttl": "5m"
  }
}
```

#### 🔧 Function Definition (`*.json`)

```json
{
  "id": "player.register",
  "version": "1.0.0",
  "name": "Player Registration",
  "category": "player",
  "entity": {
    "name": "player",
    "operation": "create"
  },
  "params": {
    "type": "object",
    "properties": {
      "username": {
        "type": "string",
        "pattern": "^[a-zA-Z0-9_]{3,16}$"
      },
      "email": {
        "type": "string",
        "format": "email"
      }
    },
    "required": ["username", "email"]
  },
  "result": {
    "type": "object",
    "properties": {
      "player_id": {"type": "string"},
      "status": {"type": "string"}
    }
  },
  "auth": {
    "permission": "player:register",
    "allow_if": "has_role('gm') || has_role('admin')"
  },
  "semantics": {
    "idempotent": true,
    "rate_limit": "10rps",
    "timeout": "30s"
  }
}
```

### 5.3 Provider Manifest 格式（新的统一标准）

```json
{
  "provider": {
    "id": "player",
    "version": "1.2.0",
    "lang": "python",
    "sdk": "croupier-py@0.3.0"
  },
  "functions": [
    {
      "id": "player.ban",
      "request": {"json_schema": "schema/ban_request.json"},
      "response": {"json_schema": "schema/ban_response.json"},
      "auth": {"require": ["player:ban"]},
      "semantics": {"idempotent": true, "rate_limit": "100/s"}
    }
  ],
  "entities": [
    {
      "id": "session",
      "title": "Session",
      "color": "#1677ff",
      "schema": {"json_schema": "schema/session.json"},
      "operations": [
        {
          "op": "create",
          "request": {"json_schema": "schema/create_session_request.json"},
          "response": {"json_schema": "schema/session.json"},
          "auth": {"require": ["session:create"]}
        },
        {
          "op": "close",
          "target": {"field": "session_id"},
          "request": {"json_schema": "schema/close_request.json"},
          "response": {"json_schema": "schema/empty.json"}
        }
      ]
    }
  ]
}
```

---

## 6️⃣ 虚拟对象在函数注册表中的表示

### 6.1 Provider Capabilities 注册

#### HTTP API 注册流程

```
Provider Process
    ↓
GET /api/v1/providers/capabilities
    ↓
POST /api/v1/providers/:id/reload（按需刷新）
    ↓
Server Registry
    ├── 验证manifest JSON
    ├── 保存到registry
    ├── 合并provider functions到descriptors
    ├── 暴露 /api/v1/functions/descriptors
    └── 暴露 /api/v1/providers/descriptors
```

#### 实现说明（go-zero 现状）

- 当前对外能力以查询为主：`GET /api/v1/providers/capabilities` 返回 Registry 中已加载的 Provider 能力列表
- Provider 刷新：`POST /api/v1/providers/:id/reload`（触发重新加载/重建 Registry）

### 6.2 Unified Descriptors 构建

#### API 端点（go-zero 现状）

```text
GET /api/v1/functions/descriptors      # 函数描述符模板列表（用于 UI 选择/渲染）
GET /api/v1/providers/capabilities     # Provider 能力列表
GET /api/v1/providers/descriptors      # Provider manifest 聚合后的 descriptors
GET /api/v1/providers/:id/entities     # Provider entities（id="*" 聚合全部）
```

### 6.3 Entity 与 Operation 的关系

在函数注册表中，Entity 和 Operation 的关系：

```
Entity (player.entity)
├── schema: { JSON Schema }
├── operations:
│   ├── create → "player.register" (Function)
│   ├── read → "player.profile.get" (Function)
│   ├── update → "player.profile.update" (Function)
│   ├── delete → "player.ban" (Function)
│   ├── list → "player.list" (Function)
│   └── unban → "player.unban" (Function)
└── ui:
    ├── display_field
    ├── title_template
    └── ...
```

**跨实体操作**（如wallet.transfer）：

```json
{
  "id": "wallet.transfer",
  "description": "Transfer currency between wallets (cross-entity operation)",
  "category": "economy",
  "params": {
    "from_player_id": {...},
    "to_player_id": {...},
    "currency_code": {...},
    "amount": {...}
  },
  "result": {...},
  "auth": {
    "permission": "wallet:transfer",
    "allow_if": "has_role('admin') || has_role('economy_manager')",
    "risk": "high",
    "two_person_rule": true
  },
  "semantics": {
    "atomic": true,
    "idempotent": true
  }
}
```

---

## 7️⃣ Agent 和 Server 中的虚拟对象处理

### 7.1 Server 端处理（HTTP 层）

1. **Descriptor 加载和缓存**
   - 启动时加载 `components/*/descriptors/*.json`
   - 构建 `s.descs` 和 `s.descIndex` 映射
   - 支持动态加载新的provider manifest

2. **Entity API 处理**
   - 扫描所有components目录查找entity定义
   - 支持CRUD操作：create/read/update/delete
   - 验证entity定义的有效性

3. **Function Invocation**
   - 基于function_id查找descriptor
   - 使用descriptor中的params验证请求
   - 路由到相应的agent执行

### 7.2 前端集成（React/Umi）

#### UI 自动生成流程

```typescript
// 1. 从 /api/v1/providers/descriptors 获取 entity/resource 定义（聚合后的 provider manifests）
//    具体取值按 resourceId/entityId 在返回的 descriptors 中查找
const descriptors = await fetch('/api/v1/providers/descriptors').then(r => r.json())

// 2. 基于 Resource Definition 生成 ProTable
<ResourceTable
  resourceId="player.resource"
  // 自动读取操作、列定义、UI配置
/>

// 3. 基于 Entity Definition 生成表单
<EntityForm
  entityId="player.entity"
  operation="create"
  // 基于entity的schema生成表单字段
/>
```

#### 前端组件（`web/src/components/XEntityForm.tsx`）

存在前端组件用于渲染entity表单，自动生成基于schema的表单界面。

---

## 8️⃣ 与函数包(Packs)系统的关系

### 8.1 Pack 与 Component 的关系

| 概念 | 定义 | 文件格式 | 作用 |
|------|------|---------|------|
| **Pack** | 函数打包单位 | `.tgz` | 包含manifest、descriptors、schemas、UI资源 |
| **Component** | 组件打包单位 | `manifest.json` | 包含entities、functions、resources的模块 |

### 8.2 Pack 结构

```
provider.tgz (或目录)
├── manifest.json           # Provider 清单
├── schema/                 # JSON Schemas
│   ├── ban_request.json
│   ├── ban_response.json
│   └── ...
├── ui/                     # UI 附加资源
│   ├── custom_component.ts
│   └── ...
└── descriptors.fds         # 可选：FileDescriptorSet
```

### 8.3 Pack Import/Export API

```text
POST /api/v1/packs/import  # 导入 pack
GET  /api/v1/packs/export  # 导出 pack（包含 descriptors/schemas/configs 等）
```

---

## 9️⃣ 实现现状

### 9.1 已完成的功能

✅ **核心架构**
- Entity、Function、Resource、Component 四层模型
- JSON Schema 驱动的定义和验证
- 函数绑定到虚拟对象的机制

✅ **现有实现的组件**
1. **player-management**
   - player.entity.json：玩家实体定义
   - player.resource.json：玩家资源配置
   - player.register/get/update/ban/unban 等函数

2. **item-management**
   - item.entity.json：物品实体定义
   - item.resource.json：物品资源配置
   - item.create/get/list/update/delete 等函数

3. **economy-system**
   - currency.entity.json：货币实体定义
   - wallet.entity.json：钱包实体定义
   - 货币和钱包相关操作
   - **跨实体操作**：wallet.transfer（涉及player和currency两个entity）

4. **entity-management**（虚拟对象管理系统本身）
   - entity.resource.json：Entity定义管理界面
   - entity.create/update/delete/preview 函数
   - schema.validate 函数

✅ **后端 API**
- `/api/v1/entities` - Entity CRUD
- `/api/v1/functions/descriptors` - 函数描述符模板列表
- `/api/v1/providers/capabilities` - Provider 能力列表（来自 Registry）
- `/api/v1/providers/descriptors` - Provider manifests 聚合 descriptors
- `/api/v1/providers/:id/entities` - Provider entities（id="*" 聚合全部）

✅ **验证机制**
- Entity Definition 验证 (internal/validation/entity.go)
- Manifest JSON Schema 验证
- Function Parameter 验证

✅ **组件管理**
- ComponentManager - 组件安装/卸载/启用/禁用
- Component Registry - 组件注册表管理
- Dependency Resolution - 依赖关系检查

### 9.2 进行中的功能

🔄 **Provider Manifest 系统**
- Server 端接收和合并 provider manifest
- 统一 descriptors 暴露 API
- 多语言 SDK (Python/Node 等) 支持

🔄 **Proto-First 生成**
- 扩展 `tools/protoc-gen-croupier` 支持 manifest 生成
- 从 .proto 文件生成 manifest.json 和 schema

### 9.3 待实现的功能

⏳ **Entity 管理界面**
- 可视化 entity 创建和编辑
- JSON Schema 编辑器
- UI 配置工具
- 预览功能

⏳ **进阶特性**
- Composite Entity（组合实体）
- Entity Relationship（实体关系）
- Workflow Orchestration（工作流编排）
- Dynamic Entity 生成

⏳ **多租户支持**
- 租户级别的 entity 隔离
- 数据隔离
- 权限隔离

---

## 🔟 架构模式与最佳实践

### 10.1 分层模式

```
Presentation Layer (UI)
├── ProTable Component    ← 基于Resource渲染
├── ProForm Component     ← 基于Entity+Function渲染
└── UI Schema

Domain Layer
├── Entity Definition     ← 业务对象的完整描述
├── Operation Definition  ← 对象支持的操作
└── Relationship          ← 对象间的关系

Service Layer
├── Function Invocation   ← 执行具体操作
├── Parameter Validation  ← 基于Schema验证
└── Auth & Permission     ← 权限检查

Data Layer
├── Repository Pattern    ← 数据访问
├── Transaction           ← 事务支持（跨entity操作）
└── Cache                 ← 缓存策略
```

### 10.2 函数组合的两种模式

#### 模式1：Entity Operation 映射
```json
Entity.operations.create → [Function1, Function2]
```
**用途**：同一操作可以由多个函数串联完成
**例子**：用户注册可能涉及验证→创建→发送邮件

#### 模式2：Resource Operation 映射
```json
Resource.operations.create → {
  function: "entity.create",
  ui: { ... }
}
```
**用途**：Resource 在 Entity 基础上添加 UI 定义和语义
**例子**：ProTable 列定义、操作按钮、批量操作

### 10.3 跨实体操作的设计

对于涉及多个 Entity 的操作（如 wallet.transfer），设计方案：

1. **操作定义在主要 Entity**
   ```json
   {
     "id": "wallet.transfer",
     "params": {
       "from_player_id": "...",
       "to_player_id": "...",
       "currency_code": "...",
       "amount": "..."
     }
   }
   ```

2. **权限和语义配置**
   ```json
   {
     "auth": {
       "risk": "high",
       "two_person_rule": true
     },
     "semantics": {
       "atomic": true,
       "idempotent": true
     }
   }
   ```

3. **关系定义**
   ```json
   {
     "relationships": {
       "currency": {
         "type": "many-to-one",
         "entity": "currency",
         "foreign_key": "currency_id"
       },
       "player": {
         "type": "many-to-one",
         "entity": "player",
         "foreign_key": "player_id"
       }
     }
   }
   ```

---

## 1️⃣1️⃣ 文件路径总结

### 核心文档
- 📄 `/Users/cui/Workspaces/croupier/ARCHITECTURE.md` - 完整的对象驱动系统架构文档
- 📄 `/Users/cui/Workspaces/croupier/docs/providers-manifest.md` - Provider Manifest 设计说明
- 📄 `/Users/cui/Workspaces/croupier/docs/providers-manifest.schema.json` - Manifest JSON Schema

### 实现代码
- 📝 `/Users/cui/Workspaces/croupier/internal/validation/entity.go` - Entity 验证
- 📝 `/Users/cui/Workspaces/croupier/internal/function/descriptor/loader.go` - Descriptor 加载
- 📝 `/Users/cui/Workspaces/croupier/internal/pack/manager.go` - Component 管理
- 📝 `/Users/cui/Workspaces/croupier/internal/app/server/http/server.go` (第4060-4400行) - Entity API 实现

### 示例定义
- `components/player-management/descriptors/`
  - 📋 `player.entity.json`
  - 📋 `player.resource.json`
  - 📋 `player.register.json` 等

- `components/item-management/descriptors/`
  - 📋 `item.entity.json`
  - 📋 `item.resource.json`

- `components/economy-system/descriptors/`
  - 📋 `currency.entity.json`
  - 📋 `wallet.entity.json`
  - 📋 `wallet.transfer.json` (跨实体操作)

- `components/entity-management/descriptors/`
  - 📋 `entity.resource.json`
  - 📋 `entity.create.json`

---

## 1️⃣2️⃣ 总结与建议

### 核心要点

1. **虚拟对象 = Entity**：业务对象的完整定义，包括数据结构、操作和UI配置

2. **四层架构**：
   - Function Layer：具体操作实现
   - Entity Layer：对象定义和操作映射
   - Resource Layer：UI操作编排
   - Component Layer：模块打包

3. **函数组合机制**：
   - Entity.operations 映射函数ID
   - Resource 在 Entity 基础上添加 UI 定义
   - 支持多函数组合和跨实体操作

4. **Provider Manifest 标准**：统一的、语言无关的能力声明标准，支持多语言 SDK

### 建议的下一步

1. **完善 Entity 管理界面**
   - 实现可视化的 entity 创建/编辑
   - JSON Schema 编辑器集成
   - UI 预览功能

2. **实现 Proto-First 生成**
   - 扩展 protoc-gen-croupier 支持 manifest 生成
   - 支持自定义注解声明权限、语义等

3. **多语言 SDK 支持**
   - Python/Node SDK 实现
   - Out-of-proc provider 模式

4. **高级特性**
   - Entity Composition（组合实体）
   - Workflow Orchestration（工作流）
   - Dynamic Entity 生成
