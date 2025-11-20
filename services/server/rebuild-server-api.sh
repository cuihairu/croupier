#!/bin/bash

# 重建完整可用的 server.api 文件

set -e

echo "🔧 重建完整的 server.api 文件..."

export PATH=$PATH:$HOME/go/bin

# 1. 备份原始文件
if [ -f "server.api" ]; then
    cp server.api server.api.backup.$(date +%Y%m%d_%H%M%S)
    echo "✅ 已备份原始文件"
fi

# 2. 基于带注释版本重建 server.api
echo "📝 基于 annotated-api.api 重建 server.api..."

# 复制带注释的版本作为基础
cp annotated-api.api temp-server.api

# 3. 检查原始 server.api 中独有的API端点
echo "🔍 检查原始 server.api 中独有的API端点..."

if [ -f "server.api" ]; then
    # 提取原始文件中的 handler 定义
    grep "@handler" server.api | sort > original_handlers.txt
    grep "@handler" temp-server.api | sort > current_handlers.txt

    # 找出原始文件中独有的handler
    comm -13 current_handlers.txt original_handlers.txt > missing_handlers.txt

    if [ -s missing_handlers.txt ]; then
        echo "📋 发现缺失的API端点:"
        cat missing_handlers.txt
        echo ""
        echo "⚠️  原始文件中有 $(wc -l < missing_handlers.txt) 个API端点未包含在当前版本中"
        echo "💡 如果需要这些端点，请手动添加到 annotated-api.api"
    else
        echo "✅ 所有API端点都已包含在带注释版本中"
    fi

    # 清理临时文件
    rm -f original_handlers.txt current_handlers.txt missing_handlers.txt
fi

# 4. 验证新的 server.api
echo "🔍 验证重建的 API 文件..."
if goctl api validate --api temp-server.api > /dev/null 2>&1; then
    echo "✅ API 文件验证通过"

    # 替换原始文件
    mv temp-server.api server.api
    echo "✅ 已更新 server.api"

    # 生成新的 swagger 文档
    echo "📄 生成最新的 swagger 文档..."
    goctl api swagger --api server.api --dir . --filename croupier-api-latest

    if [ $? -eq 0 ]; then
        echo "✅ Swagger 文档生成成功: croupier-api-latest.json"
    else
        echo "⚠️  Swagger 文档生成失败"
    fi

else
    echo "❌ API 文件验证失败"
    rm -f temp-server.api
    exit 1
fi

echo ""
echo "🎉 server.api 重建完成！"
echo ""
echo "📁 生成的文件:"
if [ -f "croupier-api-latest.json" ]; then
    echo "  - croupier-api-latest.json ($(du -h croupier-api-latest.json | cut -f1))"
fi
echo "  - server.api (更新后的API定义文件)"
echo ""
echo "💡 后续添加API的方法:"
echo "  1. 直接编辑 server.api 文件"
echo "  2. 添加类型定义: type TypeName { ... }"
echo "  3. 添加API路由: @handler HandlerName"
echo "  4. 验证: goctl api validate --api server.api"
echo "  5. 生成文档: goctl api swagger --api server.api"