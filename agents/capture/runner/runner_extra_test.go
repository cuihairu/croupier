package runner

// runner.go 补测：New 装配的 sendAlert/fetchConfig 上行 seam 在连接未就绪
// 时的兜底分支，以及 newConnector 按数据源类型的选取（legacy mysql /
// postgres / mongodb / 缺省 mysql）。

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/agents/capture/connector/mongo"
	"github.com/cuihairu/croupier/agents/capture/connector/pg"
	"github.com/cuihairu/croupier/agents/capture/source"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/require"
)

func TestSendAlertWithoutConnFails(t *testing.T) {
	r := New(testConfig(t, false), nil)
	_, err := r.sendAlert(context.Background(), &opsv1.CaptureRuleHitReport{})
	require.ErrorContains(t, err, "control conn not ready")
}

func TestFetchConfigWithoutConnFails(t *testing.T) {
	r := New(testConfig(t, false), nil)
	_, err := r.fetchConfig(context.Background())
	require.ErrorIs(t, err, errGatesConnNotReady)
}

func TestNewConnectorSelectsBySourceType(t *testing.T) {
	// legacy capture.mysql → Provider 桥接收口
	r := New(testConfig(t, false), nil)
	_, ok := r.newConnector().(*source.ProviderConnector)
	require.True(t, ok, "legacy mysql must yield provider connector")

	base := testConfig(t, false)
	base.Capture.MySQL = nil

	r = New(base, nil)
	_, ok = r.newConnector().(*source.MySQLConnector)
	require.True(t, ok, "empty sourceType must default to mysql connector")

	base.Capture.SourceType = "mysql"
	r = New(base, nil)
	_, ok = r.newConnector().(*source.MySQLConnector)
	require.True(t, ok)

	base.Capture.SourceType = "postgres"
	r = New(base, nil)
	_, ok = r.newConnector().(*pg.PostgresConnector)
	require.True(t, ok)

	base.Capture.SourceType = "mongodb"
	r = New(base, nil)
	_, ok = r.newConnector().(*mongo.MongoConnector)
	require.True(t, ok)
}
