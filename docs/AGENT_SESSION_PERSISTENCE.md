# Agent Session 持久化实现文档

## 概述

本文档描述 Croupier Server 的 Agent Session 持久化功能实现，确保 Server 重启后不丢失已注册的 Agent 和函数信息。

**实现日期**: 2026-02-08
**版本**: v0.1.1

---

## 功能特性

### 核心能力

1. **Agent Session 持久化**
   - Server 启动时从数据库恢复 Agent Sessions
   - Agent 注册时同步写入数据库
   - Agent 心跳时异步更新数据库
   - 定期清理过期 Sessions（每 5 分钟）

2. **完整函数元数据**
   - Agent 注册时传递完整的函数元数据
   - 支持 Category、Risk、Entity、Operation 等字段
   - 包含 DisplayName、Summary、Tags、Menu、Permissions

3. **Provider 配置增强**
   - Provider 配置支持 `game_id` 和 `env` 字段
   - OpenAPI Provider 存储完整的 OpenAPI 文档
   - 支持多租户（game_id）和多环境（env）隔离

---

## 数据库 Schema

### agent_sessions 表

```sql
CREATE TABLE `agent_sessions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `agent_id` varchar(64) NOT NULL,
  `game_id` varchar(64) DEFAULT NULL,
  `env` varchar(32) DEFAULT NULL,
  `rpc_addr` varchar(255) NOT NULL,
  `version` varchar(32) DEFAULT NULL,
  `region` varchar(64) DEFAULT NULL,
  `zone` varchar(64) DEFAULT NULL,
  `labels` json DEFAULT NULL,
  `functions` json DEFAULT NULL,
  `providers` json DEFAULT NULL,
  `expire_at` datetime NOT NULL,
  `last_seen` datetime NOT NULL,
  `created_at` datetime DEFAULT NULL,
  `updated_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_agent_id` (`agent_id`),
  KEY `idx_game_id` (`game_id`),
  KEY `idx_env` (`env`),
  KEY `idx_region` (`region`),
  KEY `idx_zone` (`zone`),
  KEY `idx_expire_at` (`expire_at`),
  KEY `idx_last_seen` (`last_seen`),
  KEY `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `agent_id` | string | Agent 唯一标识（唯一索引） |
| `game_id` | string | 游戏 ID（多租户） |
| `env` | string | 环境（prod/dev/staging） |
| `rpc_addr` | string | Agent 的 RPC 地址 |
| `version` | string | Agent 版本号 |
| `region` | string | 区域（如 us-west-1） |
| `zone` | string | 可用区（如 us-west-1a） |
| `labels` | JSON | 系统元数据（os, arch, hostname 等） |
| `functions` | JSON | 函数列表（map[string]FunctionMeta） |
| `providers` | JSON | Provider 列表（[]ProviderSession） |
| `expire_at` | datetime | 会话过期时间 |
| `last_seen` | datetime | 最后心跳时间 |

---

## API 变更

### NNG 控制端点

Agent 通过 NNG 协议（默认端口 19090）与 Server 通信：

#### Register（注册）

**请求**: `RegisterRequest`

```protobuf
message RegisterRequest {
  string agent_id = 1;
  string version = 2;
  repeated FunctionDescriptor functions = 3;  // 完整函数元数据
  string rpc_addr = 4;
  string game_id = 5;
  string env = 6;
  repeated AgentProcess processes = 7;
  uint32 ttl_seconds = 8;
  string region = 10;
  string zone = 11;
  map<string, string> labels = 12;
}
```

**完整函数元数据**:
- `id`: 函数 ID
- `version`: 版本号
- `category`: 分组类别（x-category）
- `risk`: 风险级别（x-risk）
- `entity`: 实体类型（x-entity）
- `operation`: 操作类型（x-operation）
- `enabled`: 是否启用
- `display_name`: 显示名称（I18nText）
- `summary`: 摘要（I18nText）
- `tags`: 标签列表
- `menu`: 菜单元数据
- `permissions`: 权限规范
- `input_schema`: 输入 JSON Schema
- `output_schema`: 输出 JSON Schema

#### Heartbeat（心跳）

**请求**: `HeartbeatRequest`

```protobuf
message HeartbeatRequest {
  string agent_id = 1;
  string session_id = 2;
}
```

**行为**:
- 更新内存中的 `expire_at` 和 `last_seen`
- 异步更新数据库中的 `expire_at` 和 `last_seen`

---

## Provider 配置

### providers.yaml 格式

```yaml
providers:
  examples:
    enabled: true
    game_id: "game-A"
    env: "prod"
    type: openapi
    config:
      openapi_spec: ./etc/openapi.example.yaml
      base_url: http://localhost:8080
