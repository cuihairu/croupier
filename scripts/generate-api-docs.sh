#!/bin/bash

# Croupier API 文档生成脚本
# 这个脚本使用多种方法从 API 定义生成导入配置

set -e

echo "🚀 开始生成 Croupier API 文档..."

# 设置路径
SERVICES_DIR="services"
API_DIR="$SERVICES_DIR/api"
OUTPUT_DIR="."

# 检查必要工具
export PATH=$PATH:$HOME/go/bin

if ! command -v goctl &> /dev/null; then
    echo "❌ goctl 未安装，正在安装..."
    go install github.com/zeromicro/go-zero/tools/goctl@latest
fi

echo "✅ goctl 已就绪"

# 方法 1: 使用简化的 API 文件生成 Swagger/OpenAPI
echo ""
echo "📝 方法 1: 生成简化的 OpenAPI 规范..."

# 创建简化版本的 API 文件，只包含核心端点
cat > "$API_DIR/simple.api" << 'EOF'
syntax = "v1"

info (
	title:   "Croupier API"
	desc:    "Croupier 游戏管理系统 API"
	author:  "Croupier Team"
	email:   "team@croupier.com"
	version: "v1.0"
)

// 请求和响应类型
type LoginRequest {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse {
	Token string `json:"token"`
	User  UserInfo `json:"user"`
}

type UserInfo {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

type GenericResponse {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
}

// 服务定义
service croupier-api {
	// 认证相关
	@handler LoginHandler
	post /api/auth/sessions (LoginRequest) returns (LoginResponse)

	@handler LogoutHandler
	delete /api/auth/sessions returns (GenericResponse)

	// 用户相关
	@handler CurrentUserHandler
	get /api/users/current returns (UserInfo)

	@handler UserProfileHandler
	get /api/users/current/profile returns (UserInfo)

	// 游戏管理
	@handler GamesListHandler
	get /api/games returns (GenericResponse)

	@handler GameCreateHandler
	post /api/games returns (GenericResponse)

	// 系统运维
	@handler HealthHandler
	get /api/health returns (GenericResponse)

	@handler ServicesHandler
	get /api/services returns (GenericResponse)

	@handler NodesHandler
	get /api/nodes returns (GenericResponse)
}
EOF

echo "✅ 简化的 API 文件已创建"

# 生成 OpenAPI/Swagger 规范
echo "📄 生成 OpenAPI/Swagger 规范..."
cd "$API_DIR"
goctl api swagger --api simple.api --dir "$OUTPUT_DIR" --filename croupier-api

echo "✅ OpenAPI 规范已生成: croupier-api.json"

# 方法 2: 转换现有 Postman 集合为 OpenAPI
echo ""
echo "📝 方法 2: 转换 Postman 集合为 OpenAPI..."

# 如果有 Newman 工具，可以转换集合
if command -v newman &> /dev/null; then
    echo "使用 Newman 转换 Postman 集合..."
    # 这里可以添加 Newman 转换命令
else
    echo "⚠️  Newman 未安装，跳过转换"
fi

# 方法 3: 生成环境配置文件
echo ""
echo "📝 方法 3: 生成环境配置..."

# 生成环境配置
cat > "$OUTPUT_DIR/croupier-api-environments.json" << 'EOF'
{
  "id": "croupier-environments",
  "name": "Croupier API Environments",
  "values": [
    {
      "id": "dev-env",
      "name": "开发环境",
      "values": [
        {
          "key": "baseUrl",
          "value": "http://localhost:8888",
          "description": "API服务地址 - 本地开发"
        },
        {
          "key": "agentUrl",
          "value": "http://localhost:8889",
          "description": "Agent服务地址 - 本地开发"
        },
        {
          "key": "edgeUrl",
          "value": "http://localhost:8890",
          "description": "Edge服务地址 - 本地开发"
        },
        {
          "key": "token",
          "value": "",
          "description": "认证token，登录后自动获取"
        },
        {
          "key": "gameId",
          "value": "demo-game",
          "description": "测试游戏ID"
        },
        {
          "key": "env",
          "value": "development",
          "description": "开发环境标识"
        }
      ]
    },
    {
      "id": "test-env",
      "name": "测试环境",
      "values": [
        {
          "key": "baseUrl",
          "value": "http://test-api.croupier.com:8888",
          "description": "API服务地址 - 测试环境"
        },
        {
          "key": "agentUrl",
          "value": "http://test-agent.croupier.com:8889",
          "description": "Agent服务地址 - 测试环境"
        },
        {
          "key": "edgeUrl",
          "value": "http://test-edge.croupier.com:8890",
          "description": "Edge服务地址 - 测试环境"
        },
        {
          "key": "token",
          "value": "",
          "description": "认证token"
        },
        {
          "key": "gameId",
          "value": "test-game-001",
          "description": "测试游戏ID"
        },
        {
          "key": "env",
          "value": "testing",
          "description": "测试环境标识"
        }
      ]
    },
    {
      "id": "prod-env",
      "name": "生产环境",
      "values": [
        {
          "key": "baseUrl",
          "value": "https://api.croupier.com",
          "description": "API服务地址 - 生产环境"
        },
        {
          "key": "agentUrl",
          "value": "https://agent.croupier.com",
          "description": "Agent服务地址 - 生产环境"
        },
        {
          "key": "edgeUrl",
          "value": "https://edge.croupier.com",
          "description": "Edge服务地址 - 生产环境"
        },
        {
          "key": "token",
          "value": "",
          "description": "认证token"
        },
        {
          "key": "gameId",
          "value": "prod-game-001",
          "description": "生产环境游戏ID"
        },
        {
          "key": "env",
          "value": "production",
          "description": "生产环境标识"
        }
      ]
    }
  ]
}
EOF

echo "✅ 环境配置已生成: croupier-api-environments.json"

# 生成 APIfox 导入配置
echo ""
echo "📝 方法 4: 生成 APIfox 导入配置..."

cat > "$OUTPUT_DIR/croupier-apifox-collection.json" << 'EOF'
{
  "info": {
    "name": "Croupier API",
    "description": "Croupier Go-Zero 微服务API集合",
    "version": "1.0.0",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "item": [
    {
      "name": "认证服务",
      "item": [
        {
          "name": "创建会话 (登录)",
          "request": {
            "method": "POST",
            "header": [
              {
                "key": "Content-Type",
                "value": "application/json"
              }
            ],
            "body": {
              "mode": "raw",
              "raw": JSON.stringify({
                "username": "admin",
                "password": "admin123"
              }, null, 2)
            },
            "url": {
              "raw": "{{baseUrl}}/api/auth/sessions",
              "host": ["{{baseUrl}}"],
              "path": ["api", "auth", "sessions"]
            },
            "description": "RESTful: 创建会话而不是登录"
          }
        },
        {
          "name": "获取当前用户",
          "request": {
            "method": "GET",
            "header": [
              {
                "key": "Authorization",
                "value": "Bearer {{token}}"
              }
            ],
            "url": {
              "raw": "{{baseUrl}}/api/users/current",
              "host": ["{{baseUrl}}"],
              "path": ["api", "users", "current"]
            },
            "description": "RESTful: 获取当前用户信息"
          }
        }
      ]
    },
    {
      "name": "游戏管理",
      "item": [
        {
          "name": "获取游戏列表",
          "request": {
            "method": "GET",
            "header": [
              {
                "key": "Authorization",
                "value": "Bearer {{token}}"
              }
            ],
            "url": {
              "raw": "{{baseUrl}}/api/games?page=1&size=20",
              "host": ["{{baseUrl}}"],
              "path": ["api", "games"],
              "query": [
                {
                  "key": "page",
                  "value": "1"
                },
                {
                  "key": "size",
                  "value": "20"
                }
              ]
            }
          }
        }
      ]
    },
    {
      "name": "系统运维",
      "item": [
        {
          "name": "健康检查",
          "request": {
            "method": "GET",
            "header": [
              {
                "key": "Authorization",
                "value": "Bearer {{token}}"
              }
            ],
            "url": {
              "raw": "{{baseUrl}}/api/health",
              "host": ["{{baseUrl}}"],
              "path": ["api", "health"]
            }
          }
        },
        {
          "name": "服务状态",
          "request": {
            "method": "GET",
            "header": [
              {
                "key": "Authorization",
                "value": "Bearer {{token}}"
              }
            ],
            "url": {
              "raw": "{{baseUrl}}/api/services",
              "host": ["{{baseUrl}}"],
              "path": ["api", "services"]
            }
          }
        }
      ]
    }
  ],
  "variable": [
    {
      "key": "baseUrl",
      "value": "http://localhost:8888",
      "type": "string"
    },
    {
      "key": "token",
      "value": "",
      "type": "string"
    }
  ]
}
EOF

echo "✅ APIfox 集合已生成: croupier-apifox-collection.json"

# 生成使用说明
echo ""
echo "📋 生成使用说明..."

cat > "$OUTPUT_DIR/API_IMPORT_GUIDE.md" << 'EOF'
# Croupier API 导入指南

本文档提供了将 Croupier API 导入到各种 API 测试工具的方法。

## 📋 生成的文件

1. **croupier-api.json** - OpenAPI/Swagger 规范文件
2. **croupier-api-environments.json** - Postman 环境配置
3. **croupier-apifox-collection.json** - APIfox 集合文件
4. **croupier-api.postman_collection.json** - 完整的 Postman 集合

## 🚀 导入方法

### 1. Postman 导入

#### 方法一：导入 OpenAPI 规范
1. 打开 Postman
2. 点击 "Import" → "Link"
3. 输入文件路径或粘贴 JSON 内容
4. 选择 "OpenAPI 3.0"

#### 方法二：导入集合文件
1. 打开 Postman
2. 点击 "Import" → "Files"
3. 选择 `croupier-api.postman_collection.json`
4. 导入环境配置 `croupier-api-environments.json`

### 2. APIfox 导入

1. 打开 APIfox
2. 点击 "导入" → "OpenAPI/Swagger"
3. 上传 `croupier-api.json`
4. 或者选择 "Postman" 格式导入 `croupier-apifox-collection.json`

### 3. 其他工具

#### Swagger UI
```bash
# 使用 Docker 运行 Swagger UI
docker run -p 80:8080 -e SWAGGER_JSON=/croupier-api.json -v $(pwd)/croupier-api.json:/croupier-api.json swaggerapi/swagger-ui
```

#### Redoc
```bash
# 安装 Redoc CLI
npm install -g redoc-cli

# 生成 HTML 文档
redoc-cli build croupier-api.json
```

## 🔧 环境配置

### 开发环境 (本地)
- API 服务: http://localhost:8888
- Agent 服务: http://localhost:8889
- Edge 服务: http://localhost:8890

### 认证方式
1. 首先调用 `POST /api/auth/sessions` 登录获取 token
2. 在后续请求中使用 `Authorization: Bearer {{token}}`

## 📊 服务架构

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   前端 UI   │───▶│  API 服务   │───▶│  游戏服务   │
│             │    │  (8888)     │    │             │
└─────────────┘    └─────────────┘    └─────────────┘
                          │                   │
                          ▼                   ▼
                   ┌─────────────┐    ┌─────────────┐
                   │ Agent 服务  │    │ Edge 服务   │
                   │  (8889)     │    │  (8890)     │
                   └─────────────┘    └─────────────┘
```

## ✅ 快速验证

导入配置后，按以下步骤验证：

1. **选择环境**: 选择"开发环境"
2. **登录认证**: 调用登录接口获取 token
3. **测试 API**: 使用 token 测试其他接口
4. **检查响应**: 确认返回数据格式正确

## 🛠️ 常见问题

### Q: 提示 401 未授权？
A: 确保先调用登录接口获取 token，并在请求头中携带 `Authorization: Bearer {{token}}`

### Q: 提示连接失败？
A: 确认本地服务已启动，检查端口是否正确

### Q: 导入失败？
A: 检查 JSON 文件格式是否正确，建议使用文本编辑器验证

---

**提示**: 生成这些配置的命令：
```bash
./generate-api-docs.sh
```
EOF

echo "✅ 使用说明已生成: API_IMPORT_GUIDE.md"

# 清理临时文件
cd "$API_DIR"
rm -f simple.api

echo ""
echo "🎉 API 文档生成完成！"
echo ""
echo "📁 生成的文件:"
echo "  - croupier-api.json (OpenAPI 规范)"
echo "  - croupier-api-environments.json (环境配置)"
echo "  - croupier-apifox-collection.json (APIfox 集合)"
echo "  - API_IMPORT_GUIDE.md (使用指南)"
echo ""
echo "📖 查看使用指南: API_IMPORT_GUIDE.md"
echo ""
echo "💡 下一步:"
echo "  1. 将 croupier-api.json 导入到 Swagger UI"
echo "  2. 将环境配置导入到 Postman"
echo "  3. 将集合导入到 APIfox"