package source

// mysql_connector.go 补测：Discover 的 DSN 打开失败与 information_schema
// 查询不可达分支、SnapshotWithProbe 的守卫链（未 Open/缺探测/表名形态）
// 与打开/事务失败收口、quote* 与 snapshotImage 的纯函数分支、Poll/Ack
// 的守卫与桥接透传（不可达源快速失败）。
//
// 诚实跳过（结构性不可达）：Discover/SnapshotWithProbe 的 sql.Open 失败
// 分支——Open 前置守卫用同一 ParseDSN 先行拒绝，任何打开级坏 DSN 到不了
// 这两处。诚实跳过（需真实 MySQL，由 TEST_MYSQL_DSN 集成测试覆盖）：
// information_schema 行集解析（Discover 主体）、LOCK INSTANCE FOR BACKUP
// 快照事务的成功路径与行扫描循环、探测成功后的解锁注入分支。

import (
	"context"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/stretchr/testify/require"
)

const connectorExtraDSNEnv = "CROUPIER_CAPTURE_MYSQL_DSN_EXTRA_CONNECTOR_TEST"

// openedConnector 构造已 Open 的契约连接器（不建连接；DSN 指向不可达地址）。
func openedConnector(t *testing.T, dsn string) *MySQLConnector {
	t.Helper()
	t.Setenv(connectorExtraDSNEnv, dsn)
	cfg := connector.SourceConfig{
		SourceID:   "mysql-extra",
		SourceType: "mysql",
		DSNEnv:     connectorExtraDSNEnv,
		Options:    map[string]string{"serverId": "424242", "tables": "game.items"},
	}
	c := NewMySQLConnector(cfg, nil)
	require.NoError(t, c.Open(t.Context(), cfg))
	return c
}

func TestDiscoverQueryFailClosure(t *testing.T) {
	// 有效形态但不可达：information_schema 查询失败收口（Tables 白名单
	// 过滤循环在查询前装配）
	c := openedConnector(t, "u:p@tcp(127.0.0.1:1)/?timeout=1s")
	_, err := c.Discover(t.Context(), connector.DiscoveryRequest{Tables: []string{"game.items"}})
	require.ErrorContains(t, err, "discover columns")
}

func TestSnapshotWithProbeGuardChain(t *testing.T) {
	// 未 Open
	fresh := NewMySQLConnector(connector.SourceConfig{DSNEnv: connectorExtraDSNEnv}, nil)
	_, err := fresh.SnapshotWithProbe(t.Context(), connector.SnapshotRequest{}, nil)
	require.ErrorContains(t, err, "not open")

	// 缺一致性探测闭包
	c := openedConnector(t, "u:p@tcp(127.0.0.1:1)/")
	_, err = c.SnapshotWithProbe(t.Context(), connector.SnapshotRequest{}, nil)
	require.ErrorContains(t, err, "snapshot consistency probe is required")

	// 表名形态：必须 database.table（"a.b.c" 的表名含点由行扫描阶段的
	// quoteSnapshotTable 拦截，此守卫只做 Cut 切分检查）
	for _, table := range []string{"nodot", ".tbl", "db."} {
		_, err = c.SnapshotWithProbe(t.Context(),
			connector.SnapshotRequest{Tables: []string{table}},
			func(context.Context) (string, error) { return "g:1", nil })
		require.ErrorContains(t, err, "must be database.table", "table %q", table)
	}
}

func TestSnapshotWithProbeBeginFailClosure(t *testing.T) {
	// 不可达 → 快照事务开启失败（readonly repeatable-read BeginTx）
	c := openedConnector(t, "u:p@tcp(127.0.0.1:1)/?timeout=1s")
	_, err := c.SnapshotWithProbe(t.Context(),
		connector.SnapshotRequest{Tables: []string{"game.items"}},
		func(context.Context) (string, error) { return "g:1", nil })
	require.ErrorContains(t, err, "snapshot begin")
}

func TestQuoteSnapshotHelpers(t *testing.T) {
	require.Equal(t, "`game`", quoteSnapshotIdentifier("game"))
	require.Equal(t, "```we``ird`", quoteSnapshotIdentifier("`we`ird"))

	out, err := quoteSnapshotTable("game.items")
	require.NoError(t, err)
	require.Equal(t, "`game`.`items`", out)

	for _, table := range []string{"nodot", ".tbl", "db.", "a.b.c"} {
		_, err = quoteSnapshotTable(table)
		require.ErrorContains(t, err, "invalid snapshot table", "table %q", table)
	}
}

func TestSnapshotImagePositionalEncoding(t *testing.T) {
	image, err := snapshotImage([]any{int64(7), "sword", nil})
	require.NoError(t, err)
	require.JSONEq(t, "7", string(image["0"]))
	require.Equal(t, `"sword"`, string(image["1"]))
	require.JSONEq(t, "null", string(image["2"]))

	// 不可序列化列 → 带下标的错误包装
	_, err = snapshotImage([]any{"ok", make(chan int)})
	require.ErrorContains(t, err, "column 1")
	require.True(t, strings.Contains(err.Error(), "chan"), "marshal failure must be surfaced")
}

func TestConnectorPollDelegatesAndFailsFast(t *testing.T) {
	// 未 Open 守卫
	fresh := NewMySQLConnector(connector.SourceConfig{DSNEnv: connectorExtraDSNEnv}, nil)
	require.ErrorContains(t, fresh.AckCheckpoint(t.Context(), connector.Position{Kind: "gtid", Value: "g:1"}),
		"not open")

	// Open 后 Poll 透传桥接：不可达源在探测处快速失败
	c := openedConnector(t, "u:p@tcp(127.0.0.1:1)/?timeout=1s")
	_, err := c.Poll(t.Context(), connector.PollRequest{})
	require.ErrorContains(t, err, "probe current mysql position")

	// Ack 透传：kind 不符在桥接处拒绝
	require.ErrorContains(t, c.AckCheckpoint(t.Context(), connector.Position{Kind: "lsn", Value: "1"}),
		"invalid mysql checkpoint")
}
