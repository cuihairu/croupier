# Phase 6 — 依赖清理

状态: ⬜ 待开始

## 目标

移除所有 go-zero 依赖，替换为标准库或 Gin 生态工具。

## 检查清单

- [ ] 6.1 替换 `logx.Logger` → `log/slog` 或 `github.com/sirupsen/logrus`
- [ ] 6.2 移除 `go.mod` 中的 `github.com/zeromicro/go-zero` 依赖
- [ ] 6.3 删除 `server.api` 文件（不再需要 goctl）
- [ ] 6.4 删除 `.goctl` 配置文件（如存在）
- [ ] 6.5 运行 `go mod tidy` 清理未使用的依赖
- [ ] 6.6 全局搜索 `zeromicro` 确认无残留引用
- [ ] 6.7 运行 `go build ./...` 确认编译通过
- [ ] 6.8 运行完整测试套件

---

## 6.1 logx.Logger 替换

### Before (go-zero logx)
```go
import "github.com/zeromicro/go-zero/core/logx"

type XxxLogic struct {
    logx.Logger
    ctx    context.Context
    svcCtx *svc.ServiceContext
}

func (l *XxxLogic) Xxx(req *types.XxxRequest) (*types.XxxResponse, error) {
    l.Infof("Processing request: %+v", req)
    // ...
}
```

### After (标准库 log/slog)
```go
import "log/slog"

type XxxLogic struct {
    logger *slog.Logger
    ctx    context.Context
    svcCtx *svc.ServiceContext
}

func NewXxxLogic(ctx context.Context, svcCtx *svc.ServiceContext) *XxxLogic {
    return &XxxLogic{
        logger: slog.Default(),
        ctx:    ctx,
        svcCtx: svcCtx,
    }
}

func (l *XxxLogic) Xxx(req *types.XxxRequest) (*types.XxxResponse, error) {
    l.logger.InfoContext(l.ctx, "Processing request", "req", req)
    // ...
}
```

### 批量替换命令

```bash
# 1. 替换 import
find internal/logic -name "*.go" -exec sed -i 's|"github.com/zeromicro/go-zero/core/logx"|"log/slog"|g' {} \;

# 2. 替换 logx.Logger → logger *slog.Logger
find internal/logic -name "*.go" -exec sed -i 's/logx\.Logger/logger *slog.Logger/g' {} \;

# 3. 替换 logx.WithContext → slog.Default()
find internal/logic -name "*.go" -exec sed -i 's/logx\.WithContext(ctx)/slog.Default()/g' {} \;
```

> 注意：`logx` 的 `Infof/Errorf` 需要手动改为 `slog` 的 `InfoContext/ErrorContext`。

---

## 6.2 移除 go-zero 依赖

```bash
# 编辑 go.mod，删除以下行：
# require github.com/zeromicro/go-zero v1.x.x

# 或使用命令
go mod edit -droprequire github.com/zeromicro/go-zero
go mod tidy
```

---

## 6.3 删除 API 定义文件

```bash
# server.api 不再需要（不再使用 goctl 生成代码）
rm server.api

# 如果有其他 .api 文件
find . -name "*.api" -delete
```

---

## 6.4 删除 goctl 配置

```bash
# 删除 goctl 配置文件（如存在）
rm -f .goctl
rm -rf .goctl/
```

---

## 6.5 验证清理结果

```bash
# 1. 检查是否还有 go-zero 引用
grep -r "zeromicro" . --include="*.go" | grep -v vendor | grep -v ".git"

# 2. 检查 go.mod
grep "zeromicro" go.mod

# 3. 编译测试
go build ./...

# 4. 运行测试
go test ./...
```

---

## 6.6 更新 README 和文档

- [ ] 更新 README.md 中的依赖说明（移除 go-zero，添加 Gin）
- [ ] 更新开发文档中的代码生成说明（不再使用 goctl）
- [ ] 更新 CI/CD 配置（如有 goctl 相关步骤）

---

## 注意事项

### common/cli 包依赖

如果 `internal/cli/common` 包依赖 go-zero 的日志或配置工具，需要同步替换：

- `logx` → `slog`
- `conf.MustLoad` → `viper` 或 `yaml.Unmarshal`

### 第三方包依赖

检查是否有其他内部包（如 `github.com/cuihairu/croupier/internal/cli/common`）依赖 go-zero，需要同步更新。

### 测试文件

如果测试文件中使用了 `httptest` 配合 go-zero 的 `rest.Server`，需要改为 Gin 的测试方式：

```go
// Before
server := rest.MustNewServer(rest.RestConf{})
handler.RegisterHandlers(server, ctx)

// After
r := gin.New()
handler.RegisterHandlers(r, ctx)
w := httptest.NewRecorder()
req, _ := http.NewRequest("GET", "/api/v1/xxx", nil)
r.ServeHTTP(w, req)
```
