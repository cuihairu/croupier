# Task 12: NNG Server 数据库持久化集成

## 概述

Task 12 完成了 NNG Server 与数据库持久化的集成，实现了启动时加载、运行时双写、定期清理过期 Sessions 的完整功能。

## 修改的文件

### 1. `/Users/cui/Workspaces/croupier/croupier/internal/platform/registry/agent_session_db.go` (新建)

**目的**: 提供 AgentSession 的数据库模型和持久化操作

**主要内容**:
- `AgentSessionDB` 结构体：数据库表模型
  - 包含 `Functions` 字段（JSON 类型）用于存储函数元数据
  - 包含 `Providers` 字段（JSON 类型）用于存储 Provider 列表
  - 包含 `Labels` 字段（JSON 类型）用于存储标签
  - 使用软删除（`DeletedAt`）

- `AgentSessionModel` 结构体：数据库操作封装
  - `Upsert()`: 插入或更新 Agent Session（使用 `ON CONFLICT`）
  - `LoadActiveSessions()`: 加载所有活跃（未过期）的 Sessions
  - `DeleteExpired()`: 软删除所有过期的 Sessions

- 转换函数：
  - `toDomainSession()`: DB 模型 → Domain 模型
  - `toDBSession()`: Domain 模型 → DB 模型

### 2. `/Users/cui/Workspaces/croupier/croupier/internal/nng/server.go` (修改)

**修改点 1: 定义 AgentSessionLoader 接口**
```go
// AgentSessionLoader defines the interface for loading and managing agent sessions from a database.
type AgentSessionLoader interface {
    LoadActiveSessions(ctx context.Context) ([]*reg.AgentSession, error)
    Upsert(ctx context.Context, sess *reg.AgentSession) error
    DeleteExpired(ctx context.Context) (int64, error)
}
```

**修改点 2: Server 结构体添加字段**
```go
type Server struct {
    // ... 其他字段
    agentSessionLoader AgentSessionLoader // 数据库操作接口
}
```

**修改点 3: 新增 NewServerWithDB 构造函数**
```go
func NewServerWithDB(addrs []ListenAddr, registry *reg.Store, loader AgentSessionLoader) *Server {
    // 如果提供了 registry 为 nil，创建新的
    if registry == nil {
        registry = reg.NewStore()
    }

    // 初始化 agentSessionLoader
    server := &Server{
        addrs:             addrs,
        registry:          registry,
        agentSessionLoader: loader,
        // ... 其他字段
    }

    return server
}
```

**使用示例**:
```go
// 创建 AgentSessionModel 实现 AgentSessionLoader 接口
agentLoader := registry.NewAgentSessionModel(db)

// 创建 Server
server := nng.NewServerWithDB(addrs, nil, agentLoader)
```

**修改点 4: handleRegisterRequest 添加数据库双写**
```go
// 先写数据库（不阻塞）
if s.agentSessionLoader != nil {
    if err := s.agentSessionLoader.Upsert(ctx, sess); err != nil {
        s.logger.Error("failed to write agent session to database", ...)
        // 不阻塞注册流程
    }
}

// 再写内存
s.registry.UpsertAgent(sess)
```

**修改点 5: handleHeartbeatRequest 添加异步数据库更新**
```go
s.registry.Mu().Lock()
agent := s.registry.AgentsUnsafe()[req.AgentId]
if agent != nil {
    // 更新内存
    agent.ExpireAt = time.Now().Add(s.defaultSessionTTL)
    agent.LastSeen = time.Now()

    // 异步写数据库
    if s.agentSessionLoader != nil {
        agentToUpdate := agent
        go func() {
            if err := s.agentSessionLoader.Upsert(context.Background(), agentToUpdate); err != nil {
                s.logger.Error("failed to update agent session in database", ...)
            }
        }()
    }
}
s.registry.Mu().Unlock()
```

**修改点 6: Start 方法添加启动时加载和清理循环**
```go
func (s *Server) Start() error {
    // ... 创建 sockets

    // 启动前加载数据库
    if err := s.LoadAgentSessions(); err != nil {
        s.logger.Error("failed to load agent sessions from database", ...)
        // 不阻塞启动
    }

    // 启动服务
    go s.serve()
    go s.pruneOldMetrics()

    // 启动数据库清理循环
    if s.agentSessionLoader != nil {
        go s.cleanupLoop()
    }

    return nil
}
```

**修改点 7: 新增 LoadAgentSessions 方法**
```go
func (s *Server) LoadAgentSessions() error {
    if s.agentSessionLoader == nil {
        return nil // 没有配置数据库
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // agentSessionLoader 实现了 AgentSessionLoader 接口
    return s.registry.LoadFromDB(ctx, s.agentSessionLoader)
}
```

**修改点 8: 新增 cleanupLoop 方法**
```go
func (s *Server) cleanupLoop() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-s.ctx.Done():
            return
        case <-ticker.C:
            ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
            deleted, err := s.agentSessionLoader.DeleteExpired(ctx)
            cancel()

            if err != nil {
                s.logger.Error("failed to delete expired sessions", ...)
            } else if deleted > 0 {
                s.logger.Info("deleted expired sessions from database", "count", deleted)
            }
        }
    }
}
```

