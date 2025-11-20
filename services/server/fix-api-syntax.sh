#!/bin/bash

# 这个脚本用于修复 go-zero API 文件中的语法错误
# 只修改顶级的 type 定义，保留嵌套结构体中的 struct 关键字

# 读取文件并逐行处理
input="server.api"
output="server_fixed.api"

# 使用 awk 来精确处理
awk '
/^type [a-zA-Z][a-zA-Z0-9_]* struct {$/ {
    # 匹配顶级类型定义: type TypeName struct {
    gsub(/struct {/, "{")
    print
    next
}
{
    print
}
' "$input" > "$output"

# 替换原文件
mv "$output" "$input"

echo "API 语法修复完成"