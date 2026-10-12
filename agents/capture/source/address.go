package source

import (
	"fmt"
	"os"
	"strings"

	// go-sql-driver DSN 解析（与 mysql_connector.go 同源：只取非凭据的连接
	// 地址，供 Readiness 探测使用）。
	drivermysql "github.com/go-sql-driver/mysql"
)

const defaultMySQLPort = 3306

// AddressFromDSN 从环境变量 DSN 解析连接地址（host:port，非凭据）。Runner 用
// 它装配 Readiness 探测目标——凭据仍只在环境变量内，不进配置文件、不落日志
// （错误消息只提环境变量名）。
//
// unix socket 地址不可 TCP 探测：返回空串由调用方降级（跳过探测并记日志），
// 不伪造一个探测不到的目标。
func AddressFromDSN(envName string) (string, error) {
	dsn := os.Getenv(envName)
	if dsn == "" {
		return "", fmt.Errorf("source: environment variable %s not set (mysql DSN)", envName)
	}
	dsnCfg, err := drivermysql.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("source: parse dsn from %s: %w", envName, err)
	}
	if dsnCfg.Net == "unix" {
		// unix socket 地址不可 TCP 探测：空串由调用方降级（跳过探测）。
		return "", nil
	}
	addr := dsnCfg.Addr
	if !strings.Contains(addr, ":") {
		addr = fmt.Sprintf("%s:%d", addr, defaultMySQLPort)
	}
	return addr, nil
}
