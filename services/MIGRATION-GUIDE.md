# 代码风格迁移指南

将所有服务从 `gozero` 风格迁移到 `go_zero` 风格（下划线分隔）。

## 🎯 迁移效果

### 文件命名变化

**旧风格 (gozero)**:
```
adminfunctionpermissionsgethandler.go     ❌ 难读
userprofilehandler.go                     ⚠️ 勉强
authloginhandler.go                       ⚠️ 勉强
```

**新风格 (go_zero)**:
```
admin_function_permissions_get_handler.go ✅ 清晰易读
user_profile_handler.go                   ✅ 符合规范
auth_login_handler.go                     ✅ 一目了然
```

## 📋 迁移步骤

### 步骤 1: 先查看计划（推荐）

```bash
cd /Users/cui/Workspaces/croupier/services

# 仅查看迁移计划，不执行
./migrate-to-go-zero.sh --dry-run
```

这会显示：
- 将要迁移的服务列表
- 文件命名变化示例
- 备份位置
- 注意事项

### 步骤 2: 提交现有代码（重要！）

```bash
cd /Users/cui/Workspaces/croupier

# 查看当前状态
git status

# 提交所有未提交的更改
git add .
git commit -m "chore: save current state before migration"

# 或者创建一个分支
git checkout -b feat/migrate-go-zero-style
```

### 步骤 3: 执行迁移

```bash
cd /Users/cui/Workspaces/croupier/services

# 方式 1: 交互式确认（推荐）
./migrate-to-go-zero.sh

# 方式 2: 自动确认
./migrate-to-go-zero.sh --yes
```

脚本会：
1. ✅ 自动备份所有服务代码
2. ✅ 重新生成所有代码（使用 go_zero 风格）
3. ✅ 生成详细的迁移报告

### 步骤 4: 检查生成结果

```bash
# 查看新的文件命名
ls server/internal/handler/ | head -10
ls agent/internal/handler/ | head -10

# 应该看到下划线分隔的文件名
```

### 步骤 5: 合并业务逻辑

**重要**: Logic 文件会被重新生成，需要手动合并业务逻辑。

#### 方法 1: 使用 git diff（推荐）

```bash
cd /Users/cui/Workspaces/croupier

# 查看所有变化
git diff services/

# 查看特定服务的 logic 变化
git diff services/server/internal/logic/

# 使用 VS Code 查看差异
code --diff backup_*/server_internal/logic/user_logic.go \
              services/server/internal/logic/user_logic.go
```

#### 方法 2: 手动对比

```bash
# 使用备份目录对比
cd /Users/cui/Workspaces/croupier/services

# 找到备份目录
ls -d backup_*

# 对比文件
diff -u backup_*/server_internal/logic/xxx_logic.go \
        server/internal/logic/xxx_logic.go
```

#### 方法 3: 使用可视化工具

**推荐工具**:
- **VS Code**: 内置 diff 功能
- **Meld**: 开源对比工具 (`brew install meld`)
- **Beyond Compare**: 专业对比工具

```bash
# 使用 Meld 对比整个目录
meld backup_*/server_internal/logic server/internal/logic
```

### 步骤 6: 编译验证

```bash
cd /Users/cui/Workspaces/croupier/services

# 编译所有服务
for service in server agent edge; do
  echo "编译 $service..."
  (cd $service && go build) || echo "编译失败: $service"
done
```

### 步骤 7: 运行测试

```bash
cd /Users/cui/Workspaces/croupier/services

# 运行所有测试
go test ./...

# 针对特定服务
cd server && go test ./...
```

### 步骤 8: 提交迁移

```bash
cd /Users/cui/Workspaces/croupier

# 查看所有变化
git status
git diff --stat

# 提交迁移
git add .
git commit -m "refactor: migrate to go_zero naming style

- 将所有服务从 gozero 迁移到 go_zero 风格
- 文件命名使用下划线分隔，符合 Go 官方规范
- 已验证编译和测试通过
"
```

## 🔍 验证清单

迁移完成后，检查以下项目：

### ✅ 文件命名

```bash
# 检查 handler 文件
find services/*/internal/handler -name "*.go" | head -20

# 应该看到下划线风格
# ✅ user_profile_handler.go
# ❌ userprofilehandler.go
```

### ✅ 编译成功

```bash
# 所有服务都能编译
cd services/server && go build
cd services/agent && go build
```

### ✅ 测试通过