```

### 配置字段

| 字段 | 类型 | 必需 | 说明 |
|------|------|------|------|
| `enabled` | bool | 是 | 是否启用 |
| `game_id` | string | 否 | 游戏 ID（多租户） |
| `env` | string | 否 | 环境（prod/dev/staging） |
| `type` | string | 是 | Provider 类型（openapi） |
| `config.openapi_spec` | string | 否 | OpenAPI 规范文件路径 |
| `config.base_url` | string | 否 | 服务基础 URL |

---

## 代码实现

### Server 端

#### 1. ServiceContext 集成

**文件**: `services/server/internal/svc/service_context.go`

```go
type ServiceContext struct {
    // ... 其他字段
    AgentSessionModel *reg.AgentSessionModel  // 新增
}

func NewServiceContext(c config.Config) *ServiceContext {
    // ... 初始化代码
    agentSessionModel := reg.NewAgentSessionModel(db)  // 新增
    // ...
    ctx := &ServiceContext{
        // ... 其他字段
        AgentSessionModel: agentSessionModel,  // 新增
    }
}
```

#### 2. NNG Server 启动

**文件**: `services/server/cmd/root.go`

```go
func startNNGControlServer(c *config.Config, svcCtx *svc.ServiceContext) {
    addr := c.Control.Addr
    if addr == "" {
        addr = ":19090"
    }
    if addr[0] == ':' {
        addr = "0.0.0.0" + addr
    }

    // 解析地址为 ListenAddr 数组
    addrs := []nng.ListenAddr{nng.ParseListenAddr(addr)}

    // 创建 NNG 控制服务器（带数据库持久化）
    nngServer := nng.NewServerWithDB(addrs, svcCtx.RegistryStore, svcCtx.AgentSessionModel)

    // 启动服务器
    if err := nngServer.Start(); err != nil {
        fmt.Printf("Failed to start NNG Control server: %v\n", err)
        return
    }

    localAddr, _ := nngServer.GetLocalAddr()
    fmt.Printf("Starting NNG ControlService on %s (SDK/Agent registration with DB persistence)...\n", localAddr)
}
```

#### 3. NNG Server 持久化逻辑

**文件**: `internal/nng/server.go`

**注册处理** (`handleRegisterRequest`):
```go
// Dual-write to database if configured
if s.agentSessionLoader != nil {
    if err := s.agentSessionLoader.Upsert(ctx, sess); err != nil {
        s.logger.Error("failed to write agent session to database", "agent_id", req.AgentId, "error", err)
        // Don't block registration if database write fails
    }
}

