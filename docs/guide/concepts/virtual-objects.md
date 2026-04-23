---
title: 虚拟对象系统
icon: cubes
order: 2
category:
  - 核心概念
tag:
  - 虚拟对象
  - 架构设计
---

# 虚拟对象系统

Croupier 采用**四层虚拟对象模型**，实现从业务定义到 UI 生成的完整驱动。

## 四层架构

```
┌────────────────────────────────────────────────────────────┐
│  Layer 4: Component（组件）                                 │
│  功能模块的打包单位，包含相关的 functions、entities、resources│
└────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────┐
│  Layer 3: Resource（资源）                                  │
│  UI 层面的操作集合，ProTable 配置、列定义、操作按钮         │
└────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────┐
│  Layer 2: Entity（实体）                                    │
│  业务对象的完整描述，JSON Schema、UI 配置、操作映射         │
└────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌────────────────────────────────────────────────────────────┐
│  Layer 1: Function（函数）                                  │
│  具体的业务操作实现，输入输出 Schema、权限、语义            │
└────────────────────────────────────────────────────────────┘
```

## Layer 1: Function（函数）

函数是系统中最小的可执行单元，代表一个具体的业务操作。

### 函数定义

```json
{
  "id": "player.ban",
  "name": "封禁玩家",
  "description": "封禁指定玩家账号",
  "category": "player",
  "risk_level": "high",
  "params": {
    "type": "object",
    "properties": {
      "player_id": {
        "type": "string",
        "title": "玩家ID"
      },
      "duration": {
        "type": "integer",
        "title": "封禁时长（小时）"
      },
      "reason": {
        "type": "string",
        "title": "封禁原因"
      }
    },
    "required": ["player_id", "duration"]
  },
  "result": {
    "type": "object",
    "properties": {
      "success": {"type": "boolean"},
      "ban_id": {"type": "string"}
    }
  },
  "auth": {
    "permission": "player.ban",
    "approval": {
      "enabled": true,
      "threshold": 2
    }
  }
}
```

### 函数属性

| 属性 | 类型 | 说明 |
|------|------|------|
| `id` | string | 函数唯一标识 |
| `name` | string | 函数显示名称 |
| `category` | string | 函数分类 |
| `risk_level` | string | 风险等级：low/medium/high |
| `params` | Schema | 输入参数定义 |
| `result` | Schema | 返回值定义 |
| `auth` | object | 权限配置 |

## Layer 2: Entity（实体）

实体是业务对象的完整描述，包含数据结构、验证规则和操作映射。

### 实体定义

```json
{
  "id": "player",
  "name": "玩家",
  "description": "玩家实体定义",
  "schema": {
    "type": "object",
    "properties": {
      "player_id": {
        "type": "string",
        "title": "玩家ID",
        "ui": {"readonly": true}
      },
      "username": {
        "type": "string",
        "title": "用户名",
        "minLength": 3,
        "maxLength": 16
      },
      "nickname": {
        "type": "string",
        "title": "昵称"
      },
      "level": {
        "type": "integer",
        "title": "等级"
      },
      "vip_level": {
        "type": "integer",
        "title": "VIP等级"
      },
      "status": {
        "type": "string",
        "title": "状态",
        "enum": ["normal", "banned", "deleted"]
      }
    }
  },
  "operations": {
    "create": {
      "function": "player.register",
      "label": "注册玩家"
    },
    "read": {
      "function": "player.get",
      "label": "查看详情"
    },
    "update": {
      "function": "player.update",
      "label": "更新信息"
    },
    "delete": {
      "function": "player.ban",
      "label": "封禁",
      "confirm": "确认封禁该玩家？"
    }
  },
  "ui": {
    "display_field": "username",
    "title_template": "{username} ({nickname})"
  }
}
```

### 实体属性

| 属性 | 类型 | 说明 |
|------|------|------|
| `schema` | Schema | JSON Schema 数据定义 |
| `operations` | object | CRUD 操作映射 |
| `ui.display_field` | string | 主显示字段 |
| `ui.title_template` | string | 标题模板 |

## Layer 3: Resource（资源）

资源是 UI 层面的操作集合，将多个函数组合成完整的管理界面。

### 资源定义

