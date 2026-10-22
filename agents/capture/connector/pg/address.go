package pg

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgconn"
)

const defaultPostgresPort = 5432

// AddressFromDSN 从环境变量 DSN 解析连接地址（host:port，非凭据）——runner
// 装配 Readiness 探测目标用。凭据只留在环境变量内（错误消息只提变量名，
// 不回显取值）。
func AddressFromDSN(envName string) (string, error) {
	dsn := os.Getenv(envName)
	if dsn == "" {
		return "", fmt.Errorf("pg: environment variable %s not set (postgres DSN)", envName)
	}
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return "", fmt.Errorf("pg: parse dsn from %s: %w", envName, err)
	}
	port := cfg.Port
	if port == 0 {
		port = defaultPostgresPort
	}
	return net.JoinHostPort(cfg.Host, strconv.Itoa(int(port))), nil
}
