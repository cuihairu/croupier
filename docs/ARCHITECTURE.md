---
title: 架构设计
icon: sitemap
order: 1
category:
  - 架构设计
tag:
  - 架构
  - 对象驱动
---

# Croupier 对象驱动的组件化后台管理系统

## 📋 概述

基于 JSON Schema 驱动的对象化游戏后台管理系统，实现了类似 Ant Design Pro 的低代码/无代码管理界面生成能力。

## 🏗️ 核心架构

### 三层架构设计

```
┌─────────────────────────────────────────────────────────┐
│                    UI Resource Layer                   │
│  ┌─────────────────┐  ┌─────────────────┐              │
│  │ player.resource │  │  item.resource  │  ...         │
│  │   (ProTable)    │  │   (ProTable)    │              │
│  └─────────────────┘  └─────────────────┘              │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                   Entity Definition Layer               │
│  ┌─────────────────┐  ┌─────────────────┐              │
│  │  player.entity  │  │   item.entity   │  ...         │
│  │  (JSON Schema)  │  │  (JSON Schema)  │              │
│  └─────────────────┘  └─────────────────┘              │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                    Function Layer                      │
│  ┌─────────────────┐  ┌─────────────────┐              │
│  │ player.register │  │  item.create    │  ...         │
│  │ player.profile  │  │  item.list      │              │
│  │ player.ban      │  │  item.update    │              │
│  └─────────────────┘  └─────────────────┘              │
└─────────────────────────────────────────────────────────┘
```

### 核心概念

#### 1. Entity (实体)
- **定义**: 业务对象的完整描述，包含 JSON Schema、UI 配置、操作映射
- **文件**: `*.entity.json`
- **作用**: 定义数据结构、验证规则、UI 展示方式
- **示例**: `player.entity.json`, `item.entity.json`

#### 2. Function (函数)
- **定义**: 具体的业务操作实现，包含输入输出 Schema、权限、语义
- **文件**: `*.json` (非 entity/resource 类型)
- **作用**: 实际的 CRUD 操作逻辑
- **示例**: `player.register.json`, `item.create.json`

#### 3. Resource (资源)
- **定义**: UI 层面的操作集合，将多个函数组合成完整的管理界面
- **文件**: `*.resource.json`
- **作用**: 定义 ProTable 配置、列定义、操作按钮
- **示例**: `player.resource.json`, `item.resource.json`

#### 4. Component (组件)
- **定义**: 功能模块的打包单位，包含相关的 entities、functions、resources
- **文件**: `manifest.json`
- **作用**: 模块化管理、依赖控制、安装/卸载
- **示例**: `player-management`, `item-management`

## 🔧 技术实现

### JSON Schema 驱动

使用 JSON Schema 作为核心描述语言：

```json
{
  "type": "object",
  "properties": {
    "username": {
      "type": "string",
      "pattern": "^[a-zA-Z0-9_]{3,16}$",
      "description": "用户名",
      "searchable": true
    }
  }
}
```

### 对象绑定机制

函数通过 `entity` 字段绑定到业务对象：

```json
{
  "id": "player.register",
  "entity": {
    "name": "player",
    "operation": "create"
  }
}
```

### UI 自动生成

基于 Entity 定义自动生成：
- **ProTable**: 列表页面
- **ProForm**: 表单页面
- **ProDescriptions**: 详情页面

## 📁 目录结构

```
components/
├── player-management/
│   ├── manifest.json              # 组件清单
│   └── descriptors/
│       ├── player.entity.json     # 玩家实体定义
│       ├── player.resource.json   # 玩家资源配置
│       ├── player.register.json   # 注册函数
│       ├── player.profile.get.json
│       ├── player.profile.update.json
│       ├── player.ban.json
│       ├── player.unban.json
│       └── player.list.json
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
└── economy-system/
    ├── manifest.json
    └── descriptors/
        ├── currency.resource.json
        ├── wallet.add.json
        ├── wallet.deduct.json
        └── wallet.get.json
```

## 🔄 工作流程

### 1. 定义实体 (Entity)
```json
{
  "id": "player.entity",
  "type": "entity",
  "schema": { /* JSON Schema */ },
  "operations": {
    "create": ["player.register"],
    "read": ["player.profile.get"],
    "update": ["player.profile.update"],
    "delete": ["player.ban"]
  },
  "ui": {
    "display_field": "username",
    "title_template": "{username} ({nickname})"
  }
}
```