```bash
# 测试通过
cd services && go test ./...
```

### ✅ 业务逻辑完整

```bash
# 对比业务逻辑是否完整
git diff services/server/internal/logic/
```

## 🚨 常见问题

### Q1: 编译错误 "undefined: XXX"

**原因**: types.go 重新生成，可能类型定义变化

**解决**:
1. 检查 API 定义是否完整
2. 对比新旧 types.go 差异
3. 更新引用位置

```bash
# 查看 types.go 变化
git diff services/server/internal/types/types.go
```

### Q2: Logic 文件的业务逻辑丢失

**原因**: Logic 文件被重新生成，只有空模板

**解决**:
```bash
# 从备份恢复
cd /Users/cui/Workspaces/croupier/services

# 找到备份目录
BACKUP_DIR=$(ls -d backup_* | tail -1)

# 复制业务逻辑
cp ${BACKUP_DIR}/server_internal/logic/xxx_logic.go \
   server/internal/logic/xxx_logic.go
```

### Q3: Handler 文件有自定义修改

**原因**: Handler 文件被完全覆盖

**解决**:
```bash
# 从备份查看自定义修改
diff ${BACKUP_DIR}/server_internal/handler/xxx_handler.go \
     server/internal/handler/xxx_handler.go

# 手动应用自定义修改
```

**建议**: Handler 应该是薄层，业务逻辑应该在 Logic 层

### Q4: 测试失败

**解决步骤**:
1. 检查导入路径是否正确
2. 验证 types 定义是否一致
3. 确认业务逻辑已完整迁移
4. 检查测试文件本身是否需要更新

## 🔄 回滚方法

### 方法 1: 从备份恢复（快速）

```bash
cd /Users/cui/Workspaces/croupier/services

# 查看备份目录
ls -d backup_*

# 恢复特定服务
BACKUP_DIR=$(ls -d backup_* | tail -1)
cp -r ${BACKUP_DIR}/server_internal server/internal

# 或恢复所有服务
for service in server agent edge; do
  if [[ -d ${BACKUP_DIR}/${service}_internal ]]; then
    cp -r ${BACKUP_DIR}/${service}_internal ${service}/internal
  fi
done
```

### 方法 2: 使用 Git 回滚（推荐）

```bash
cd /Users/cui/Workspaces/croupier

# 回滚所有更改
git reset --hard HEAD

# 或回滚特定文件
git checkout HEAD -- services/server/internal/
```

## 📊 迁移报告

脚本会自动生成详细报告：

```bash
# 查看报告
cat services/migration_report_*.md

# 报告包含：
# - 备份位置
# - 迁移的服务列表
# - 文件统计
# - 后续步骤
# - 常见问题
```

## 💡 最佳实践

### ✅ 推荐做法

1. **分批迁移**: 先迁移一个服务，验证后再迁移其他
2. **保持 Git 清洁**: 迁移前提交所有更改
3. **详细测试**: 编译、单元测试、集成测试
4. **代码评审**: 团队成员检查迁移结果

### ⚠️ 注意事项

1. **业务逻辑**: 务必检查 logic 文件的业务代码
2. **自定义修改**: Handler 和 Middleware 的自定义部分
3. **测试覆盖**: 确保测试通过
4. **团队同步**: 通知团队成员代码结构变化

## 🎯 分步迁移（推荐）

如果一次性迁移所有服务风险较大，可以分步进行：

### 步骤 1: 先迁移一个服务（如 demo）

```bash
cd /Users/cui/Workspaces/croupier/services/demo

# 备份
cp -r internal internal.backup

# 生成
goctl api go -api demo.api -dir . --style go_zero

# 验证
go build
go test ./...

# 如果有问题，回滚
# cp -r internal.backup internal
```

### 步骤 2: 验证成功后，迁移其他服务

```bash
cd /Users/cui/Workspaces/croupier/services

# 使用脚本批量迁移
./migrate-to-go-zero.sh
```

## 📚 相关文档

- 配置说明: [README-CONFIG.md](README-CONFIG.md)
- 代码生成: [README.md](README.md)
- Go 命名规范: https://go.dev/doc/effective_go#names

## 🆘 获取帮助

如果迁移遇到问题：

1. 查看迁移报告: `migration_report_*.md`
2. 检查备份目录: `backup_*`
3. 使用 `git diff` 查看具体变化
4. 参考本文档的常见问题部分
