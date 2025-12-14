# Approvals 存储配置

本文档介绍了如何配置 Croupier 系统中的 Approvals（审批）存储。

## 支持的存储类型

Croupier 支持三种 Approvals 存储类型：

1. **内存存储（MemStore）** - 适用于测试和开发环境
2. **PostgreSQL 存储（PGStore）** - 适用于生产环境
3. **SQLite 存储（SQLiteStore）** - 适用于单机部署

## 配置示例

### 1. 内存存储（默认）

```go
package main

import "github.com/cuihairu/croupier/internal/platform/approvals"

func main() {
    // 创建内存存储
    store := approvals.NewMemStore()

    // 使用存储...
}
```

### 2. PostgreSQL 存储

```go
package main

import (
    "log"
    "github.com/cuihairu/croupier/internal/platform/approvals"
)

func main() {
    // PostgreSQL DSN 格式: postgres://user:password@host:port/database?sslmode=disable
    dsn := "postgres://croupier:password@localhost:5432/croupier?sslmode=disable"

    store, err := approvals.NewPGStore(dsn)
    if err != nil {
        log.Fatal(err)
    }

    // 使用存储...
}
```

### 3. SQLite 存储

```go
package main

import (
    "log"
    "github.com/cuihairu/croupier/internal/platform/approvals"
)

func main() {
    // SQLite DSN 可以是文件路径或内存模式
    dsn := "data/croupier.db"  // 文件存储

    store, err := approvals.NewSQLiteStore(dsn)
    if err != nil {
        log.Fatal(err)
    }

    // 使用存储...
}
```

## 数据库模式

使用 PostgreSQL 或 SQLite 时，系统会自动创建 `approvals` 表：

```sql
CREATE TABLE `approvals` (
  `id` varchar(255) NOT NULL PRIMARY KEY,
  `state` varchar(50) NOT NULL,
  `function_id` varchar(255) NOT NULL,
  `game_id` varchar(255) NOT NULL,
  `env` varchar(100) NOT NULL,
  `actor` varchar(255) NOT NULL,
  `mode` varchar(50) DEFAULT 'invoke',
  `idempotency_key` varchar(255),
  `route` varchar(500),
  `target_service_id` varchar(255),
  `hash_key` varchar(255),
  `payload` blob,
  `reason` text,
  `created_at` timestamp NOT NULL,
  `updated_at` timestamp NOT NULL,
  `deleted_at` timestamp,

  -- 索引
  INDEX `idx_approvals_state` (`state`),
  INDEX `idx_approvals_function_id` (`function_id`),
  INDEX `idx_approvals_game_id` (`game_id`),
  INDEX `idx_approvals_env` (`env`),
  INDEX `idx_approvals_actor` (`actor`),
  INDEX `idx_approvals_idempotency_key` (`idempotency_key`),
  INDEX `idx_approvals_target_service_id` (`target_service_id`),
  INDEX `idx_approvals_hash_key` (`hash_key`),
  INDEX `idx_approvals_created_at` (`created_at`),
  INDEX `idx_approvals_updated_at` (`updated_at`),
  INDEX `idx_approvals_deleted_at` (`deleted_at`)
);
```

## 使用建议

- **开发/测试环境**: 使用 MemStore，无需数据库依赖
- **生产环境**: 使用 PostgreSQL，支持高并发和集群部署
- **单机部署**: 使用 SQLite，轻量级且易于部署

## 迁移数据

从 MemStore 迁移到 PostgreSQL 或 SQLite：

```go
// 1. 创建内存存储并加载数据
memStore := approvals.NewMemStore()

// 2. 创建目标存储
targetStore, err := approvals.NewPGStore("postgres://...")

// 3. 遍历并迁移数据
// （需要根据具体需求实现迁移逻辑）
```