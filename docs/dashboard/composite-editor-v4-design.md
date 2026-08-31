---
title: 组合页编辑器 V4 — 组件模板与三层组合
---

# 组合页编辑器 V4 — 组件模板与三层组合

> 状态：设计中（本文是 V4 的设计文档，实现见后续任务清单）
> 前置：V3 已上线（组件化编辑器 + 通用事件 + 动作链），见 [V3 使用指南](./composite-editor-v3.md)
> 参考产品：Appsmith Module、Retool Custom Component、amis 组合渲染器

## 1. 核心概念

用户提出的分层组合模型：

```text
第一层  函数（已有）
        SDK 注册的原子能力：player.list、mail.send、inventory.grant…

第二层  组件模板（V4 新增——核心缺失）
        多个函数 + 布局 + 联动 → 封装为可复用组件

第三层  页面（已有）
        多个组件 → 组合成复杂工作台页面
```

**为什么需要第二层**：当前从零搭建一个"玩家管理"页面需要拖 3-5 个函数、配弹窗、配行操作、配联动——每次都重复。封装后一次拖入即得到完整功能块，然后在页面上组合多个功能块。

## 2. 组件模板模型

### 2.1 数据结构

```go
// ComponentTemplate 是可复用的页面组件模板（V4 核心实体）。
type ComponentTemplate struct {
    gorm.Model
    // 唯一标识（如 player-management）
    Key string `gorm:"size:128;uniqueIndex"`
    // 显示名（LocalizedText）
    Name JSON `gorm:"type:json"`
    // 描述
    Description JSON `gorm:"type:json"`
    // 分类：运营 / 客服 / 数据 / 配置 / 自定义
    Category string `gorm:"size:32;index"`
    // 图标（antd icon 名）
    Icon string `gorm:"size:64"`
    // 依赖的函数 ID 列表（拖入时检查 scope 可用性）
    RequiredFunctions JSON `gorm:"type:json"`
    // 页面组件子树（与编辑器 PageNode 同构）
    Tree JSON `gorm:"type:json"`
    // 是否为内置模板（契约自动生成 vs 用户保存）
    Builtin bool `gorm:"default:false"`
    // 创建者
    CreatedBy string `gorm:"size:64"`
}
```

### 2.2 Tree 结构（与编辑器 PageNode 同构）

```json
{
  "key": "player-management",
  "name": { "zh-CN": "玩家管理", "en-US": "Player Management" },
  "description": "搜索玩家→查看详情→执行操作",
  "category": "运营",
  "requiredFunctions": ["player.list", "player.get", "mail.send"],
  "tree": [
    {
      "id": "table",
      "type": "fnTable",
      "props": {
        "functionId": "player.list",
        "title": "玩家列表",
        "span": 24,
        "autoRun": true,
        "columns": ["uid", "nickname", "level", "vip"],
        "rowActions": [
          {
            "label": "发邮件",
            "targetSection": "mail-modal",
            "params": { "player_id": "uid" }
          }
        ]
      },
      "events": {
        "onRowClick": { "kind": "openModal", "target": "detail-modal" }
      }
    },
    {
      "id": "detail-modal",
      "type": "modal",
      "props": { "title": "玩家详情", "width": "medium" },
      "children": [
        {
          "id": "detail-fields",
          "type": "fnFields",
          "props": { "functionId": "player.get", "span": 24 }
        }
      ]
    },
    {
      "id": "mail-modal",
      "type": "modal",
      "props": { "title": "发送邮件", "width": "medium" },
      "children": [
        {
          "id": "mail-form",
          "type": "fnForm",
          "props": {
            "functionId": "mail.send",
            "span": 24,
            "onSuccess": { "kind": "refreshNode", "target": "table" }
          }
        }
      ]
    }
  ]
}
```

### 2.3 内置模板自动生成

当 SDK 注册的函数契约具有完整 CRUD 能力时（`collection_query` + `item_query` + `item_update` + `item_delete`），自动生成「资源管理」组件模板：

```text
资源管理组件 = CRUD 表格 + 详情弹窗 + 新建/编辑弹窗 + 删除确认
```

这消除了最高频的重复搭建场景。生成规则：

| 契约能力                      | 生成的组件节点                           |
| ----------------------------- | ---------------------------------------- |
| `collection_query`            | fnTable（列=输出 schema，autoRun，分页） |
| `item_query`                  | fnFields（详情弹窗内，行点击触发）       |
| `item_create` / `item_update` | fnForm（新建/编辑弹窗，顶部按钮触发）    |
| `item_delete`                 | 行操作（确认弹窗，danger）               |

