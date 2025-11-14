# 🔥 Croupier Go SDK 热重载示例

这个示例展示了如何在Go游戏服务器中集成Croupier SDK的热重载功能。

## 🚀 快速开始

### 1. 安装依赖

```bash
# 安装Air热重载工具（如果还没有安装）
go install github.com/cosmtrek/air@latest

# 安装项目依赖
go mod init go-hotreload-example
go mod tidy
```

### 2. 启动Croupier Agent

```bash
# 在另一个终端启动Agent
cd ../../
make build
./bin/croupier-agent --config configs/agent.example.yaml
```

### 3. 启动热重载开发服务器

```bash
# 使用Air启动热重载
air

# 或者直接运行
go run main.go
```

## 🔧 配置说明

### Air配置 (.air.toml)

- **监听文件类型**: `.go`, `.yaml`, `.json`
- **排除目录**: `tmp`, `vendor`, `testdata`
- **构建延迟**: 1秒（防止频繁触发）
- **自动清理**: 退出时清理临时文件

### 热重载配置

```go
hotConfig := croupier.HotReloadConfig{
    Enabled:                 true,        // 启用热重载
    AutoReconnect:          true,         // 自动重连
    ReconnectDelay:         5 * time.Second,  // 重连延迟
    MaxRetryAttempts:       5,            // 最大重试次数
    HealthCheckInterval:    30 * time.Second, // 健康检查间隔
    GracefulShutdownTimeout: 30 * time.Second, // 优雅关闭超时
}
```

## 🎯 功能演示

### 1. 自动重连机制

当Air重启进程时，SDK会：
- 检测到连接断开
- 自动重连到Agent
- 重新注册所有函数
- 恢复正常服务

### 2. 函数热重载

示例中演示了两种重载方式：

**单函数重载**:
```go
newDesc := croupier.FunctionDescriptor{
    ID:      "player.ban",
    Version: "1.1.0",
}
hotReloader.ReloadFunction("player.ban", newDesc, handlePlayerBanV2)
```

**批量重载**:
```go
functions := map[string]croupier.FunctionDescriptor{
    "server.status": { ID: "server.status", Version: "2.0.0" },
}
handlers := map[string]croupier.FunctionHandler{
    "server.status": handleServerStatusV2,
}
hotReloader.ReloadFunctions(functions, handlers)
```

### 3. 优雅关闭

```go
// Ctrl+C时触发优雅关闭
hotReloader.GracefulShutdown(30 * time.Second)
```

## 📊 监控和调试

### 热重载状态查看

```go
status := hotReloader.GetReloadStatus()
fmt.Printf("重连次数: %d\n", status.ReconnectCount)
fmt.Printf("函数重载: %d\n", status.FunctionReloads)
fmt.Printf("失败次数: %d\n", status.FailedReloads)
```

### 文件监听

启用文件监听后，SDK会监控指定目录：
- 配置文件变更自动重载
- 支持多种文件格式 (`.yaml`, `.json`)
- 防抖机制避免频繁触发

## 🛠️ 开发工作流

### 1. 修改代码
- 编辑 `main.go` 中的函数
- 修改函数版本或实现
- Air会自动检测并重新编译

### 2. 测试函数调用
```bash
# 测试玩家封禁（会调用新版本函数）
curl -X POST http://localhost:8080/api/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "player.ban",
    "payload": "{\"player_id\":\"123\",\"reason\":\"cheating\"}"
  }'

# 测试服务器状态
curl -X POST http://localhost:8080/api/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "server.status",
    "payload": "{}"
  }'
```

### 3. 查看日志
- Air会显示构建过程
- SDK会输出重载状态
- 函数调用日志实时显示

## 🔍 故障排除

### 常见问题

1. **连接失败**
   ```
   Failed to connect to agent
   ```
   - 检查Agent是否正在运行
   - 确认端口19090未被占用
   - 检查网络连接

2. **重载失败**
   ```
   Failed to reload function
   ```
   - 检查函数描述符格式
   - 确认函数处理器正确
   - 查看详细错误日志

3. **Air启动失败**
   ```
   air: command not found
   ```
   - 确认已安装Air: `go install github.com/cosmtrek/air@latest`
   - 检查GOPATH/bin是否在PATH中

### 日志级别

设置环境变量控制日志详细程度：
```bash
export LOG_LEVEL=debug   # 详细日志
export LOG_LEVEL=info    # 标准日志
export LOG_LEVEL=warn    # 仅警告
```

## 🎮 生产环境考虑

虽然这个示例主要用于开发环境，但热重载功能在生产环境也有用途：

- **配置热更新**: 不重启服务更新配置
- **紧急修复**: 快速部署关键修复
- **灰度发布**: 逐步更新函数版本

生产环境建议：
- 关闭Air自动重载
- 使用手动触发重载
- 增加更多安全检查
- 启用审计日志

## 📚 相关文档

- [SDK热重载支持文档](../../docs/SDK_HOTRELOAD_SUPPORT.md)
- [热更新方案总览](../../docs/HOT_RELOAD_SOLUTIONS.md)
- [Croupier架构说明](../../README.md)

---

*🔥 享受高效的热重载开发体验！*