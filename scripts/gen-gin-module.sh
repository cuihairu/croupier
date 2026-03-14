#!/bin/bash
# Gin 模块代码生成脚本
# 用法: ./scripts/gen-gin-module.sh <module_name>

set -e

MODULE_NAME=$1

if [ -z "$MODULE_NAME" ]; then
    echo "Usage: $0 <module_name>"
    echo "Example: $0 admin"
    exit 1
fi

BASE_DIR="services/server/internal/api/$MODULE_NAME"

echo "🚀 Creating Gin module: $MODULE_NAME"

# 创建目录
mkdir -p "$BASE_DIR"

# 生成 DTO 文件
cat > "$BASE_DIR/dto.go" << 'EOF'
package MODULE_NAME

// TODO: 定义请求和响应结构体
// 示例:
// type CreateRequest struct {
//     Name string `json:"name" binding:"required"`
// }
//
// type CreateResponse struct {
//     ID   uint   `json:"id"`
//     Name string `json:"name"`
// }
EOF

# 生成 Service 文件
cat > "$BASE_DIR/service.go" << 'EOF'
package MODULE_NAME

import (
    "context"
    "github.com/cuihairu/croupier/services/server/internal/model"
    "gorm.io/gorm"
)

type Service struct {
    db *gorm.DB
    // TODO: 添加需要的 model 依赖
}

func NewService(db *gorm.DB) *Service {
    return &Service{
        db: db,
    }
}

// TODO: 实现业务逻辑方法
// 示例:
// func (s *Service) Create(ctx context.Context, req *CreateRequest) (*CreateResponse, error) {
//     // 业务逻辑
//     return &CreateResponse{}, nil
// }
EOF

# 生成 Handler 文件
cat > "$BASE_DIR/handler.go" << 'EOF'
package MODULE_NAME

import (
    "github.com/gin-gonic/gin"
    "github.com/cuihairu/croupier/services/server/internal/pkg/response"
)

type Handler struct {
    service *Service
}

func NewHandler(service *Service) *Handler {
    return &Handler{
        service: service,
    }
}

// TODO: 实现 HTTP 处理器方法
// 示例:
// func (h *Handler) Create(c *gin.Context) {
//     var req CreateRequest
//     if err := c.ShouldBindJSON(&req); err != nil {
//         response.Error(c, 400, "参数错误: "+err.Error())
//         return
//     }
//
//     resp, err := h.service.Create(c.Request.Context(), &req)
//     if err != nil {
//         response.Error(c, 500, err.Error())
//         return
//     }
//
//     response.Success(c, resp)
// }
EOF

# 生成路由注册文件
cat > "$BASE_DIR/routes.go" << 'EOF'
package MODULE_NAME

import (
    "github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, handler *Handler) {
    group := r.Group("/MODULE_NAME")
    {
        // TODO: 注册路由
        // 示例:
        // group.POST("", handler.Create)
        // group.GET("/:id", handler.Get)
        // group.PUT("/:id", handler.Update)
        // group.DELETE("/:id", handler.Delete)
        // group.GET("", handler.List)
    }
}
EOF

# 替换模块名
sed -i "s/MODULE_NAME/$MODULE_NAME/g" "$BASE_DIR"/*.go

echo "✅ Module $MODULE_NAME created successfully!"
echo ""
echo "📁 Files created:"
echo "  - $BASE_DIR/dto.go"
echo "  - $BASE_DIR/service.go"
echo "  - $BASE_DIR/handler.go"
echo "  - $BASE_DIR/routes.go"
echo ""
echo "📝 Next steps:"
echo "  1. 编辑 dto.go 定义请求/响应结构"
echo "  2. 编辑 service.go 实现业务逻辑"
echo "  3. 编辑 handler.go 实现 HTTP 处理器"
echo "  4. 编辑 routes.go 注册路由"
echo "  5. 在 internal/router/router.go 中注册模块路由"
