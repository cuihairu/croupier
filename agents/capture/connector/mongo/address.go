package mongo

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const defaultMongoPort = 27017

// AddressFromDSN 从环境变量 DSN 解析连接地址（host:port，非凭据）——runner
// 装配 Readiness 探测目标用。凭据只留在环境变量内（错误消息只提变量名，
// 不回显取值）。mongodb+srv 走 SRV 记录解析（driver 同语义）；多 host 取
// 首个（探测面只需一个可达代表）。
func AddressFromDSN(envName string) (string, error) {
	dsn := os.Getenv(envName)
	if dsn == "" {
		return "", fmt.Errorf("mongo: environment variable %s not set (mongodb DSN)", envName)
	}
	u, err := url.Parse(dsn)
	if err != nil || u.Scheme != "mongodb" && u.Scheme != "mongodb+srv" {
		return "", fmt.Errorf("mongo: parse dsn from %s: want mongodb:// URI", envName)
	}
	host := u.Host
	if i := strings.IndexByte(host, ','); i >= 0 {
		host = host[:i]
	}
	if u.Scheme == "mongodb+srv" {
		h := strings.TrimSuffix(host, ":"+strconv.Itoa(defaultMongoPort))
		_, recs, err := net.LookupSRV("mongodb", "tcp", h)
		if err != nil || len(recs) == 0 {
			return "", fmt.Errorf("mongo: srv lookup %s via %s: %w", h, envName, err)
		}
		return net.JoinHostPort(recs[0].Target, strconv.Itoa(int(recs[0].Port))), nil
	}
	if !strings.Contains(host, ":") {
		host = net.JoinHostPort(host, strconv.Itoa(defaultMongoPort))
	}
	return host, nil
}
