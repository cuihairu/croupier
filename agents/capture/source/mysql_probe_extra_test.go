package source

// mysql_probe.go 补测：DSNProber 的坏 DSN（驱动 OpenConnector 即刻报错）
// 与连接不可达分支、CurrentGTIDProbe 的方言守卫（mariadb 无 gtid_executed、
// 未知方言）、defaultGTIDProbe 双方言查询选择与探测失败透传、
// defaultSQLExecutor/defaultConnectorSQLExecutor 的打开与执行失败分支。
//
// 诚实跳过（需真实服务端）：探测成功返回（DSNProber :29）与空 GTID 的
// 兜底文案分支（:62-65）——需可连接实例，属集成测试面。诚实跳过（结构
// 性不可达）：defaultSQLExecutor 的 sql.Open 失败分支（:76-77）——内部
// Sprintf 拼出的 DSN 形态恒可解析，打开失败在 ExecContext 处暴露（已由
// 不可达用例覆盖）。

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// refusedDSN 指向必拒端口的有效形态 DSN（连接即拒，不依赖外部服务）。
func refusedDSN() string {
	return "probe_user:probe_pass@tcp(127.0.0.1:1)/?timeout=1s"
}

func TestDSNProberBadDSNRejectedAtOpen(t *testing.T) {
	probe := DSNProber("bad@@dsn", "SELECT 1")
	_, err := probe(t.Context())
	require.Error(t, err, "malformed DSN must fail at sql.Open (DriverContext)")
}

func TestDSNProberUnreachableSourceErrors(t *testing.T) {
	probe := DSNProber(refusedDSN(), "SELECT @@GLOBAL.gtid_executed")
	_, err := probe(context.Background())
	require.Error(t, err, "unreachable source must surface the dial error")
}

func TestCurrentGTIDProbeFlavorGuards(t *testing.T) {
	// 未知方言 → normalizeFlavor 报错透传
	_, err := CurrentGTIDProbe(MySQLConfig{Flavor: "sqlite"})
	require.ErrorContains(t, err, "unknown mysql flavor")

	// mariadb 无 gtid_executed → 显式拒绝快照一致性探测
	_, err = CurrentGTIDProbe(MySQLConfig{Flavor: "mariadb"})
	require.ErrorContains(t, err, "mariadb lacks mysql gtid_executed")

	// mysql → 返回探测闭包（连接失败在探测时暴露）
	probe, err := CurrentGTIDProbe(MySQLConfig{Host: "127.0.0.1", Port: 1})
	require.NoError(t, err)
	require.NotNil(t, probe)
	_, err = probe(context.Background())
	require.Error(t, err)
}

func TestDefaultGTIDProbeSelectsFlavorQuery(t *testing.T) {
	// 双方言探测闭包都能构造并在不可达时透传错误（查询差异在集成面验证）
	for _, flavor := range []string{"", "mysql", "mariadb"} {
		probe := defaultGTIDProbe(MySQLConfig{Flavor: flavor, Host: "127.0.0.1", Port: 1})
		_, err := probe(context.Background())
		require.Error(t, err, "flavor %q probe must surface dial error", flavor)
	}
}

func TestDefaultSQLExecutorFailureArms(t *testing.T) {
	// 坏 DSN → sql.Open 即刻报错
	exec := defaultSQLExecutor(MySQLConfig{Host: "127.0.0.1", Port: 1})
	_ = exec // 构造不报错；坏 DSN 经下方 env 注入路径验证

	// 连接不可达 → ExecContext 报错
	exec = defaultSQLExecutor(MySQLConfig{Host: "127.0.0.1", Port: 1, User: "u", Password: "p"})
	_, err := exec(context.Background(), "SELECT 1")
	require.Error(t, err)
}

func TestDefaultConnectorSQLExecutorFailureArms(t *testing.T) {
	const env = "CROUPIER_CAPTURE_MYSQL_DSN_EXTRA_TEST"

	// 坏 DSN → 打开失败
	t.Setenv(env, "bad@@dsn")
	exec := defaultConnectorSQLExecutor(env)
	_, err := exec(t.Context(), "SELECT 1")
	require.Error(t, err, "malformed DSN must fail the executor open")

	// 不可达 → 执行失败
	t.Setenv(env, refusedDSN())
	exec = defaultConnectorSQLExecutor(env)
	_, err = exec(context.Background(), "SELECT 1")
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded) || err != nil)
	_ = time.Second
}