## 3. 编辑器改造

### 3.1 左栏改为三 Tab

```text
┌─ 左栏 ────────────────────────┐
│ ┌────┐ ┌────┐ ┌────┐         │
│ │组件库│ │函数│ │大纲│         │
│ └────┘ └────┘ └────┘         │
│ ┌──────────────────────────┐ │
│ │ 🔍 搜索组件…              │ │
│ ├──────────────────────────┤ │
│ │ 📦 玩家管理     [运营]    │ │
│ │    搜索→详情→操作          │ │
│ │ 📦 邮件管理     [运营]    │ │
│ │    模板列表→发送→记录      │ │
│ │ 📦 数据面板     [数据]    │ │
│ │    多图表+筛选             │ │
│ │ 📦 配置管理     [配置]    │ │
│ │    CRUD+版本回滚          │ │
│ │ ─ ─ ─ 自定义 ─ ─ ─        │ │
│ │ 📦 我的组件 A             │ │
│ └──────────────────────────┘ │
└──────────────────────────────┘
```

### 3.2 拖入组件 → 实例化

```text
从组件库拖入「玩家管理」→
  1. 检查 scope：requiredFunctions 是否全部可用
  2. 复制 tree 子树，重新分配所有节点 id
  3. 重映射内部引用（events/rowActions/onSuccess 的 target）
  4. 整棵子树插入画布（可继续编辑，不是黑盒）
```

### 3.3 选中多节点 → 保存为组件

```text
画布上 Shift 点选多个节点 →
  右键 → 「保存为组件」→
  弹出表单：组件名 / 描述 / 分类 / 图标 →
  序列化选中的节点子树（含引用关系）→
  存入 ComponentTemplate（builtin=false）
```

### 3.4 组件实例展开编辑

组件拖入后是**普通节点**（非黑盒引用）——用户可以直接修改内部任何配置。这是与 Appsmith Module 的关键差异（Appsmith 的 Module 是引用，修改需要回到 Module 定义处）。

选择展开而非引用的理由：

- 游戏后台页面通常只需要微调（改列、改文案），不需要全局同步
- 避免引用带来的版本管理复杂度（组件改了，已发布页面是否自动更新？）
- 保持编辑器模型简单（只有 PageNode 树，没有引用间接层）

## 4. API 设计

### 4.1 组件模板 CRUD

```text
GET    /api/v1/component-templates           # 列表（含内置+自定义）
GET    /api/v1/component-templates/:key      # 详情
POST   /api/v1/component-templates           # 创建（用户保存）
PUT    /api/v1/component-templates/:key      # 更新
DELETE /api/v1/component-templates/:key      # 删除（内置不可删）
```

### 4.2 预置模板生成

```text
POST /api/v1/component-templates/regenerate  # 手动触发契约扫描→生成
                                             #（agent 注册时自动触发）
```

### 4.3 拖入时检查

```text
POST /api/v1/component-templates/:key/check  # 检查当前 scope 可用性
→ { "available": true/false, "missing": ["mail.send"] }
```

## 5. 任务清单

| #        | 任务       | 内容                                               | 预估   |
| -------- | ---------- | -------------------------------------------------- | ------ |
| V4-1     | 后端模型   | ComponentTemplate 表 + CRUD API + 契约扫描自动生成 | 1d     |
| V4-2     | 组件库面板 | 左栏 Tab + 搜索/分类/描述展示 + 拖入               | 0.5d   |
| V4-3     | 实例化     | tree 复制 + id 重分配 + 引用重映射 + scope 检查    | 0.5d   |
| V4-4     | 保存为组件 | 选中多节点 → 右键 → 序列化保存                     | 0.5d   |
| V4-5     | 预置模板   | CRUD 资源→「资源管理」组件（契约推导）             | 1d     |
| V4-6     | 集成测试   | 拖入→编辑→保存→发布全链 + round-trip               | 0.5d   |
| **合计** |            |                                                    | **4d** |

## 6. 与 V3 的关系

V4 是 V3 的**扩展**而非替代：

- V3 的所有功能（事件/动作链/分组弹窗/回读）不变
- 组件库面板新增在左栏（与函数/大纲并列）
- 保存的组件模板可包含 V3 的所有高级特性（事件/链/分组）
- 编译/发布链零改动（组件实例化后就是普通 PageNode 树）
