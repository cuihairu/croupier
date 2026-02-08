# Player 函数 Pack 配置示例

本示例展示如何使用 **OpenAPI 3.0.3** 标准定义函数。

## 📁 目录结构

```
player/
├── openapi.yaml    # OpenAPI 3.0.3 规范（函数定义）
├── pack.sh         # 打包脚本
└── README.md       # 本文档
```

## 🚀 快速开始

### 1. 打包 Pack

```bash
cd /Users/cui/Workspaces/croupier/croupier/packs/player
./pack.sh
```

打包后会生成 `player.pack.tgz` 文件。

### 2. 导入到系统

```bash
curl -X POST http://localhost:8080/api/v1/functions/_import \
  -H "Content-Type: application/json" \
  -H "X-Game-ID: your-game-id" \
  -d @openapi.yaml
```

或通过 Pack API：

```bash
curl -X POST http://localhost:8080/api/v1/packs/import \
  -F "pack=@player.pack.tgz" \
  -H "X-Game-ID: your-game-id"
```

## 📋 OpenAPI 3.0.3 规范

### 基本结构

```yaml
openapi: 3.0.3
info:
  title: Player Functions Pack
  version: 1.0.0
  x-category: player    # OpenAPI 扩展：函数分类
paths:
  /player/get:
    post:
      operationId: player.get
      summary: Get Player Info
      description: 根据玩家ID获取玩家详细信息
      x-category: player    # 函数分类
      x-risk: safe          # 风险级别：safe/warning/danger
      x-entity: Player      # 关联实体类型
      x-operation: read     # CRUD 操作：create/read/update/delete/custom
      tags:
        - player
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                player_id:
                  type: string
                  description: 玩家ID
              required:
                - player_id
      responses:
        '200':
          description: 成功响应
          content:
            application/json:
              schema:
                type: object
                properties:
                  data:
                    type: object
                    properties:
                      player_id:
                        type: string
                      name:
                        type: string
components: {}
```

## 🔧 OpenAPI 3.0.3 字段说明

### 扩展字段（x-* 前缀）

| 字段 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `x-category` | string | 函数分类 | player, item, system |
| `x-risk` | string | 风险级别 | safe, warning, danger |
| `x-entity` | string | 关联实体类型 | Player, Item, Guild |
| `x-operation` | string | CRUD 操作类型 | create, read, update, delete, custom |

### 标准字段

| 字段 | 说明 | 示例 |
|------|------|------|
| `operationId` | 唯一操作 ID | player.get |
| `summary` | 简短摘要 | "Get Player Info" |
| `description` | 详细描述 | 支持多语言 |
| `tags` | 分组标签 | ["player", "query"] |

## 📝 开发流程

1. **编辑 openapi.yaml**: 定义函数规范（OpenAPI 3.0.3）
2. **验证规范**: 使用 OpenAPI 验证工具检查语法
3. **打包**: `./pack.sh`
4. **导入**: 通过 API 导入 pack
5. **测试**: 访问函数目录，点击调用

## 🔍 调试技巧

### 验证 OpenAPI 规范

```bash
# 使用 Swagger CLI 验证
docker run --rm -v $(pwd):/spec openapitools/swagger-cli validate openapi.yaml
```

### 查看已导入的函数

```bash
curl http://localhost:8080/api/v1/functions
```

### 获取函数的 OpenAPI spec

```bash
curl http://localhost:8080/api/v1/functions/player.get/openapi
```

## 📚 相关文档

- [OpenAPI 3.0.3 规范](https://swagger.io/specification/)
- [函数系统文档](../../docs/function.md)
- [SDK 使用指南](../../docs/sdk.md)

---

**作者**: Croupier Team
**更新时间**: 2025-02-09
