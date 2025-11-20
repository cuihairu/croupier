#!/bin/bash

# 修复 server.api 文件中的 any 类型问题

echo "🔧 修复 server.api 中的 any 类型..."

# 将 [][]any 替换为 [][]interface{} (这在 go-zero 中也有问题)
# 更好的方法是替换为 [][]string 或 [][]int64

echo "替换 any 类型为 interface{}..."
sed -i '' 's/\[\]\[\]any/\[\]\[\]interface{}/g' server.api

echo "✅ any 类型修复完成"

echo ""
echo "🔍 验证修复结果..."
export PATH=$PATH:~/go/bin

# 尝试验证
if goctl api validate --api server.api > /dev/null 2>&1; then
    echo "✅ API 文件验证通过"
else
    echo "⚠️ API 文件验证仍有问题，尝试进一步修复..."
    # 如果 interface{} 也有问题，我们可以使用更简单的类型
    sed -i '' 's/\[\]\[\]interface{}/\[\]\[\]string/g' server.api
    echo "已将 interface{} 替换为 string"
fi

echo ""
echo "📊 文件信息:"
echo "总行数: $(wc -l < server.api)"
echo "文件大小: $(du -h server.api | cut -f1)"