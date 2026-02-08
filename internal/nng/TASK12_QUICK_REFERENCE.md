# Task 12 快速参考指南

## 如何使用数据库持久化的 NNG Server

### 1. 创建带数据库的 Server

```go
import (
    "github.com/cuihairu/croupier/internal/nng"
    "github.com/cuihairu/croupier/internal/platform/registry"
    "gorm.io/gorm"
    "gorm.io/driver/postgres"
)

// 打开数据库连接
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
if err != nil {
    log.Fatal(err)
}

// 自动迁移
db.AutoMigrate(&registry.AgentSessionDB{})

// 创建 AgentSessionLoader
agentLoader := registry.NewAgentSessionModel(db)

// 创建 Server
addrs := []nng.ListenAddr{nng.ParseListenAddr(":19090")}
server := nng.NewServerWithDB(addrs, nil, agentLoader)

// 启动 Server
if err := server.Start(); err != nil {
    log.Fatal(err)
}
```

### 2. 关键方法

#### NewServerWithDB
```go
func NewServerWithDB(addrs []ListenAddr, registry *reg.Store, loader AgentSessionLoader) *Server
```
- 创建支持数据库持久化的 NNG Server
- 使用依赖注入模式，通过 `AgentSessionLoader` 接口访问数据库
- 需要先创建 `AgentSessionModel` 实例并传入

#### LoadAgentSessions
```go
func (s *Server) LoadAgentSessions() error
```
- 从数据库加载活跃 Sessions 到内存
- 通常在 `Start()` 中自动调用
- 失败不阻塞启动

### 3. 数据流

#### 注册时（双写）
```
Request → handleRegisterRequest
    ↓
构建 AgentSession
    ↓
[同步] 写数据库（失败仅记录）
    ↓
[同步] 写内存（必须成功）
    ↓
返回响应
```

#### 心跳时（异步更新）
```
Request → handleHeartbeatRequest
    ↓
更新内存 (ExpireAt, LastSeen)
    ↓
[异步 goroutine] 写数据库
    ↓
立即返回
```

#### 启动时（加载）
```
Start() → 创建 sockets
    ↓
LoadAgentSessions() → 查询数据库
    ↓
LoadFromDB() → 加载到内存
    ↓
启动服务 goroutines
```

#### 后台（清理）
```
cleanupLoop (每 5 分钟)
    ↓
DeleteExpired() → 软删除过期 Sessions
    ↓
记录日志
```

### 4. 数据库表结构

```sql
CREATE TABLE agent_sessions (
    id SERIAL PRIMARY KEY,
    agent_id VARCHAR(64) UNIQUE NOT NULL,
    game_id VARCHAR(64),
    env VARCHAR(32),
    rpc_addr VARCHAR(255) NOT NULL,
    version VARCHAR(32),
    region VARCHAR(64),
    zone VARCHAR(64),
    labels JSON,
    functions JSON,
    providers JSON,
    expire_at TIMESTAMP NOT NULL,
    last_seen TIMESTAMP NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_agent_sessions_game_id ON agent_sessions(game_id);
CREATE INDEX idx_agent_sessions_env ON agent_sessions(env);
CREATE INDEX idx_agent_sessions_region ON agent_sessions(region);
CREATE INDEX idx_agent_sessions_zone ON agent_sessions(zone);
CREATE INDEX idx_agent_sessions_expire_at ON agent_sessions(expire_at);
CREATE INDEX idx_agent_sessions_last_seen ON agent_sessions(last_seen);
CREATE INDEX idx_agent_sessions_deleted_at ON agent_sessions(deleted_at);
```

### 5. JSON 字段格式

#### labels
```json
{"key1": "value1", "key2": "value2"}
```

#### functions
```json
{
    "function_id_1": {"enabled": true, "version": "1.0"},
    "function_id_2": {"enabled": false, "version": "2.0"}
}
```

#### providers
```json
[
    {
        "provider_id": "provider1",
        "game_id": "game1",
        "env": "prod",
        "addr": "localhost:19091",
        "version": "1.0.0",
        "last_seen_unix": 1707360000,
        "function_ids": ["func1", "func2"]
    }
]
```

### 6. 测试

运行集成测试：
```bash
go test ./internal/nng/ -v -run TestServerWithDB
```

运行所有数据库相关测试：
```bash
go test ./internal/nng/ -v
```

### 7. 监控要点

#### 关键日志
- `"loaded N active agent sessions from database"` - 启动时加载的 Sessions 数量
- `"failed to write agent session to database"` - 数据库写入失败（注册时）
- `"failed to update agent session in database"` - 数据库更新失败（心跳时）
- `"deleted expired sessions from database"` - 清理过期 Sessions

#### 性能指标
- 注册延迟：数据库写入是同步的（但不阻塞）
- 心跳延迟：数据库写入是异步的（不影响响应）
- 内存占用：Sessions 同时存在于内存和数据库

### 8. 故障处理

#### 数据库连接失败
- `Start()` 不会失败
- `LoadAgentSessions()` 返回错误但记录日志
- 后续操作降级为仅内存模式

#### 数据库写入失败
- 注册：记录错误，不阻塞请求
- 心跳：记录错误，不阻塞请求
- 内存操作不受影响

#### 数据库查询失败
- `LoadAgentSessions()` 返回错误
- Server 启动不受影响

### 9. 配置建议

#### 生产环境
```go
// 使用连接池
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    MaxIdleConns: 10,
    MaxOpenConns: 100,
    ConnMaxLifetime: time.Hour,
})

// 设置超时
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
server.LoadAgentSessions()
```

#### 开发环境
```go
// 使用 SQLite
db, err := gorm.Open(sqlite.Open("croupier.db"), &gorm.Config{})
```

### 10. 注意事项

⚠️ **重要**：
1. 心跳更新是异步的，可能短暂延迟
2. 数据库失败不会影响内存操作
3. 清理循环默认 5 分钟间隔
4. 使用软删除（deleted_at）
5. Functions 字段必须非空（初始化为空 map）

✅ **最佳实践**：
1. 定期备份数据库
2. 监控数据库连接池
3. 设置适当的索引
4. 监控过期 Sessions 清理
5. 使用连接池避免频繁连接

## 总结

Task 12 实现了完整的数据库持久化功能：
- ✅ 启动时加载
- ✅ 注册时双写
- ✅ 心跳时异步更新
- ✅ 定期清理过期
- ✅ 完整测试覆盖

所有功能都已实现并通过测试！