s.registry.UpsertAgent(sess)  // 写入内存
```

**心跳处理** (`handleHeartbeatRequest`):
```go
s.registry.Mu().Lock()
agent := s.registry.AgentsUnsafe()[req.AgentId]
if agent != nil {
    agent.ExpireAt = time.Now().Add(s.defaultSessionTTL)
    agent.LastSeen = time.Now()

    // Async dual-write to database if configured
    if s.agentSessionLoader != nil {
        agentToUpdate := agent
        go func() {
            if err := s.agentSessionLoader.Upsert(context.Background(), agentToUpdate); err != nil {
                s.logger.Error("failed to update agent session in database", "agent_id", req.AgentId, "error", err)
            }
        }()
    }
}
s.registry.Mu().Unlock()
```

**启动时加载** (`LoadAgentSessions`):
```go
func (s *Server) LoadAgentSessions() error {
    if s.agentSessionLoader == nil {
        return nil // No database configured
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Load sessions into registry
    if err := s.registry.LoadFromDB(ctx, s.agentSessionLoader); err != nil {
        return fmt.Errorf("failed to load agent sessions: %w", err)
    }

    return nil
}
```

**定期清理** (`cleanupLoop`):
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
                s.logger.Error("failed to delete expired sessions", "error", err)
            } else if deleted > 0 {
                s.logger.Info("deleted expired sessions from database", "count", deleted)
            }
        }
    }
}
```

### Agent 端

#### 1. 完整函数元数据注册

**文件**: `internal/app/agent/upstream.go`

```go
func (c *UpstreamClient) syncOnce(ctx context.Context) error {
    // ... 前置检查

    // Convert to FunctionDescriptors with complete information
    var funcs []*agentv1.FunctionDescriptor
    for fid, instances := range localData {
        meta := metaSnapshot[fid]
        desc := &agentv1.FunctionDescriptor{
            Id:      fid,
            Enabled: len(instances) > 0,
            Version: pickVersion(versionSnapshot[fid]),
        }

        // Copy all metadata fields if available
        if meta != nil {
            desc.Category = meta.Category
            desc.Risk = meta.Risk
            desc.Entity = meta.Entity
            desc.Operation = meta.Operation
            desc.InputSchema = meta.InputSchema
            desc.OutputSchema = meta.OutputSchema
            desc.Tags = meta.Tags

            // Set display_name and summary from metadata
            if meta.Summary != "" {
                desc.DisplayName = &componentv1.I18nText{
                    En: meta.Summary,
                    Zh: meta.Summary,
                }
            }
            if meta.Description != "" {
                desc.Summary = &componentv1.I18nText{
                    En: meta.Description,
                    Zh: meta.Description,
                }
            }

            // Generate menu metadata from category/entity
            if meta.Category != "" {
                desc.Menu = &componentv1.Menu{
                    Section: "Functions",
                    Group:   toTitle(meta.Category),
                    Path:    "/functions/" + fid,
                    Order:   100,
                    Hidden:  false,
                }
                if meta.Entity != "" {
                    desc.Menu.Group = toTitle(meta.Entity)
                }
            }

            // Generate permissions from operation type
            if meta.Operation != "" {
                desc.Permissions = &componentv1.PermissionSpec{
                    Verbs:   operationToVerbs(meta.Operation),
                    Scopes:  []string{"game", "env", "function_id"},
                    Defaults: []*componentv1.RoleBinding{
                        {Role: "admin", Verbs: []string{"*"}},
                        {Role: "operator", Verbs: operationToVerbs(meta.Operation)},
                    },
                }
            }
        }
        funcs = append(funcs, desc)
    }

    // ... 发送注册请求
}
```

#### 2. Provider 配置增强

**文件**: `internal/app/agent/provider.go`

```go
type ProviderEntry struct {
    Enabled bool                   `yaml:"enabled"`
    Type    string                 `yaml:"type"`
    GameID  string                 `yaml:"game_id"`  // 新增
    Env     string                 `yaml:"env"`      // 新增
    Config  map[string]interface{} `yaml:"config"`
}
```

#### 3. OpenAPI Provider 增强

**文件**: `internal/platform/openapi/provider.go`

```go
type Provider struct {
    config        provider.ProviderConfig
    openapiConfig *Config
    httpClient    *http.Client
    rateLimiter   ratelimit.Limiter
    methods       []string
    methodMap     map[string]*APIMethod
    openapiDoc    json.RawMessage       // 新增：存储原始 OpenAPI 文档
    mu            sync.RWMutex
}

// GetOpenAPIDoc returns the raw OpenAPI document JSON
func (p *Provider) GetOpenAPIDoc() json.RawMessage {
    p.mu.RLock()
    defer p.mu.RUnlock()
    return p.openapiDoc
}
```

---

## 数据流图

### Agent 注册流程

```
┌─────────┐         ┌─────────┐         ┌─────────┐         ┌──────────┐
│  Agent  │────────>│   NNG   │────────>│ Server  │────────>│ Registry  │
│         │ Register │         │ Handle  │         │          │
└─────────┘         └─────────┘         └─────────┘         └─────┬────┘
                                                                  │
                                                                  v
                                                           ┌──────────────┐
                                                           │ Database     │
                                                           │ (Dual-write) │
                                                           └──────────────┘
```

### Server 启动恢复流程

```
┌─────────┐         ┌─────────┐         ┌─────────┐         ┌──────────┐
│ Server  │         │  NNG    │         │Registry │         │ Database │
│  Start  │────────>│  Start  │────────>│LoadFromDB│────────>│LoadActive│
└─────────┘         └─────────┘         └─────────┘         └─────┬────┘
                                                                  │
                                                                  v
                                                           ┌──────────────┐
                                                           │Agent Sessions│
                                                           │  Restored    │
                                                           └──────────────┘
```

### 心跳更新流程

```
┌─────────┐         ┌─────────┐         ┌─────────┐
│  Agent  │────────>│   NNG   │────────>│ Registry │
│ Heartbeat│         │ Handle  │         │UpsertAgent│
└─────────┘         └─────────┘         └─────┬────┘
                                                  │
                                                  v
                                           ┌──────────────┐
                                           │ Memory Update│
                                           │ (sync)        │
                                           └──────────────┘
                                                  │
                                                  v
                                           ┌──────────────┐
                                           │ Database     │
                                           │ (async)       │
                                           └──────────────┘
```

---

## 配置示例

### Server 配置

**文件**: `etc/server.yaml`

```yaml
server:
  name: croupier-server
  host: "0.0.0.0"
  port: 8080

control:
  addr: ":19090"  # NNG Control Service 端口

database:
  driver: "sqlite"
  dsn: "./data/croupier.db"
  # 或 PostgreSQL:
  # driver: "postgres"
  # dsn: "host=localhost port=5432 user=croupier password=... dbname=croupier sslmode=disable"
```

### Agent 配置

**文件**: `etc/agent.yaml`

```yaml
agent:
  id: "agent-001"
  server_addr: "tcp://localhost:19090"
  game_id: "mygame"
  env: "prod"
  region: "us-west-1"
  zone: "us-west-1a"

providers:
  http-api:
    enabled: true
    game_id: "mygame"
    env: "prod"
    type: openapi
    config:
      openapi_spec: ./etc/openapi.http.yaml
      base_url: http://localhost:9000
```

---

## 监控与维护

### 日志关键字

**Server 启动**:
```
Starting NNG ControlService on tcp://0.0.0.0:19090 (SDK/Agent registration with DB persistence)...
loaded N active agent sessions from database
```

**Agent 注册**:
```
Agent registered via NNG agent_id=agent-001 game_id=mygame
```

**数据库操作**:
```
failed to write agent session to database agent_id=... error=...
deleted expired sessions from database count=5
```

### 数据库维护

**查看活跃 Sessions**:
```sql
SELECT
    agent_id,
    game_id,
    env,
    version,
    expire_at,
    last_seen,
    json_array_length(functions) as function_count
FROM agent_sessions
WHERE deleted_at IS NULL
  AND expire_at > datetime('now')
ORDER BY last_seen DESC;
```

**手动清理过期 Sessions**:
```sql
UPDATE agent_sessions
SET deleted_at = datetime('now')
WHERE expire_at <= datetime('now')
  AND deleted_at IS NULL;
```

**查看 Session 详情**:
```sql
SELECT
    agent_id,
    json_extract(functions, '$') as functions,
    json_extract(providers, '$') as providers
FROM agent_sessions
WHERE agent_id = 'agent-001'
LIMIT 1;
```

---

## 故障排查

### 问题：Server 重启后 Agent Session 丢失

**检查项**:
1. 数据库连接是否正常
2. `agent_sessions` 表是否存在
3. Server 日志是否有 "loaded X active agent sessions"

**解决方案**:
```bash
# 检查数据库
sqlite3 data/croupier.db "SELECT COUNT(*) FROM agent_sessions WHERE deleted_at IS NULL;"

# 检查 Server 配置
grep -A 10 "database:" etc/server.yaml

# 查看 Server 日志
tail -100 logs/croupier-server.log | grep "agent session"
```

### 问题：Agent 注册成功但数据库未写入

**检查项**:
1. Agent 是否传递了完整的 `game_id` 和 `env`
2. 数据库写入是否有错误日志
3. `Upsert` 操作是否成功

**解决方案**:
```bash
# 查看 Agent 注册日志
tail -100 logs/agent.log | grep "synced with upstream"

# 检查数据库记录
sqlite3 data/croupier.db "SELECT agent_id, game_id, env FROM agent_sessions ORDER BY created_at DESC LIMIT 10;"
```

### 问题：过期 Sessions 未自动清理

**检查项**:
1. `cleanupLoop` 是否正常运行
2. 清理间隔是否正确配置（默认 5 分钟）

**解决方案**:
```bash
# 检查 Server 日志
tail -f logs/croupier-server.log | grep "deleted expired"

# 手动触发清理（需要在 Server 代码中添加管理接口）
# 或直接执行 SQL：
UPDATE agent_sessions SET deleted_at = datetime('now') WHERE expire_at <= datetime('now');
```

---

## 性能考虑

### 异步写入策略

- **注册**: 同步写入数据库，失败不阻塞（仅记录日志）
- **心跳**: 异步写入数据库，避免影响心跳性能
- **定期清理**: 后台 goroutine 定期执行，不阻塞主流程

### 数据库优化

**索引**:
```sql
-- 查询优化
CREATE INDEX idx_expire_at_active ON agent_sessions(expire_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_game_env ON agent_sessions(game_id, env);
```

**连接池**:
```go
// GORM 配置示例
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    DisableAutomaticPing: false,
})
sqlDB, _ := db.DB()
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)
sqlDB.SetConnMaxLifetime(time.Hour)
```

---

## 测试验证

### 单元测试

**NNG Server 持久化测试**:
```bash
go test -v ./internal/nng/... -run "DB"
```

**测试覆盖**:
- `TestServerWithDB` - Server 创建和数据库加载
- `TestHandleRegisterWithDB` - 注册时双写
- `TestHandleHeartbeatWithDB` - 心跳时异步更新
- `TestLoadAgentSessionsFromDB` - 启动时加载
- `TestDeleteExpiredSessions` - 定期清理

### 集成测试

**端到端流程**:
1. 启动 Server（带数据库）
2. Agent 注册函数
3. 验证数据库记录
4. 重启 Server
5. 验证 Session 恢复
6. 等待过期，验证自动清理

---

## 未来改进

1. **集群支持**: 多个 Server 实例共享数据库
2. **分片策略**: 按游戏/环境分片存储
3. **缓存层**: Redis 缓存热点数据
4. **监控指标**: Prometheus 指标导出
5. **事件通知**: Session 变更事件推送

---

## 参考资料

- **协议定义**: `proto/croupier/agent/v1/register.proto`
- **实现代码**:
  - `internal/nng/server.go`
  - `internal/platform/registry/store.go`
  - `internal/platform/registry/agent_session_db.go`
  - `services/server/internal/svc/service_context.go`
- **测试代码**: `internal/nng/server_db_integration_test.go`

---

**文档版本**: 1.0
**最后更新**: 2026-02-08
