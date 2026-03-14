# Phase 0 — 清理与依赖准备

状态: ⬜ 待开始

## 目标

移除临时文件，添加 Gin/Viper 依赖，确保项目可以编译。

## 检查清单

- [ ] 删除 `internal/api/` 目录（goctl 生成的临时 API 文件）
- [ ] 删除 `gin_config.go`（如存在）
- [ ] 添加 Gin 依赖
- [ ] 添加 Viper 依赖
- [ ] 确认 `go.mod` 更新正确
- [ ] 运行 `go build ./...` 确认无编译错误（此时会有，记录错误数量）

## 操作命令

```bash
# 切换到正确分支
git checkout feature/migrate-gin

# 添加依赖
go get github.com/gin-gonic/gin@latest
go get github.com/spf13/viper@latest
go get github.com/gin-contrib/cors@latest

# 整理依赖
go mod tidy
```

## 注意

- `go-zero` 依赖在 Phase 6 才移除，Phase 0 只是添加新依赖
- 如果存在 `internal/api/` 目录，确认内容是否为 goctl 生成（无业务逻辑）再删除