```json
{
  "id": "player.resource",
  "type": "pro-table",
  "name": "玩家管理",
  "entity": "player",
  "ui": {
    "type": "pro-table",
    "columns": [
      {
        "dataIndex": "player_id",
        "title": "玩家ID",
        "width": 120,
        "fixed": "left"
      },
      {
        "dataIndex": "username",
        "title": "用户名",
        "width": 150,
        "searchable": true
      },
      {
        "dataIndex": "nickname",
        "title": "昵称",
        "width": 150
      },
      {
        "dataIndex": "level",
        "title": "等级",
        "width": 80,
        "sortable": true
      },
      {
        "dataIndex": "status",
        "title": "状态",
        "width": 100,
        "valueEnum": {
          "normal": {"text": "正常", "status": "Success"},
          "banned": {"text": "封禁", "status": "Error"},
          "deleted": {"text": "删除", "status": "Default"}
        }
      }
    ],
    "actions": [
      {
        "type": "create",
        "operation": "create",
        "label": "新建玩家"
      },
      {
        "type": "edit",
        "operation": "update",
        "label": "编辑"
      },
      {
        "type": "delete",
        "operation": "delete",
        "label": "封禁",
        "confirm": "确认封禁该玩家？"
      }
    ],
    "toolbar": [
      {
        "type": "export",
        "label": "导出"
      }
    ]
  }
}
```

### 资源类型

| 类型 | 说明 | 组件 |
|------|------|------|
| `pro-table` | 列表页面 | ProTable |
| `pro-form` | 表单页面 | ProForm |
| `pro-descriptions` | 详情页面 | ProDescriptions |

## Layer 4: Component（组件）

组件是功能模块的打包单位，包含相关的函数、实体和资源。

### 组件定义（manifest.json）

```json
{
  "id": "player-management",
  "name": "玩家管理",
  "version": "1.0.0",
  "description": "玩家管理功能模块",
  "dependencies": [],
  "author": "Croupier Team",
  "functions": [
    {"id": "player.register"},
    {"id": "player.get"},
    {"id": "player.update"},
    {"id": "player.ban"},
    {"id": "player.unban"},
    {"id": "player.list"}
  ],
  "entities": [
    {"id": "player"}
  ],
  "resources": [
    {"id": "player.resource"}
  ]
}
```

### 组件打包

组件被打包成 `.tgz` 文件：

```
player-management-1.0.0.tgz
├── manifest.json
└── descriptors/
    ├── player.entity.json
    ├── player.resource.json
    ├── player.register.json
    ├── player.get.json
    ├── player.update.json
    ├── player.ban.json
    └── player.list.json
```

## UI 自动生成

基于定义自动生成：

### 列表页面

```typescript
// 自动生成 ProTable 配置
const tableConfig = {
  columns: resource.ui.columns,
  request: async (params) => {
    return await invokeFunction('player.list', params);
  },
  toolBarRender: () => [
    <Button type="primary">新建</Button>
  ]
};
```

### 表单页面

```typescript
// 基于 Schema 自动生成表单
const formConfig = {
  schema: entity.schema,
  onFinish: async (values) => {
    return await invokeFunction('player.update', values);
  }
};
```

## 对象绑定机制

函数通过 `entity` 字段绑定到业务对象：

```json
{
  "id": "player.ban",
  "entity": {
    "name": "player",
    "operation": "delete"
  }
}
```

这种绑定使得：
1. UI 可以自动识别函数属于哪个实体
2. 可以批量生成 CRUD 操作
3. 支持通用的权限控制

## 目录结构

```
descriptors/
├── player.entity.json             # 玩家实体
├── player.resource.json           # 玩家资源
├── player.register.json           # 注册函数
├── player.get.json                # 获取函数
├── player.update.json             # 更新函数
├── player.ban.json                # 封禁函数
├── item.entity.json
├── item.resource.json
└── item.list.json
│       ├── item.entity.json
│       └── ...
└── economy-system/
    ├── manifest.json
    └── descriptors/
        ├── currency.resource.json
        └── ...
```

## 相关文档

- [函数管理](./function-management.md)
- [函数管理](./function-management.md)
- [系统架构](../../architecture/)
