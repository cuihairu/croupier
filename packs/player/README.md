# Player 函数 Pack 配置示例

本示例展示如何创建完整的函数配置，包括 **UI 配置**、**路由设置** 和 **权限管理**。

## 📁 目录结构

```
player/
├── descriptors/
│   └── player.get.json       # 函数定义（参数、权限、路由等）
├── ui/
│   └── player.get.uischema.json  # UI 配置（表单、布局等）
├── pack.sh                    # 打包脚本
└── README.md                  # 本文档
```

## 🚀 快速开始

### 1. 打包 Pack

```bash
cd /Users/cui/Workspaces/croupier/croupier/packs/player
./pack.sh
```

打包后会生成 `player.tgz` 文件。

### 2. 导入到系统

```bash
curl -X POST http://localhost:8080/api/v1/packs/import \
  -F "pack=@player.tgz" \
  -H "X-Game-ID: your-game-id"
```

### 3. 访问函数

- **函数目录**: `/game/functions/catalog`
- 找到 "获取玩家信息" 函数
- 点击 **▶️ 调用函数** 按钮
- 自动跳转到: `/game/player/get?fid=player.get`

## 📋 配置说明

### 函数定义 (player.get.json)

```json
{
  "id": "player.get",                    // 函数唯一ID
  "version": "1.0.0",                    // 版本号
  "category": "player",                  // 分类（用于自动分组）
  "display_name": {                      // 显示名称
    "zh": "获取玩家信息",
    "en": "Get Player Info"
  },
  "summary": {                           // 摘要说明
    "zh": "根据玩家ID获取玩家详细信息",
    "en": "Get player details by player ID"
  },
  "tags": ["玩家", "查询", "基础"],      // 标签
  "auth": {                              // 权限配置
    "permission": "player.get",
    "roles": ["admin", "gm", "operator"]
  },
  "params": {                            // 参数 JSON Schema
    "$schema": "https://json-schema.org/draft/2020-12/schema",
    "type": "object",
    "properties": {
      "player_id": {
        "type": "string",
        "title": "玩家ID"
      },
      "include_assets": {
        "type": "boolean",
        "title": "包含资产信息",
        "default": false
      }
    },
    "required": ["player_id"]
  },
  "menu": {                              // 路由配置 ⭐
    "section": "玩家管理",
    "group": "基础功能",
    "path": "/game/player/get",          // 自定义路由路径
    "order": 10,
    "hidden": false
  }
}
```

### UI 配置 (player.get.uischema.json)

```json
{
  "ui:layout": {                         // 布局配置
    "type": "grid",                      // grid | tabs
    "cols": 2                            // 列数
  },
  "ui:groups": [                         // 字段分组
    {
      "title": "基础信息",
      "fields": ["player_id"]
    },
    {
      "title": "高级选项",
      "fields": ["include_assets"]
    }
  ],
  "ui:fields": {                         // 字段配置
    "player_id": {
      "widget": "input",                 // input | textarea | number | select | switch | date
      "placeholder": "请输入玩家ID",
      "rules": [
        { "required": true, "message": "玩家ID不能为空" },
        { "min": 6, "message": "玩家ID至少6个字符" }
      ]
    },
    "include_assets": {
      "widget": "switch",
      "valuePropName": "checked"
    }
  }
}
```

## 🎨 UI Widget 类型

| Widget | 说明 | 适用场景 |
|--------|------|----------|
| `input` | 单行输入框 | 短文本（ID、名称等） |
| `textarea` | 多行文本 | 长文本（描述、备注等） |
| `number` | 数字输入 | 数值（金额、数量等） |
| `select` | 下拉选择 | 枚举值（状态、类型等） |
| `switch` | 开关 | 布尔值（是否启用等） |
| `checkbox` | 复选框 | 多选标签 |
| `date` | 日期选择 | 日期字段 |
| `date-time` | 日期时间 | 时间戳 |

## 🔐 权限配置

### 在 Descriptor 中定义

```json
{
  "auth": {
    "permission": "player.get",      // 权限标识
    "roles": ["admin", "gm"]         // 允许的角色
  }
}
```

### 通过 API 管理

```bash
# 查看权限
GET /api/v1/functions/player.get/permissions

# 更新权限
PUT /api/v1/functions/player.get/permissions
Content-Type: application/json

{
  "permissions": [
    {
      "resource": "player.get",
      "actions": ["read", "invoke"],
      "roles": ["admin", "gm", "operator"]
    }
  ]
}
```

## 🛣️ 路由配置

### 路由自动生成规则

1. **如果 `menu.path` 存在**: 使用自定义路径
   - 示例: `/game/player/get`

2. **如果 `menu.path` 不存在**: 使用默认路径
   - 格式: `/game/functions?fid={function_id}`
   - 示例: `/game/functions?fid=player.get`

3. **前端访问**:
   - 函数目录点击"调用函数"按钮 → 跳转到 `menu.path`
   - 直接在浏览器输入 URL 也可以访问

### 路由配置项

```json
{
  "menu": {
    "section": "玩家管理",           // 一级菜单（可选）
    "group": "基础功能",             // 二级分组（可选）
    "path": "/game/player/get",     // 路由路径 ⭐
    "order": 10,                    // 显示顺序
    "hidden": false                 // 是否在菜单隐藏
  }
}
```

## 📊 参数验证规则

```json
{
  "ui:fields": {
    "field_name": {
      "rules": [
        { "required": true, "message": "必填" },
        { "min": 6, "message": "最小长度" },
        { "max": 20, "message": "最大长度" },
        { "pattern": "^[a-zA-Z0-9]+$", "message": "只能包含字母和数字" }
      ]
    }
  }
}
```

## 🔧 高级配置

### Tabs 布局示例

```json
{
  "ui:layout": { "type": "tabs" },
  "ui:groups": [
    { "title": "基础信息", "fields": ["player_id", "name"] },
    { "title": "资产信息", "fields": ["gold", "diamond"] },
    { "title": "其他", "fields": ["memo"] }
  ]
}
```

### Select Widget 配置

```json
{
  "status": {
    "widget": "select",
    "placeholder": "请选择状态",
    "options": [
      { "label": "在线", "value": "online" },
      { "label": "离线", "value": "offline" },
      { "label": "封禁", "value": "banned" }
    ]
  }
}
```

## 📝 开发流程

1. **创建 descriptor**: 定义函数参数、权限、路由
2. **创建 ui schema**: 定义表单UI布局
3. **打包**: `./pack.sh`
4. **导入**: 通过 API 导入 pack
5. **测试**: 访问函数目录，点击调用
6. **迭代**: 修改配置后重新打包导入

## 🔍 调试技巧

### 查看已导入的函数

```bash
curl http://localhost:8080/api/v1/functions/descriptors
```

### 查看特定函数配置

```bash
curl http://localhost:8080/api/v1/functions/player.get
```

### 查看 UI 配置

```bash
curl http://localhost:8080/api/v1/functions/player.get/ui
```

## 📚 相关文档

- [函数 Pack 系统文档](../../CLAUDE.md)
- [JSON Schema 规范](https://json-schema.org/)
- [X-Render UI 文档](../../web/README.md)

---

**作者**: Croupier Team
**更新时间**: 2025-02-01
