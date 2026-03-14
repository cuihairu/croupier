# Phase 2 — 启动入口重构

状态: ⬜ 待开始

## 目标

将 `cmd/root.go` 从 `rest.MustNewServer` 改为 Gin 引擎启动。

## 检查清单

- [ ] 2.1 替换配置加载：`conf.MustLoad` → `viper` 或直接 `yaml.Unmarshal`
- [ ] 2.2 替换服务器创建：`rest.MustNewServer` → `gin.New()`
- [ ] 2.3 注册全局中间件（Logger、CORS、Recovery）
- [ ] 2.4 注册认证中间件
- [ ] 2.5 调用 `handler.RegisterHandlers(r, ctx)`
- [ ] 2.6 替换 `server.Start()` → `r.Run(addr)`
- [ ] 2.7 移除 `c.RestConf.Port`、`c.RestConf.Host` 等引用，改用 `c.Server.Port`
- [ ] 2.8 移除 `applyRuntimeDefaults` 中对 `c.RestConf.Timeout` 的引用

## Before → After 对比

### Before
```go
import (
    "github.com/zeromicro/go-zero/core/conf"
    "github.com/zeromicro/go-zero/rest"
)

func runServer() error {
    var c config.Config
    conf.MustLoad(cfgFile, &c, conf.UseEnv())

    if port > 0 {
        c.RestConf.Port = port
    }

    server := rest.MustNewServer(c.RestConf)
    defer server.Stop()

    authMiddleware := svc.NewAuthMiddleware(ctx)
    server.Use(authMiddleware)

    handler.RegisterHandlers(server, ctx)
    server.Start()
    return nil
}
```

### After
```go
import (
    "fmt"
    "github.com/gin-gonic/gin"
    "github.com/gin-contrib/cors"
    "gopkg.in/yaml.v3"
    "os"
)

func runServer() error {
    data, err := os.ReadFile(cfgFile)
    if err != nil {
        return fmt.Errorf("读取配置文件失败: %w", err)
    }
    // 展开环境变量
    expanded := os.ExpandEnv(string(data))
    var c config.Config
    if err := yaml.Unmarshal([]byte(expanded), &c); err != nil {
        return fmt.Errorf("解析配置文件失败: %w", err)
    }

    if port > 0 {
        c.Server.Port = port
    }
    if host != "" {
        c.Server.Host = host
    }

    ctx := svc.NewServiceContext(c)

    // 设置 Gin 模式
    switch c.Server.Mode {
    case "prod":
        gin.SetMode(gin.ReleaseMode)
    case "test":
        gin.SetMode(gin.TestMode)
    default:
        gin.SetMode(gin.DebugMode)
    }

    r := gin.New()
    r.Use(gin.Recovery())
    r.Use(cors.Default())

    // 注册路由（含认证中间件）
    handler.RegisterHandlers(r, ctx)

    addr := fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
    fmt.Printf("Starting Croupier Server at %s (mode: %s)...\n", addr, c.Server.Mode)
    return r.Run(addr)
}
```

## 注意

- `conf.UseEnv()` 支持 `${VAR}` 展开，After 方案用 `os.ExpandEnv` 替代
- SSE 超时不再依赖 go-zero Timeout 字段，由 Gin 的 `http.Server.WriteTimeout` 控制
- `applyRuntimeDefaults` 函数中所有 `c.RestConf.*` 引用需同步更新
