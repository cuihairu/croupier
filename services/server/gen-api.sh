#!/bin/bash

# API 代码生成脚本 - 使用 go_zero 命名风格
# 符合 Go 官方规范：使用下划线分隔的文件名

set -e

echo "🚀 开始生成 API 代码（go_zero 风格）..."

# 确保 goctl 在 PATH 中
export PATH=$PATH:$HOME/go/bin

# 检查 goctl 是否存在
if ! command -v goctl &> /dev/null; then
    echo "❌ 错误: goctl 未安装"
    echo "💡 请运行: go install github.com/zeromicro/go-zero/tools/goctl@latest"
    exit 1
fi

# 检查 API 文件是否存在
if [ ! -f "server.api" ]; then
    echo "❌ 错误: server.api 文件不存在"
    exit 1
fi

# 验证 API 文件
echo "🔍 验证 API 文件..."
if ! goctl api validate --api server.api; then
    echo "❌ API 文件验证失败"
    exit 1
fi

# 备份现有代码
echo "📦 备份现有代码..."
timestamp=$(date +%Y%m%d_%H%M%S)
if [ -d "internal" ]; then
    cp -r internal "internal.backup.$timestamp"
    echo "✅ 备份完成: internal.backup.$timestamp"
fi

# 生成代码（使用 go_zero 风格）
echo "⚙️  生成代码（go_zero 命名风格）..."
goctl api go \
    -api server.api \
    -dir . \
    --style=go_zero

if [ $? -eq 0 ]; then
    echo "✅ 代码生成成功！"
    echo ""
    echo "📁 生成的文件使用下划线命名风格，例如："
    echo "   - admin_user_handler.go"
    echo "   - user_profile_logic.go"
    echo ""
    echo "⚠️  注意："
    echo "   - types.go 文件会被覆盖"
    echo "   - 请手动合并你的业务逻辑代码"
    echo "   - 备份位置: internal.backup.$timestamp"
else
    echo "❌ 代码生成失败"
    exit 1
fi
