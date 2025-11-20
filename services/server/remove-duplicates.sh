#!/bin/bash

# 删除 server.api 文件中的重复类型定义

echo "🔧 开始修复 server.api 中的重复类型定义..."

# 要删除的重复类型定义（保留第一次出现，删除后续的）
declare -A duplicates=(
    ["OpsHealthResponse"]="771"
    ["OpsHealthUpdateRequest"]="776"
    ["OpsHealthRunRequest"]="780"
    ["BackupEntry"]="1323"
    ["OpsBackupsResponse"]="1334"
    ["OpsBackupCreateRequest"]="1338"
    ["OpsBackupCreateResponse"]="1343"
    ["ProviderInfo"]="946"
)

# 按行号从大到小排序，避免删除行号偏移的问题
for type_name in "${!duplicates[@]}"; do
    line_num="${duplicates[$type_name]}"
    echo "删除重复类型 $type_name (第 $line_num 行)"

    # 找到该类型定义的结束行（下一个 type 定义或文件末尾）
    end_line=$(sed -n "$((line_num + 1)),/^type /{ /^type / {=; q; } }" server.api | tail -1)
    if [ -z "$end_line" ]; then
        # 如果没找到下一个 type 定义，就到文件末尾
        end_line=$(wc -l < server.api)
    fi

    echo "  删除第 $line_num 到 $((end_line - 1)) 行"
    sed -i '' "$line_num,$((end_line - 1))d" server.api
done

echo "✅ 重复类型定义修复完成"

# 验证是否还有重复
echo ""
echo "🔍 验证修复结果..."
duplicates_left=$(grep -n "^type " server.api | awk '{print $2}' | sort | uniq -d | wc -l)

if [ "$duplicates_left" -eq 0 ]; then
    echo "✅ 没有发现重复的类型定义"
else
    echo "⚠️  仍有重复的类型定义："
    grep -n "^type " server.api | awk '{print $2}' | sort | uniq -d
fi

echo ""
echo "📊 类型定义统计："
echo "总类型数: $(grep -c "^type " server.api)"
echo "总行数: $(wc -l < server.api)"