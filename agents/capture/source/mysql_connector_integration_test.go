//go:build integration

package source

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cuihairu/croupier/agents/capture/connector"
)

// MySQLConnector 契约适配器活源冒烟（todo 4306 增量三，CI capture-matrix
// 与本地 docker 同跑法）：MySQLSource 包进统一契约后走完整链——Open
// （DSNEnv 通道）→ Poll 空位点 → 事件按契约桥接（位置下标行像/位点
// kind=gtid）→ Checkpoint → Close。
//
// 环境变量：TEST_MYSQL_DSN（缺席跳过；要求源端 binlog ROW + GTID on，与
// TestCaptureSourceFlavors 同一容器约定）。

func TestMySQLConnectorContract(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN not configured")
	}
	const dbname = "capture_mcadapter_mysql"
	mustExecStmts(t, dsn,
		"DROP DATABASE IF EXISTS "+dbname,
		"CREATE DATABASE "+dbname,
		"CREATE TABLE "+dbname+".t1 (id INT PRIMARY KEY AUTO_INCREMENT, v VARCHAR(64))",
		fmt.Sprintf("INSERT INTO %s.t1 (v) VALUES ('seed')", dbname),
	)
	t.Cleanup(func() {
		mustExecStmts(t, dsn, "DROP DATABASE IF EXISTS "+dbname)
	})

	cfg := connector.SourceConfig{
		SourceID:   "mysql-itest",
		SourceType: "mysql",
		DSNEnv:     "TEST_MYSQL_DSN",
		Options: map[string]string{
			"serverId": "424243",
			"flavor":   "mysql",
			"tables":   dbname + ".t1",
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	c := NewMySQLConnector(cfg, nil)
	if err := c.Open(ctx, cfg); err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = c.Close(ctx) }()

	// Discover：元数据 + 能力同源（契约面的摄取前置）。
	disc, err := c.Discover(ctx, connector.DiscoveryRequest{Tables: []string{dbname + ".t1"}})
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(disc.Tables) != 1 || disc.Tables[0].Name != dbname+".t1" {
		t.Fatalf("discover tables = %+v", disc.Tables)
	}
	if len(disc.Tables[0].PrimaryKey) != 1 || disc.Tables[0].PrimaryKey[0] != "id" {
		t.Fatalf("discover pk = %+v", disc.Tables[0].PrimaryKey)
	}
	if !disc.Capabilities.AfterImage || !disc.Capabilities.ResumableCheckpoint {
		t.Fatalf("capabilities = %+v", disc.Capabilities)
	}

	// Poll：空位点起订，桥接事件按契约形态断言。
	events, err := c.Poll(ctx, connector.PollRequest{Tables: []string{dbname + ".t1"}})
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	mustExecStmts(t, dsn, fmt.Sprintf("INSERT INTO %s.t1 (v) VALUES ('marker-mc')", dbname))
	deadline := time.After(15 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("event stream closed before marker-mc")
			}
			if ev.Operation != connector.OpInsert || string(ev.After["1"]) != `"marker-mc"` {
				continue
			}
			if ev.Database != dbname || ev.Table != "t1" {
				t.Fatalf("identity = %+v", ev)
			}
			if ev.ColumnsMapped {
				t.Fatal("mysql rows must stay positional (columnsMapped=false)")
			}
			if ev.Position.Kind != "gtid" || ev.Position.Value == "" || !ev.Position.Valid() {
				t.Fatalf("position = %+v", ev.Position)
			}
			if err := ev.Validate(); err != nil {
				t.Fatalf("contract validation: %v", err)
			}
			// Checkpoint：GTID 位点快照。
			cp, err := c.Checkpoint(ctx)
			if err != nil || cp.Kind != "gtid" || cp.Value == "" {
				t.Fatalf("checkpoint = %+v err=%v", cp, err)
			}
			return
		case <-deadline:
			t.Fatal("timed out waiting for marker-mc")
		}
	}
}
