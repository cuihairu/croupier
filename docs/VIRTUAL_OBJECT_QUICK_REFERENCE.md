# Croupier 虚拟对象(Virtual Object) - 快速参考指南

## 🎯 核心概念速览

### 四层架构

```
Function (函数)
    ↓ 绑定到
Entity (虚拟对象)
    ↓ 组织成
Resource (资源/UI)
    ↓ 打包为
Component (组件)
```

| 层级 | 文件 | 定义 |
|------|------|------|
| **Function** | `*.json` | 单个操作实现 |
| **Entity** | `*.entity.json` | 业务对象定义 + 操作映射 |
| **Resource** | `*.resource.json` | UI展现层 + 函数组合 |
| **Component** | `manifest.json` | 模块打包单位 |

---

## 📝 关键文件清单

### 核心文档 (必读)
```
📄 ARCHITECTURE.md                          # 完整架构文档
📄 docs/providers-manifest.md               # Provider标准说明
📄 docs/providers-manifest.schema.json      # Manifest验证规范
📄 docs/VIRTUAL_OBJECT_DESIGN.md            # 本项目的详细分析
```

### 实现代码
```
📝 internal/validation/entity.go            # Entity验证逻辑
📝 internal/function/descriptor/loader.go   # Descriptor加载
📝 internal/pack/manager.go                 # 组件管理器
📝 internal/app/server/http/server.go       # HTTP API实现(L4060+)
```

### 实例定义
```
components/
├── player-management/
│   ├── manifest.json
│   └── descriptors/
│       ├── player.entity.json              # Entity示例
│       ├── player.resource.json            # Resource示例
│       └── player.register.json            # Function示例
├── item-management/                        # 物品管理
├── economy-system/                         # 经济系统(跨实体)
└── entity-management/                      # 虚拟对象管理系统本身
```

---

## 🏗️ 虚拟对象设计模板

### 1️⃣ Entity Definition (`*.entity.json`)

```json
{
  "id": "player.entity",
  "version": "1.0.0",
  "name": "Player Entity",
  "type": "entity",
  "category": "player",
  "schema": {
    "type": "object",
    "properties": {
      "player_id": {"type": "string", "primary_key": true},
      "username": {"type": "string", "unique": true, "searchable": true},
      "status": {"type": "string", "enum": ["active", "banned"], "filterable": true}
    },
    "required": ["player_id", "username"]
  },
  "operations": {
    "create": ["player.register"],           // 可以是单函数或函数数组
    "read": ["player.profile.get"],
    "update": ["player.profile.update"],
    "delete": ["player.ban"],
    "list": ["player.list"],
    "custom": ["player.unban"]               // 自定义操作
  },
  "ui": {
    "display_field": "username",             // 显示字段
    "title_template": "{username} ({id})",   // 标题模板
    "avatar_field": "avatar_url",            // 头像字段
    "status_field": "status"                 // 状态字段
  }
}
```

**关键点**：
- `operations` 映射标准CRUD和自定义操作到函数
- 每个操作可绑定单个或多个函数
- `ui` 定义显示配置

### 2️⃣ Function Definition (`*.json`)

