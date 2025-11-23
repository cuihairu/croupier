#!/bin/bash

# 触发GitHub Actions工作流来测试修复
echo "Creating a test commit to trigger GitHub Actions..."

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