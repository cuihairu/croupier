package healthprobe

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// DefaultDBTimeout DB 探测硬超时（对齐 server 侧 dbmon 探针 5s 硬超时口径）。
const DefaultDBTimeout = 5 * time.Second

// QueryModePing 声明式跳过只读断言（只 Ping 连通）。
const QueryModePing = "NONE"

// DBChecker 数据库源连通探测：只用标准库 database/sql，core 零外部依赖不破——
// driver 由装配侧 blank import 注册（server/agent 各自依赖图已含 mysql/pgx）。
// Check 执行一次「连接+Ping+只读断言」；Query 为空用默认 SELECT 1，QueryModePing
// 跳过断言。
type DBChecker struct {
	DSN     string
	Driver  string
	Timeout time.Duration
	Query   string
}

// NewDBChecker 构造 DB 探测。
func NewDBChecker(dsn, driver string, timeout time.Duration) *DBChecker {
	return &DBChecker{DSN: dsn, Driver: driver, Timeout: timeout}
}

// Check implements Checker.
func (c *DBChecker) Check(ctx context.Context) error {
	return c.CheckWith(ctx, nil)
}

// CheckWith 在给定连接上执行同一套连通判定（Ping + 只读断言）；db 为 nil 时
// 自开连接（语义同 Check）。装配侧（dbmon 诊断端点）用它把连通判定与诊断
// 电池合到同一次执行（同一连接）——探测实现只留一份，不新开第二套连接。
// db 已就绪时 Driver/DSN 不参与（它们只用于自开连接）。
func (c *DBChecker) CheckWith(ctx context.Context, db *sql.DB) error {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = DefaultDBTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if db == nil {
		if strings.TrimSpace(c.Driver) == "" {
			return fmt.Errorf("db checker: driver required")
		}
		if strings.TrimSpace(c.DSN) == "" {
			return fmt.Errorf("db checker: dsn required")
		}
		var err error
		db, err = sql.Open(c.Driver, c.DSN)
		if err != nil {
			return fmt.Errorf("db open: %w", err)
		}
		defer db.Close()
		db.SetMaxOpenConns(1)
	}

	if err := db.PingContext(cctx); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}
	q := strings.TrimSpace(c.Query)
	if q == "" {
		q = "SELECT 1"
	}
	if strings.EqualFold(q, QueryModePing) {
		return nil
	}
	rows, err := db.QueryContext(cctx, q)
	if err != nil {
		return fmt.Errorf("db query: %w", err)
	}
	defer rows.Close()
	return rows.Err()
}

// Details implements Detailing：随结果上报的探测类型明细。
func (c *DBChecker) Details() map[string]string {
	q := strings.TrimSpace(c.Query)
	if q == "" {
		q = "SELECT 1"
	}
	return map[string]string{"probeType": "db", "driver": c.Driver, "assertion": q}
}