```json
{
  "id": "player.register",
  "version": "1.0.0",
  "entity": {
    "name": "player",
    "operation": "create"                    // 关联到Entity的操作
  },
  "params": {
    "type": "object",
    "properties": {
      "username": {"type": "string", "pattern": "^[a-zA-Z0-9_]{3,16}$"},
      "email": {"type": "string", "format": "email"}
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

**关键点**：
- `entity` 字段指定关联的对象和操作
- `params` 和 `result` 定义输入输出Schema
- `auth` 定义权限和条件
- `semantics` 定义限流、并发等

### 3️⃣ Resource Definition (`*.resource.json`)

```json
{
  "id": "player.resource",
  "type": "resource",
  "entity": {
    "name": "player",
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
      "label": "玩家列表"
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
        "searchable": true
      },
      {
        "dataIndex": "status",
        "title": "状态",
        "filterable": true
      }
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

**关键点**：
- `operations` 中每个操作指定具体函数和UI标签
- `ui` 定义ProTable的列定义和行为
- `actions` 定义工具栏和行操作按钮
- `features` 定义表格功能

### 4️⃣ Component Manifest (`manifest.json`)

```json
{
  "id": "player-management",
  "name": "Player Management System",
  "version": "1.0.0",
  "category": "player",
  "dependencies": [],
  "entities": [
    {"id": "player", "name": "Player"}
  ],
  "functions": [
    {
      "id": "player.register",
      "version": "1.0.0",
      "enabled": true,
      "description": "Register a new player"
    }
  ]
}
```

---

## 🔄 函数组合模式

### 模式1: 单操作单函数

```json
"operations": {
  "create": "player.register"               // 直接指定函数ID
}
```

### 模式2: 单操作多函数

```json
"operations": {
  "create": ["player.validate", "player.register", "player.notify"]  // 按顺序执行
}
```

### 模式3: 跨实体操作

```json
{
  "id": "wallet.transfer",
  "params": {
    "from_player_id": "...",
    "to_player_id": "...",
    "currency_code": "...",
    "amount": "..."
  },
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

---

## 🔌 HTTP API 端点

### Entity 管理

| 端点 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/api/entities` | GET | `entities:read` | 列出所有entity |
| `/api/entities` | POST | `entities:create` | 创建新entity |
| `/api/entities/:id` | GET | `entities:read` | 获取entity详情 |
| `/api/entities/:id` | PUT | `entities:update` | 更新entity |
| `/api/entities/:id` | DELETE | `entities:delete` | 删除entity |

### Descriptor 与 Provider

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/descriptors` | GET | 获取所有descriptor |
| `/api/descriptors?detailed=true` | GET | 获取详细descriptor + provider manifest |
| `/api/providers/capabilities` | POST | 注册provider能力 |
| `/api/providers/descriptors` | GET | 获取所有provider的descriptors |
| `/api/providers/entities` | GET | 聚合所有provider的entities |

---

## ✅ 设计检查清单

创建虚拟对象时，确保：

- [ ] **Entity定义**
  - [ ] ID遵循 `entity.name` 命名
  - [ ] type为 `entity`
  - [ ] schema是有效的JSON Schema
  - [ ] operations映射了必要的CRUD操作
  - [ ] ui配置了display_field和title_template

- [ ] **Function定义**
  - [ ] ID遵循 `entity.operation` 命名
  - [ ] 有entity字段指定关联对象
  - [ ] params是有效的JSON Schema
  - [ ] result定义了输出格式
  - [ ] auth声明了权限要求
  - [ ] semantics定义了限流和超时

- [ ] **Resource定义**
  - [ ] operations的function字段指向存在的函数
  - [ ] ui.columns定义了所有必要的列
  - [ ] actions定义了toolbar和row操作
  - [ ] auth权限与entity操作一致

- [ ] **Component清单**
  - [ ] 所有function都在manifest中声明
  - [ ] dependencies指定了依赖的其他component
  - [ ] entities声明了所有定义的entity

---

## 🚀 常见任务

### 创建新的虚拟对象

1. 在 `components/{component}/descriptors/` 中创建文件：
   ```
   entity.entity.json     - Entity定义
   entity.resource.json   - Resource定义
   entity.create.json     - 创建函数
   entity.get.json        - 读取函数
   entity.update.json     - 更新函数
   entity.delete.json     - 删除函数
   entity.list.json       - 列表函数
   ```

2. 在 `components/{component}/manifest.json` 中声明函数

3. POST to `/api/entities` 创建entity（或直接编辑JSON）

### 添加自定义操作

1. 创建函数定义 `entity.custom_op.json`
2. 在entity的operations中添加：
   ```json
   "custom_op": ["entity.custom_op"]
   ```
3. 如需UI，在resource中添加到actions

### 实现跨实体操作

1. 创建函数，params中包含多个entity的ID
2. 在auth中标记 `"risk": "high"` 和 `"two_person_rule": true`
3. 在semantics中标记 `"atomic": true`
4. 在function params中定义完整的关系和验证规则

---

## 📊 现有实现参考

### Player Management (完整示例)
- 路径: `components/player-management/`
- 包含: player.entity + player.resource + 所有CRUD函数
- 特点: 标准CRUD + 自定义操作(unban)

### Economy System (跨实体示例)
- 路径: `components/economy-system/`
- 包含: currency + wallet + wallet.transfer(跨实体)
- 特点: 关系定义 + 原子操作

### Entity Management (系统本身)
- 路径: `components/entity-management/`
- 特点: entity.resource用于管理entity定义本身

---

## 🔍 调试技巧

### 验证Entity定义

```bash
# Entity验证
curl -X POST http://localhost:8080/api/entities/:id/validate

# 获取所有entity
curl http://localhost:8080/api/entities

# 检查descriptor
curl http://localhost:8080/api/descriptors?id=player.entity
```

### 查看实现源码

- Entity验证: `internal/validation/entity.go`
- HTTP处理: `internal/app/server/http/server.go:4060`
- Descriptor加载: `internal/function/descriptor/loader.go`

---

## 📚 相关文档

- **完整分析**: `docs/VIRTUAL_OBJECT_DESIGN.md`
- **Manifest标准**: `docs/providers-manifest.md`
- **架构文档**: `ARCHITECTURE.md`
- **TODO任务**: `todo.md` (函数管理与 Provider Manifest 部分)

