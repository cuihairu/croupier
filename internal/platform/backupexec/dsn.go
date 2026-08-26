package backupexec

import (
	"fmt"
	"net/url"
	"strings"
)

// parseMySQLDSN 解析 user:pass@tcp(host:port)/db 形态的 DSN。
func parseMySQLDSN(dsn string) (host, port, user, password, database string, err error) {
	// 尝试标准 URL 形态。
	if strings.HasPrefix(dsn, "mysql://") || strings.HasPrefix(dsn, "mysql+dial:") {
		u, perr := url.Parse(dsn)
		if perr == nil {
			return u.Hostname(), orDefault(u.Port(), "3306"), u.User.Username(), hiddenPassword(u), strings.TrimPrefix(u.Path, "/"), nil
		}
	}
	// go-sql-driver 形态：user:pass@tcp(host:port)/db?params
	atIdx := strings.LastIndex(dsn, "@tcp(")
	if atIdx < 0 {
		return "", "", "", "", "", fmt.Errorf("unsupported mysql DSN form")
	}
	cred := dsn[:atIdx]
	if cIdx := strings.Index(cred, ":"); cIdx >= 0 {
		user, password = cred[:cIdx], cred[cIdx+1:]
	} else {
		user = cred
	}
	rest := dsn[atIdx+len("@tcp("):]
	endIdx := strings.Index(rest, ")")
	if endIdx < 0 {
		return "", "", "", "", "", fmt.Errorf("unsupported mysql DSN form")
	}
	hostPort := rest[:endIdx]
	host = hostPort
	if cIdx := strings.LastIndex(hostPort, ":"); cIdx >= 0 {
		host, port = hostPort[:cIdx], hostPort[cIdx+1:]
	}
	port = orDefault(port, "3306")
	dbPart := rest[endIdx+1:]
	if sIdx := strings.Index(dbPart, "/"); sIdx >= 0 {
		database = dbPart[sIdx+1:]
		if pIdx := strings.IndexAny(database, "?"); pIdx >= 0 {
			database = database[:pIdx]
		}
	}
	return host, port, user, password, database, nil
}

// parsePostgresDSN 解析 postgres key=value 或 URL 形态 DSN。
func parsePostgresDSN(dsn string) (host, port, user, password, database string, err error) {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, perr := url.Parse(dsn)
		if perr != nil {
			return "", "", "", "", "", perr
		}
		return u.Hostname(), orDefault(u.Port(), "5432"), u.User.Username(), hiddenPassword(u), strings.TrimPrefix(u.Path, "/"), nil
	}
	// key=value 形态。
	for _, kv := range strings.Fields(dsn) {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "host":
			host = parts[1]
		case "port":
			port = parts[1]
		case "user":
			user = parts[1]
		case "password":
			password = parts[1]
		case "dbname":
			database = parts[1]
		}
	}
	if host == "" || user == "" || database == "" {
		return "", "", "", "", "", fmt.Errorf("postgres DSN missing host/user/dbname")
	}
	port = orDefault(port, "5432")
	return host, port, user, password, database, nil
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func hiddenPassword(u *url.URL) string {
	if u.User == nil {
		return ""
	}
	pw, _ := u.User.Password()
	return pw
}
