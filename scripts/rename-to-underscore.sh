#!/bin/bash

# 批量将驼峰命名的文件改为下划线风格

# 找到需要重命名的文件
find services/server/internal -name "*handler.go" -o -name "*logic.go" | while read file; do
    # 获取目录和文件名
    dir=$(dirname "$file")
    filename=$(basename "$file")

    # 将驼峰命名转换为下划线命名
    # authloginhandler.go -> auth_login_handler.go
    # roledetaillogic.go -> role_detail_logic.go
    newname=$(echo "$filename" | sed -E 's/([a-z])([A-Z])/\1_\2/g' | tr '[:upper:]' '[:lower:]')

    # 如果文件名有变化，则重命名
    if [ "$filename" != "$newname" ]; then
        echo "Renaming: $file -> $dir/$newname"
        mv "$file" "$dir/$newname"
    fi
done

echo "Done renaming files to underscore style!"