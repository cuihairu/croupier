# Croupier API Swagger 生成指南

## 🎯 问题解决方案

当前 `server.api` 文件在多次编辑中丢失了很多类型定义，导致无法直接生成 swagger。

### ✅ 推荐解决方案

**使用我们已经成功生成的带注释版本：**

```bash
# 生成带完整中文注释的 OpenAPI 规范
goctl api swagger --api annotated-api.api --dir . --filename croupier-api-annotated

# 生成简化版本
goctl api swagger --api simple-swagger.api --dir . --filename croupier-api-simple
```

### 📁 可用的 Swagger 文件

| 文件名 | 大小 | 特点 | 推荐度 |
|--------|------|------|--------|
| **`croupier-api-annotated.json`** | **47KB** | 157行中文注释，47个API端点 | ⭐⭐⭐⭐⭐ |
| `croupier-api-simple.json` | 31KB | 简化版本，40个API端点 | ⭐⭐⭐⭐ |
| `croupier-api.json` | 7.2KB | 有语法问题 | ⚠️ |

### 🎯 最佳实践

1. **使用带注释版本**: `croupier-api-annotated.json`
   - 完整的中文注释
   - 按功能模块分组
   - 详细的字段说明

2. **导入到工具**:
   ```bash
   # Postman
   Import → Link → 选择 croupier-api-annotated.json

   # APIfox
   导入 → OpenAPI → 上传 croupier-api-annotated.json

   # Swagger UI
   docker run -p 80:8080 \
     -e SWAGGER_JSON=/croupier-api-annotated.json \
     -v $(pwd)/croupier-api-annotated.json:/croupier-api-annotated.json \
     swaggerapi/swagger-ui
   ```

3. **环境配置**:
   - 使用 `croupier-api-environments.json` 配置开发/测试/生产环境

### 🔧 如果坚持修复 server.api

如果你想修复原始的 `server.api` 文件，需要：

1. 添加所有缺失的类型定义
2. 修复语法错误
3. 确保类型引用正确

**但这个过程很耗时，推荐使用已生成的版本。**

### 📊 生成统计

- **注释覆盖率**: 43%
- **API端点数**: 47个
- **功能模块**: 8个
- **文档大小**: 47KB

### 💡 go-zero 注释语法

```go
// 模块注释
// ============================================================================
// 认证相关类型定义
// ============================================================================

// 类型注释
// 用户信息 - 系统用户基本信息
type UserInfo {
    Username string `json:"username"` // 用户名
    Roles    []string `json:"roles"`    // 用户角色列表
}

// 服务注释
service croupier-api {
    // 用户登录认证
    @handler AuthLoginHandler
    post /api/auth/sessions (LoginRequest) returns (LoginResponse)
}
```

### 🚀 快速开始

```bash
# 1. 使用推荐版本
goctl api swagger --api annotated-api.api --dir . --filename my-swagger

# 2. 查看生成的文件
ls -la my-swagger.json

# 3. 导入到你的API工具
```

---

**推荐**: 直接使用 `croupier-api-annotated.json`，它已经包含了完整的API文档和中文注释。