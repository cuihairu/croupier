# 🚀 快速开始：代码风格迁移

将所有服务从 `gozero` 迁移到 `go_zero` 风格，只需 3 步！

## 📋 前置准备（必须！）

```bash
cd /Users/cui/Workspaces/croupier

# 1. 提交当前所有更改（重要！）
git add .
git commit -m "chore: save state before migration"

# 2. 或者创建分支
git checkout -b feat/migrate-go-zero-style
```

## 🎯 三步迁移

### 步骤 1: 查看计划

```bash
cd /Users/cui/Workspaces/croupier/services

# 预览迁移计划（不执行）
./migrate-to-go-zero.sh --dry-run
```

### 步骤 2: 执行迁移

```bash
# 交互式确认（推荐）
./migrate-to-go-zero.sh

# 或自动确认
./migrate-to-go-zero.sh --yes
```

脚本会自动：
- ✅ 备份所有代码
- ✅ 重新生成代码（go_zero 风格）
- ✅ 生成迁移报告

### 步骤 3: 验证结果

```bash
# 运行验证脚本
./verify-migration.sh
```

## ✅ 验证检查项

- [x] 配置文件正确
- [x] 文件使用下划线命名
- [x] 编译成功
- [x] 测试通过
- [x] 目录结构完整
- [x] 备份已创建

## 📊 命名变化示例

**迁移前**:
```
❌ adminfunctionpermissionsgethandler.go  (难读)
❌ userprofilehandler.go                  (勉强)
```

**迁移后**:
```
✅ admin_function_permissions_get_handler.go  (清晰)
✅ user_profile_handler.go                    (易读)
```

## ⚠️ 重要提醒

### 业务逻辑需要手动合并

Logic 文件会被重新生成，使用 git diff 查看变化：

```bash
cd /Users/cui/Workspaces/croupier

# 查看所有变化
git diff services/

# 查看 logic 文件变化
git diff services/server/internal/logic/
```

### 使用 VS Code 合并

```bash
# 对比单个文件
code --diff backup_*/server_internal/logic/xxx_logic.go \
              services/server/internal/logic/xxx_logic.go
```

## 🔄 如果出现问题

### 快速回滚

```bash
# 方法 1: 使用 Git（推荐）
git reset --hard HEAD

# 方法 2: 从备份恢复
cd /Users/cui/Workspaces/croupier/services
BACKUP_DIR=$(ls -d backup_* | tail -1)
cp -r ${BACKUP_DIR}/server_internal server/internal
```

## 📚 详细文档

- **完整迁移指南**: [MIGRATION-GUIDE.md](MIGRATION-GUIDE.md)
- **配置说明**: [README-CONFIG.md](README-CONFIG.md)
- **常见问题**: 见 MIGRATION-GUIDE.md

## 🆘 需要帮助？

1. 查看迁移报告: `cat migration_report_*.md`
2. 运行验证脚本: `./verify-migration.sh`
3. 阅读完整文档: `MIGRATION-GUIDE.md`

---

**提示**: 建议先在一个分支上测试迁移，验证无误后再合并到主分支。
