package source

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql" // database/sql 驱动注册（gtid 探测连接）
)

// GTIDProbe 空位点时的起点探测：返回源当前 gtid_executed 字符串。
// 位点失效降级路径——「从可得起点重扫」= 从当前时刻起订阅，不回放历史。
type GTIDProbe func(ctx context.Context) (string, error)

// DSNProber builds a probe that executes one SQL statement against a plain
// database connection. It keeps credential handling at the source boundary.
func DSNProber(dsn, query string) GTIDProbe {
	return func(ctx context.Context) (string, error) {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return "", err
		}
		defer func() { _ = db.Close() }()
		var value string
		if err := db.QueryRowContext(ctx, query).Scan(&value); err != nil {
			return "", err
		}
		return value, nil
	}
}

// CurrentGTIDProbe reads the source's GTID set in the caller's consistency
// window. It is only valid for MySQL; MariaDB lacks gtid_executed and must
// not be treated as recovery-capable.
func CurrentGTIDProbe(cfg MySQLConfig) (GTIDProbe, error) {
	if flavor, err := cfg.normalizeFlavor(); err != nil || flavor != "mysql" {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("source: mariadb lacks mysql gtid_executed for snapshot consistency")
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s", cfg.User, cfg.Password, cfg.Host, cfg.Port)
	return DSNProber(dsn, "SELECT @@GLOBAL.gtid_executed"), nil
}

func defaultGTIDProbe(cfg MySQLConfig) GTIDProbe {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s", cfg.User, cfg.Password, cfg.Host, cfg.Port)
	// 方言分叉（todo 4307）：GTID 位点变量与格式两侧不同——
	//   mysql:   @@GLOBAL.gtid_executed（UUID:interval 集合）
	//   mariadb: @@GLOBAL.gtid_current_pos（domain-server-seq；MariaDB 没有
	//            gtid_executed 变量，查询直接报错）
	flavor, _ := cfg.normalizeFlavor() // 未知值由 Open 报错；探测侧兜底 mysql
	query := "SELECT @@GLOBAL.gtid_executed"
	if flavor == "mariadb" {
		query = "SELECT @@GLOBAL.gtid_current_pos"
	}
	probe := DSNProber(dsn, query)
	return func(ctx context.Context) (string, error) {
		gtid, err := probe(ctx)
		if err == nil && gtid == "" {
			if flavor == "mariadb" {
				return "", fmt.Errorf("gtid_current_pos is empty: source has no binlog events yet")
			}
			return "", fmt.Errorf("gtid_executed is empty: gtid_mode must be ON")
		}
		return gtid, err
	}
}

func defaultSQLExecutor(cfg MySQLConfig) func(context.Context, string) (sql.Result, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?timeout=5s", cfg.User, cfg.Password, cfg.Host, cfg.Port)
	return func(ctx context.Context, query string) (sql.Result, error) {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		defer func() { _ = db.Close() }()
		return db.ExecContext(ctx, query)
	}
}

func defaultConnectorSQLExecutor(dsnEnv string) func(context.Context, string) (sql.Result, error) {
	return func(ctx context.Context, query string) (sql.Result, error) {
		db, err := sql.Open("mysql", os.Getenv(dsnEnv))
		if err != nil {
			return nil, err
		}
		defer func() { _ = db.Close() }()
		return db.ExecContext(ctx, query)
	}
}
