# Workspace 用户指南

## 1. 简介

### 什么是 Workspace

Workspace 是 Croupier Dashboard 的核心功能，它允许你通过可视化编排的方式，快速创建和管理游戏后台的各种管理界面。

### 核心功能

- **可视化编排**：通过拖拽和配置，无需编写代码即可创建管理界面
- **配置驱动**：所有界面由配置决定，修改配置即可改变界面
- **灵活布局**：支持多种布局类型，满足不同的管理需求
- **实时预览**：配置修改后立即看到效果

### 能力状态图例

- `✅` 稳定能力：主链路可用，默认推荐
- `🧪` 增强能力：已可用，持续优化中
- `❌` 暂不支持：当前版本不建议使用

### 使用场景

- **玩家管理**：查询玩家信息、修改玩家数据、封禁解封等
- **订单管理**：查看订单列表、订单详情、订单处理等
- **道具管理**：道具列表、道具发放、道具回收等
- **活动管理**：活动配置、活动数据查看等

## 2. 快速开始

### 访问 Workspace

1. 登录 Croupier Dashboard
2. 在左侧菜单中找到"控制台"
3. 点击对应的对象（如"玩家管理"）
4. 进入 Workspace 页面

### 查看现有配置

访问 Workspace 页面后，系统会自动加载配置并渲染界面。如果没有配置，会显示默认界面。

### 基本操作

- **切换 Tab**：点击顶部的标签页切换不同的功能模块
- **查询数据**：在查询表单中输入条件，点击"查询"按钮
- **查看详情**：在列表中点击行，或在详情页面查看完整信息
- **执行操作**：点击操作按钮执行相应的功能

## 3. 使用编排器

### 打开编排器

1. 访问 `/workspace-editor/:objectKey`（如 `/workspace-editor/player`）
2. 或在 Workspace 页面点击"编辑配置"按钮

### 编排器界面

编排器采用三栏布局：

```
┌─────────────┬─────────────────────┬─────────────┐
│             │                     │             │
│  函数列表    │    布局设计器        │  实时预览    │
│             │                     │             │
│  - 可搜索   │  - 添加 Tab         │  - 预览效果  │
│  - 可拖拽   │  - 配置布局         │  - 查看代码  │
│             │  - 配置函数         │             │
│             │                     │             │
└─────────────┴─────────────────────┴─────────────┘
```

### 添加 Tab

1. 点击"添加 Tab"按钮
2. 输入标题和图标（可选）
3. 点击"确定"

### 配置布局

1. 选择布局类型：

   - **表单-详情**：先查询，再显示详情和操作
   - **列表**：显示数据列表
   - **表单**：数据提交表单
   - **详情**：只读详情展示

2. 配置布局参数：
   - 列表布局：配置列表函数、列配置
   - 表单-详情：配置查询函数、详情分区、操作按钮
   - 表单布局：配置提交函数、表单字段
   - 详情布局：配置详情函数、详情分区

### 使用模板

1. 点击"使用模板"按钮
2. 选择合适的模板：
   - **基础列表**：简单的列表页面
   - **表单-详情**：查询和详情展示
   - **完整管理**：包含列表、详情、操作的完整界面
3. 点击"使用"应用模板

### 保存配置

1. 完成配置后，点击"保存配置"按钮
2. 系统会验证配置并保存
3. 保存成功后，访问 Workspace 页面即可看到效果

## 4. 布局类型

### Tabs 布局

多标签页组织，适合将多个功能模块组织在一起。

**示例**：

```json
{
  "type": "tabs",
  "tabs": [
    {
      "key": "info",
      "title": "玩家信息",
      "layout": { ... }
    },
    {
      "key": "list",
      "title": "玩家列表",
      "layout": { ... }
    }
  ]
}
```

### FormDetail 布局

先通过表单查询，然后显示详情和操作按钮。

**适用场景**：

- 玩家信息查询
- 订单详情查询
- 道具信息查询

**示例**：

```json
{
  "type": "form-detail",
  "queryFunction": "player.getInfo",
  "queryFields": [{ "key": "playerId", "label": "玩家ID", "type": "input", "required": true }],
  "detailSections": [
    {
      "title": "基本信息",
      "fields": [
        { "key": "playerId", "label": "玩家ID" },
        { "key": "nickname", "label": "昵称" },
        { "key": "level", "label": "等级" }
      ]
    }
  ],
  "actions": [
    {
      "key": "updateLevel",
      "label": "更新等级",
      "function": "player.updateLevel"
    }
  ]
}
```

### List 布局

显示数据列表，支持分页、搜索、行操作等。

**适用场景**：

- 玩家列表
- 订单列表
- 道具列表

**示例**：

