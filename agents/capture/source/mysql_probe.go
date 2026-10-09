package source

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql" // database/sql 驱动注册（gtid 探测连接）
)

// GTIDProbe 空位点时的起点探测：返回源当前 gtid_executed 字符串。
// 位点失效降级路径——「从可得起点重扫」= 从当前时刻起订阅，不回放历史。
type GTIDProbe func(ctx context.Context) (string, error)

func defaultGTIDProbe(cfg MySQLConfig) GTIDProbe {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s", cfg.User, cfg.Password, cfg.Host, cfg.Port)
	return func(ctx context.Context) (string, error) {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return "", err
		}
		defer func() { _ = db.Close() }()
		var gtid string
		if err := db.QueryRowContext(ctx, "SELECT @@GLOBAL.gtid_executed").Scan(&gtid); err != nil {
			return "", err
		}
		if gtid == "" {
			return "", fmt.Errorf("gtid_executed is empty: gtid_mode must be ON")
		}
		return gtid, nil
	}
}
