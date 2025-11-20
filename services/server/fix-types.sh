#!/bin/bash

# 修复 go-zero API 文件中不支持的类型

sed -i '' 's/time\.Time/string/g' server.api
echo "已将 time.Time 替换为 string"