```json
{
  "type": "list",
  "listFunction": "player.list",
  "columns": [
    { "key": "playerId", "title": "玩家ID", "width": 150 },
    { "key": "nickname", "title": "昵称", "width": 150 },
    { "key": "level", "title": "等级", "width": 100 },
    { "key": "gold", "title": "金币", "width": 120, "render": "money" }
  ]
}
```

### Form 布局

数据提交表单。

**适用场景**：

- 创建玩家
- 发送邮件
- 发放道具

**示例**：

```json
{
  "type": "form",
  "submitFunction": "player.create",
  "fields": [
    { "key": "nickname", "label": "昵称", "type": "input", "required": true },
    { "key": "level", "label": "等级", "type": "number", "required": true }
  ]
}
```

### Detail 布局

只读详情展示。

**适用场景**：

- 系统信息
- 配置详情
- 统计数据

**示例**：

```json
{
  "type": "detail",
  "detailFunction": "system.info",
  "sections": [
    {
      "title": "系统信息",
      "fields": [
        { "key": "version", "label": "版本" },
        { "key": "uptime", "label": "运行时间" }
      ]
    }
  ]
}
```

## 5. 高级功能

### 自定义渲染

支持自定义字段渲染方式：

- **status**：状态徽章
- **datetime**：日期时间格式化
- **date**：日期格式化
- **tag**：标签
- **money**：金额格式化
- **link**：链接
- **image**：图片

**示例**：

```json
{
  "key": "status",
  "label": "状态",
  "render": "status",
  "renderOptions": {
    "statusMap": {
      "1": { "text": "启用", "status": "success" },
      "0": { "text": "禁用", "status": "default" }
    }
  }
}
```

### 权限控制

可以为 Workspace、Tab、操作按钮配置权限：

```json
{
  "objectKey": "player",
  "permissions": ["player.view"],
  "layout": {
    "type": "tabs",
    "tabs": [
      {
        "key": "info",
        "permissions": ["player.info"],
        "layout": {
          "type": "form-detail",
          "actions": [
            {
              "key": "ban",
              "label": "封禁",
              "permissions": ["player.ban"]
            }
          ]
        }
      }
    ]
  }
}
```

### 配置导入导出

**导出配置**：

1. 在编排器中点击"导出配置"
2. 保存 JSON 文件

**导入配置**：

1. 在编排器中点击"导入配置"
2. 选择 JSON 文件
3. 系统会验证配置并导入

## 6. 常见问题

### 配置不生效怎么办？

1. 检查配置是否保存成功
2. 刷新页面重新加载配置
3. 检查浏览器控制台是否有错误
4. 检查配置格式是否正确

### 如何调试？

1. 在编排器中切换到"代码"视图，查看配置 JSON
2. 使用浏览器开发者工具查看网络请求
3. 查看控制台错误信息
4. 使用"全屏预览"功能测试效果

### 性能优化建议

1. **减少列数**：列表布局不要配置过多列
2. **使用分页**：大数据量时启用分页
3. **合理使用缓存**：配置会自动缓存 5 分钟
4. **避免复杂渲染**：尽量使用简单的渲染方式

## 7. 最佳实践

### 命名规范

- **objectKey**：使用小写字母和下划线，如 `player`、`game_order`
- **Tab key**：使用有意义的英文，如 `info`、`list`、`settings`
- **字段 key**：与后端返回的字段名保持一致

### 布局设计建议

1. **首页使用列表**：方便快速浏览数据
2. **详情页分区展示**：将相关信息分组
3. **操作按钮分类**：常用操作放在前面，危险操作用红色
4. **合理使用 Tab**：不要创建过多 Tab，一般 3-5 个为宜

### 性能优化技巧

1. **懒加载**：只在需要时加载数据
2. **缓存配置**：利用配置缓存减少请求
3. **虚拟滚动**：大列表使用虚拟滚动
4. **防抖节流**：搜索输入使用防抖

## 8. 快捷键

| 快捷键     | 功能     |
| ---------- | -------- |
| `Ctrl + S` | 保存配置 |
| `Ctrl + P` | 预览配置 |
| `Ctrl + E` | 导出配置 |
| `Ctrl + I` | 导入配置 |

## 9. 获取帮助

- 📖 [开发文档](./WORKSPACE_DEV_GUIDE.md)
- 📖 [API 文档](./WORKSPACE_API.md)
- 💬 [讨论区](https://github.com/cuihairu/croupier/discussions)
- 🐛 [问题反馈](https://github.com/cuihairu/croupier/issues)

## 10. 更新日志

### v1.0.0 (2026-03-06)

- ✅ 完成基础架构
- ✅ 实现 Layout Engine
- ✅ 实现可视化编排工具
- ✅ 支持稳定布局（form-detail/list/form/detail）
- 🧪 支持增强布局（kanban/timeline/split/wizard/dashboard/grid/custom）
- ✅ 配置缓存机制
- ✅ 实时预览功能
