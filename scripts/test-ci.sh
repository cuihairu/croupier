#!/bin/bash

# 触发GitHub Actions工作流来测试修复
# ⚠️  警告: 此脚本会创建提交并推送到远程仓库
# 建议仅在测试分支上使用，不要在 main/master 分支上执行

echo "Creating a test commit to trigger GitHub Actions..."
echo ""
echo "⚠️  警告: 此脚本会执行以下操作:"
echo "  1. 创建测试提交"
echo "  2. 推送到远程仓库"
echo "  3. 使用 force-push 清理测试提交"
echo ""
read -p "确认继续? (y/N): " confirm
echo "$confirm" | grep -qE "^[yY]" || { echo "操作已取消"; exit 0; }

# 创建一个简单的测试文件
echo "test CI pipeline trigger" > .ci-test-trigger

# 添加并提交
git add .ci-test-trigger
git commit -m "test: trigger CI pipeline to verify tar extraction fixes"

# 推送
git push

# 清理
rm .ci-test-trigger
git add .ci-test-trigger
git commit -m "clean: remove CI test trigger file" --amend --no-edit
git push --force-with-lease

echo "CI pipeline trigger complete. Check GitHub Actions for results."