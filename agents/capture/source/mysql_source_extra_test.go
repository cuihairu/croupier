package source

// mysql.go 补测：NewMySQLSource 仅注入 ExecSQL 时的探测兜底装配、
// MySQLCapabilities 能力自报、Open 的未知方言拒绝与有效书签解析路径
// （不可达源快速失败，同步器无泄漏）、Close 的同步器释放与幂等。
//
// 诚实跳过（需真实服务端）：Open 成功尾段（事件泵启动）与 Close 后的
// 复制流真正断开——由 TEST_MYSQL_DSN 集成测试覆盖。

import (
	"context"
	"database/sql"
	"testing"

	"github.com/go-mysql-org/go-mysql/replication"
	"github.com/stretchr/testify/require"
)

func TestNewMySQLSourceExecSQLOnlyWiring(t *testing.T) {
	// 只注入 ExecSQL：探测闭包兜底装配（两者独立可选）
	s := NewMySQLSource(MySQLConfig{
		ExecSQL: func(_ context.Context, _ string) (sql.Result, error) { return nil, nil },
	}, nil)
	require.NotNil(t, s.probe)
	require.NotNil(t, s.execSQL)

	// 只注入 Probe：一致性锁连接兜底装配
	s2 := NewMySQLSource(MySQLConfig{
		Probe: func(context.Context) (string, error) { return "g:1", nil },
	}, nil)
	require.NotNil(t, s2.probe)
	require.NotNil(t, s2.execSQL)
}

func TestMySQLCapabilitiesContract(t *testing.T) {
	caps := MySQLCapabilities()
	require.True(t, caps.AfterImage)
	require.True(t, caps.TransactionMetadata)
	require.True(t, caps.ResumableCheckpoint)
}

func TestOpenRejectsUnknownFlavor(t *testing.T) {
	s := NewMySQLSource(MySQLConfig{Flavor: "sqlite"}, nil)
	_, err := s.Open(t.Context(), Bookmark{})
	require.ErrorContains(t, err, "unknown mysql flavor")
}

func TestOpenValidBookmarkParsesThenFailsFast(t *testing.T) {
	const gtid = "3E11FA47-71CA-11E1-9E33-C80AA9429562:1-5"
	s := newTestSource(nil)
	_, err := s.Open(t.Context(), Bookmark{Position: gtid})
	require.Error(t, err, "unreachable source must fail fast after bookmark parse")
	require.Nil(t, s.syncer, "failed open must not leak syncer")
	// 位点在连接建立前已进快照（Bookmark 随时可观测）
	require.Equal(t, "3e11fa47-71ca-11e1-9e33-c80aa9429562:1-5", s.Bookmark().Position, "gtid set normalizes to lowercase")
}

func TestCloseReleasesSyncerIdempotent(t *testing.T) {
	s := newTestSource(nil)
	require.NoError(t, s.Close(), "close without syncer is a no-op")

	s.syncer = replication.NewBinlogSyncer(replication.BinlogSyncerConfig{ServerID: 77})
	require.NoError(t, s.Close())
	require.Nil(t, s.syncer, "syncer must be released after close")
	require.NoError(t, s.Close(), "second close is a no-op")
}