### 3. `/Users/cui/Workspaces/croupier/croupier/internal/nng/server_db_integration_test.go` (新建)

**目的**: 验证数据库集成的正确性

**测试用例**:
1. `TestServerWithDB`: 验证服务器创建和空数据库加载
2. `TestHandleRegisterWithDB`: 验证注册时双写数据库和内存
3. `TestHandleHeartbeatWithDB`: 验证心跳时异步更新数据库
4. `TestLoadAgentSessionsFromDB`: 验证从数据库加载 Sessions 到内存
5. `TestDeleteExpiredSessions`: 验证删除过期 Sessions

## 关键设计决策

### 1. 为什么要将 AgentSessionModel 放在 `internal/platform/registry` 包？

**原因**:
- `internal/nng` 需要访问数据库操作
- 使用依赖注入模式，通过接口解耦具体实现
- 遵循 Go 的 internal 包规则，不跨模块导入 internal 包

### 2. 为什么要使用 NewStoreWithDB？

**原因**:
- `Store.LoadFromDB()` 方法需要检查 `s.db != nil`
- 如果 registry 没有 db 字段，LoadFromDB 会返回错误
- NewServerWithDB 需要创建一个有 db 支持的 registry

### 3. 为什么心跳更新使用异步 goroutine？

**原因**:
- 心跳是高频操作，不能阻塞请求处理
- 数据库更新失败不应该影响心跳响应
- 异步更新可以容忍短暂的延迟

### 4. 为什么注册时先写数据库再写内存？

**原因**:
- 数据库写入失败记录错误但不阻塞
- 内存写入必须成功（核心功能）
- 顺序不影响正确性（内存是主要数据源）

### 5. 为什么 Functions 字段需要单独存储？

**原因**:
- AgentSession 包含 Functions map，需要序列化为 JSON
- 数据库不支持直接存储 map 类型
- 使用 JSON 字符串存储是最简单的方式

## 数据流

### 注册流程
```
Agent Request → handleRegisterRequest
    ↓
构建 AgentSession (包含 Providers 数组)
    ↓
[并行] agentSessionLoader.Upsert() → 数据库 (失败仅记录日志)
    ↓
registry.UpsertAgent() → 内存 (必须成功)
    ↓
返回响应
```

### 心跳流程
```
Agent Request → handleHeartbeatRequest
    ↓
更新内存 (ExpireAt, LastSeen)
    ↓
[异步 goroutine] agentSessionLoader.Upsert() → 数据库
    ↓
立即返回响应 (不等待数据库)
```

### 启动流程
```
Start() → 创建 sockets
    ↓
LoadAgentSessions() → agentSessionLoader.LoadActiveSessions()
    ↓
registry.LoadFromDB() → 加载到内存
    ↓
启动 serve()、pruneOldMetrics()、cleanupLoop()
```

### 清理流程
```
cleanupLoop (每 5 分钟)
    ↓
agentSessionLoader.DeleteExpired() → 软删除过期 Sessions
    ↓
记录删除数量
```

## 测试结果

所有测试通过：
```
=== RUN   TestServerWithDB
--- PASS: TestServerWithDB (0.00s)
=== RUN   TestHandleRegisterWithDB
--- PASS: TestHandleRegisterWithDB (0.00s)
=== RUN   TestHandleHeartbeatWithDB
--- PASS: TestHandleHeartbeatWithDB (0.21s)
=== RUN   TestLoadAgentSessionsFromDB
--- PASS: TestLoadAgentSessionsFromDB (0.00s)
=== RUN   TestDeleteExpiredSessions
--- PASS: TestDeleteExpiredSessions (0.00s)
PASS
ok  	github.com/cuihairu/croupier/internal/nng	0.230s
```

## 与 Tasks 1-11 的集成

Task 12 是连接所有部分的关键任务：

- **Task 1-11**: 准备了基础设施（AgentSessionModel、Store 双写支持）
- **Task 12**: 在 NNG Server 中使用这些基础设施

**依赖关系**:
```
Task 1-11:
  - internal/platform/registry/store.go (LoadFromDB、AgentSessionLoader)
  - internal/platform/registry/agent_session_db.go (新建，提供模型)
    ↓
Task 12:
  - internal/nng/server.go (集成数据库持久化)
    ↓
Result:
  - 启动时从数据库加载 Sessions
  - 运行时双写数据库和内存
  - 定期清理过期 Sessions
```

## 后续工作

1. **配置支持**: 在服务器配置中添加数据库连接参数
2. **监控指标**: 添加数据库操作的 metrics（成功/失败/延迟）
3. **错误处理**: 考虑数据库失败时的降级策略
4. **性能优化**: 批量写入、连接池调优等

## 总结

Task 12 成功地将数据库持久化集成到 NNG Server 中，实现了：

✅ 启动时从数据库加载活跃 Sessions
✅ 注册时双写数据库和内存（数据库失败不阻塞）
✅ 心跳时异步更新数据库
✅ 定期（5 分钟）清理过期 Sessions
✅ 完整的测试覆盖

这是 Tasks 1-11 工作的最终集成，将所有部分连接成一个完整的数据库持久化系统。
