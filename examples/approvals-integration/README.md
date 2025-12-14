# Approvals 存储集成示例

本示例展示了如何在 Croupier 系统中集成使用 PostgreSQL 或 SQLite 的 Approvals 存储。

## 集成步骤

### 1. 选择存储类型

根据你的需求选择合适的存储：

```go
import "github.com/cuihairu/croupier/internal/platform/approvals"

var store approvals.Store
var err error

// 开发环境 - 内存存储
store = approvals.NewMemStore()

// 单机部署 - SQLite 存储
store, err = approvals.NewSQLiteStore("data/approvals.db")

// 生产环境 - PostgreSQL 存储
store, err = approvals.NewPGStore("postgres://user:pass@localhost:5432/croupier?sslmode=disable")
```

### 2. 注入到服务层

在你的服务中注入存储：

```go
type Service struct {
    approvalStore approvals.Store
}

func NewService(approvalStore approvals.Store) *Service {
    return &Service{
        approvalStore: approvalStore,
    }
}
```

### 3. 使用存储 API

```go
// 创建审批请求
approval := &approvals.Approval{
    ID:         generateID(),
    State:      "pending",
    FunctionID: functionID,
    GameID:     gameID,
    Env:        env,
    Actor:      userID,
    Payload:    requestPayload,
}

created, err := s.approvalStore.Create(approval)

// 查询待审批
pending, total, err := s.approvalStore.List(
    approvals.Filter{State: "pending"},
    approvals.Page{Page: 1, Size: 20},
)

// 批准审批
approved, err := s.approvalStore.Approve(approvalID)
```

## 配置建议

### 开发环境配置
```yaml
approvals:
  type: memory  # 无需配置
```

### 测试环境配置
```yaml
approvals:
  type: sqlite
  dsn: "data/test-approvals.db"
```

### 生产环境配置
```yaml
approvals:
  type: postgres
  dsn: "postgres://croupier:password@db:5432/croupier?sslmode=disable"
  max_connections: 20
  max_idle_connections: 5
```

## 性能考虑

1. **索引优化**：已为常用查询字段创建索引
2. **连接池**：生产环境使用连接池管理数据库连接
3. **分页查询**：使用分页避免大量数据加载
4. **软删除**：支持软删除，数据可恢复

## 监控指标

建议监控以下指标：
- 待审批数量
- 审批处理时间
- 存储查询延迟
- 数据库连接数

## 备份策略

- **SQLite**：定期备份 .db 文件
- **PostgreSQL**：使用 pg_dump 或流复制

## 故障恢复

- 实现健康检查，确保存储可用
- 准备降级方案（如临时使用内存存储）
- 记录审计日志，便于故障追踪