//go:build integration

package source

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-mysql-org/go-mysql/mysql"
	_ "github.com/go-sql-driver/mysql"
)

// 活源兼容性冒烟（todo 4307，CI capture-matrix 与本地 docker 同跑法）：
// 每个 flavor（TEST_MYSQL_DSN / TEST_MARIADB_DSN，缺席跳过）走完整链——
// 建库建表写种子行 → 空书签 Open（探测当前 GTID 起点）→ 追写一行 →
// 白名单过滤后收到该行事件 → Bookmark 位点按方言可解析回读。
//
// 源端前置（CI service/本地 docker 同参数）：binlog 开 + ROW + server_id，
// MySQL 另需 --gtid-mode=ON --enforce-gtid-consistency=ON（MariaDB GTID 恒开）。

type flavorCase struct {
	name   string
	flavor string
	dsnEnv string
}

func flavorCases(t *testing.T) []flavorCase {
	t.Helper()
	cases := []flavorCase{
		{name: "mysql", flavor: "mysql", dsnEnv: "TEST_MYSQL_DSN"},
		{name: "mariadb", flavor: "mariadb", dsnEnv: "TEST_MARIADB_DSN"},
	}
	var out []flavorCase
	for _, c := range cases {
		if os.Getenv(c.dsnEnv) != "" {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		t.Skip("no TEST_MYSQL_DSN / TEST_MARIADB_DSN configured")
	}
	return out
}

// mustExecStmts 逐条执行 DDL/DML（go-sql-driver 默认不开 multi-statement，
// 逐条既是兼容做法也让失败定位到语句级；库/表先 DROP 再建，测试可重复跑）。
func mustExecStmts(t *testing.T, dsn string, stmts ...string) {
	t.Helper()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("admin open: %v", err)
	}
	defer func() { _ = db.Close() }()
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("admin exec %q: %v", s, err)
		}
	}
}

// waitEvent 从事件 channel 等指定表的首个 marker 命中事件（超时即失败）。
func waitEvent(t *testing.T, events <-chan ChangeEvent, marker string) ChangeEvent {
	t.Helper()
	deadline := time.After(15 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatalf("event stream closed before %q", marker)
			}
			if strings.Contains(string(ev.AfterJSON), marker) {
				return ev
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event with marker %q", marker)
		}
	}
}

func TestCaptureSourceFlavors(t *testing.T) {
	for _, fc := range flavorCases(t) {
		t.Run(fc.name, func(t *testing.T) {
			dsn := os.Getenv(fc.dsnEnv)
			dbname := "capture_flavor_" + fc.name
			mustExecStmts(t, dsn,
				"DROP DATABASE IF EXISTS "+dbname,
				"CREATE DATABASE "+dbname,
				"CREATE TABLE "+dbname+".t1 (id INT PRIMARY KEY AUTO_INCREMENT, v VARCHAR(64))",
				fmt.Sprintf("INSERT INTO %s.t1 (v) VALUES ('seed')", dbname),
			)

			cfg := MySQLConfig{
				Host:     "127.0.0.1",
				Port:     portFromDSN(t, dsn),
				User:     userFromDSN(t, dsn),
				Password: passFromDSN(t, dsn),
				ServerID: 424242,
				Flavor:   fc.flavor,
				Tables:   map[string]bool{dbname + ".t1": true},
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			s := NewMySQLSource(cfg, nil)
			// 空书签：探测当前起点（验证方言 GTID 变量/格式分支）。
			events, err := s.Open(ctx, Bookmark{})
			if err != nil {
				t.Fatalf("open with empty bookmark: %v", err)
			}

			// 追写一行 → 期望事件（白名单过滤语义同链验证）。
			mustExecStmts(t, dsn, fmt.Sprintf("INSERT INTO %s.t1 (v) VALUES ('marker-live')", dbname))
			ev := waitEvent(t, events, "marker-live")
			if ev.Database != dbname || ev.Table != "t1" || ev.Op != OpInsert {
				t.Fatalf("event identity mismatch: %+v", ev)
			}

			// 位点快照按方言可解析（书签回放语义的前提）。
			bm := s.Bookmark()
			if !bm.Valid() {
				t.Fatalf("bookmark empty after events")
			}
			if _, err := mysql.ParseGTIDSet(fc.flavor, bm.Position); err != nil {
				t.Fatalf("bookmark %q not parseable as %s: %v", bm.Position, fc.flavor, err)
			}
			t.Logf("[%s] event ok, bookmark=%s", fc.flavor, bm.Position)

			if err := s.Close(); err != nil {
				t.Fatalf("close: %v", err)
			}
		})
	}
}

// ---- DSN 拆解（服务容器 DSN 形态 root:root@tcp(127.0.0.1:3306)/）----

func portFromDSN(t *testing.T, dsn string) uint16 {
	t.Helper()
	i := strings.Index(dsn, "tcp(")
	if i < 0 {
		t.Fatalf("dsn without tcp host: %s", dsn)
	}
	host := dsn[i+4:]
	j := strings.Index(host, ":")
	k := strings.Index(host, ")")
	if j < 0 || k < 0 || j > k {
		t.Fatalf("dsn without port: %s", dsn)
	}
	var port uint16
	if _, err := fmt.Sscanf(host[j+1:k], "%d", &port); err != nil {
		t.Fatalf("bad port in dsn %s: %v", dsn, err)
	}
	return port
}

func credFromDSN(t *testing.T, dsn string) (string, string) {
	t.Helper()
	j := strings.Index(dsn, "@")
	if j < 0 {
		t.Fatalf("dsn without credentials: %s", dsn)
	}
	cred := dsn[:j]
	u, p, ok := strings.Cut(cred, ":")
	if !ok {
		t.Fatalf("dsn credentials without colon: %s", dsn)
	}
	return u, p
}

func userFromDSN(t *testing.T, dsn string) string { u, _ := credFromDSN(t, dsn); return u }
func passFromDSN(t *testing.T, dsn string) string { _, p := credFromDSN(t, dsn); return p }