### 2. 实现函数 (Functions)
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

### 3. 配置资源 (Resource)
```json
{
  "id": "player.resource",
  "type": "resource",
  "operations": {
    "create": {"function": "player.register"},
    "read": {"function": "player.profile.get"}
  },
  "ui": {
    "type": "pro-table",
    "columns": [ /* 列定义 */ ],
    "actions": ["create", "edit", "delete"]
  }
}
```

### 4. 打包组件 (Component)
```json
{
  "id": "player-management",
  "dependencies": [],
  "functions": [
    {"id": "player.register"},
    {"id": "player.profile.get"}
  ]
}
```

## 🎯 系统优势

### 1. 低代码/无代码
- **可视化配置**: 通过管理界面创建和编辑实体
- **自动生成UI**: 基于 JSON Schema + UI Schema（Formily）自动生成表单和表格
- **即插即用**: 组件化安装和卸载

### 2. 类型安全
- **输入验证**: JSON Schema 验证所有输入
- **输出类型**: 明确的返回值类型定义
- **编译时检查**: 前端可以基于 Schema 生成 TypeScript 类型

### 3. 权限控制
- **细粒度**: 函数级别的权限控制
- **灵活策略**: 支持 RBAC 和 ABAC
- **条件权限**: `allow_if` 表达式支持

### 4. 可扩展性
- **组件化**: 模块独立开发和部署
- **依赖管理**: 自动解析组件依赖关系
- **版本控制**: 组件和函数的版本管理

## 📱 前端集成

### ProTable 组件
```typescript
<ResourceTable
  resourceId="player.resource"
  // 自动读取 /api/v1/functions/descriptors?type=player
  // 根据 operations 和 ui 配置生成界面
/>
```

### 自动表单生成
```typescript
<EntityForm
  entityId="player.entity"
  operation="create"
  // 基于 player.entity.json 生成表单
  // 调用 player.register 函数
/>
```

## 🔗 API 接口

### 组件管理
- `GET /api/components` - 获取组件列表
- `POST /api/components/install` - 安装组件
- `DELETE /api/components/:id` - 卸载组件
- `POST /api/components/:id/enable` - 启用组件
- `POST /api/components/:id/disable` - 禁用组件

### 实体管理（已提供基础能力，待增强）
- `GET /api/v1/entities` - 获取实体列表
- `POST /api/v1/entities` - 创建新实体
- `GET /api/v1/entities/:id` - 获取实体详情
- `PUT /api/v1/entities/:id` - 更新实体定义
- `DELETE /api/v1/entities/:id` - 删除实体
- `GET /api/v1/entities/:id/preview` - 预览实体 UI
- `POST /api/v1/entities/validate` - 校验实体定义（JSON Schema）

### 函数调用
- `POST /api/invoke` - 调用函数
- `GET /api/v1/providers/descriptors` - 获取 Provider Manifest/统一描述符视图
- `GET /api/ui_schema` - 获取 UI Schema

## 🚀 实现状态

### ✅ 已完成
1. **组件管理系统**
   - ComponentManager 实现
   - 组件安装/卸载/启用/禁用
   - 依赖关系检查
   - HTTP API 集成

2. **玩家管理组件**
   - 完整的 CRUD 操作
   - 实体定义 (player.entity.json)
   - 资源配置 (player.resource.json)
   - 所有函数描述符

3. **物品管理组件**
   - 基础 CRUD 操作
   - 实体定义 (item.entity.json)
   - 资源配置 (item.resource.json)
   - 部分函数描述符

4. **经济系统组件**
   - 货币管理资源配置
   - 基础架构

### 🔄 进行中
1. **实体管理界面**
   - 可视化实体创建和编辑
   - JSON Schema 编辑器
   - UI 配置工具
   - 预览功能

### 📋 待实现
详见 `server/todo.md`（按 P0/P1… 分级的未完成项汇总）。

## 🎯 长期目标

1. **完整的低代码平台**
   - 拖拽式界面设计器
   - 工作流编排
   - 数据源连接器

2. **多租户支持**
   - 租户级别的组件隔离
   - 数据隔离
   - 权限隔离

3. **插件生态**
   - 第三方组件市场
   - 组件模板库
   - 社区贡献机制

---

这个架构为游戏后台管理提供了强大的扩展性和可维护性，同时大大降低了新功能开发的门槛。
