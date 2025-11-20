#!/bin/bash

# 查找 server.api 中所有缺失的类型定义

echo "🔍 查找 server.api 中缺失的类型定义..."

# 创建临时文件来存储所有使用的类型
grep -o '(Req\|Response)\)' server.api | sed 's/[()]//g' | sort | uniq > used_types.txt

# 创建临时文件来存储所有定义的类型
grep -E '^type [A-Za-z][A-Za-z0-9_]*' server.api | awk '{print $2}' | sort | uniq > defined_types.txt

echo ""
echo "📋 使用但未定义的类型:"
while IFS= read -r type; do
    if ! grep -q "^$type$" defined_types.txt; then
        echo "  - $type"
    fi
done < used_types.txt

# 清理临时文件
rm -f used_types.txt defined_types.txt

echo ""
echo "💡 建议使用已生成的带注释版本:"
echo "  goctl api swagger --api annotated-api.api --dir . --filename croupier-api"