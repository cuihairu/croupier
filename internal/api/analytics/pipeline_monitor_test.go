package analytics

import (
	"context"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type stubRows struct{ next bool }

func (r stubRows) Next() bool        { return r.next }
func (r stubRows) Scan(...any) error { return nil }
func (r stubRows) Close() error      { return nil }

type stubConn struct {
	// connErr is what the connect step reports; queryErr what Query returns.
	connErr  error
	queryErr error
	rows     warehouseRows
	query    string
}

func (c *stubConn) Query(_ context.Context, q string, _ ...any) (warehouseRows, error) {
	c.query = q
	if c.queryErr != nil {
		return nil, c.queryErr
	}
	if c.rows != nil {
		return c.rows, nil
	}
	return stubRows{next: false}, nil
}

func newMonitorTestSink(t *testing.T) *model.AlertModel {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return model.NewAlertModel(db)
}

func newTestMonitor(sink *model.AlertModel, conn *stubConn) *PipelineMonitor {
	m := NewPipelineMonitor(sink, nil)
	m.conn = func() (warehouseConn, error) {
		if conn != nil {
			return conn, conn.connErr
		}
		return nil, errWarehouseDisabled
	}
	return m
}

func TestMonitor_DisabledIsSilent(t *testing.T) {
	sink := newMonitorTestSink(t)
	m := newTestMonitor(sink, nil) // warehouse disabled
	require.NoError(t, m.Check(context.Background()))
	alerts, _, err := sink.List(context.Background(), model.ListAlertsOptions{})
	require.NoError(t, err)
	assert.Empty(t, alerts)
}

func TestMonitor_ClickHouseUnreachableFiresAndResolves(t *testing.T) {
	sink := newMonitorTestSink(t)
	conn := &stubConn{connErr: errors.New("dial tcp refused"), queryErr: errors.New("dial tcp refused")}
	m := newTestMonitor(sink, conn)

	require.NoError(t, m.Check(context.Background()))
	fired, err := sink.FindByAlertID(context.Background(), "pipe:clickhouse_unreachable")
	require.NoError(t, err)
	assert.Equal(t, PipelineAlertStatusFiring, fired.Status)
	assert.Equal(t, AlertTypeDataPipeline, fired.Type)
	assert.Equal(t, "critical", fired.Level)

	// Recovers on the next pass.
	conn.connErr = nil
	conn.queryErr = nil
	require.NoError(t, m.Check(context.Background()))
	resolved, err := sink.FindByAlertID(context.Background(), "pipe:clickhouse_unreachable")
	require.NoError(t, err)
	assert.Equal(t, PipelineAlertStatusResolved, resolved.Status)
}

func TestMonitor_DeadLetterBacklogThreshold(t *testing.T) {
	sink := newMonitorTestSink(t)
	m := NewPipelineMonitor(sink, nil)
	m.cfg.DeadThreshold = 3
	m.dead = stubCounter{n: 5}

	require.NoError(t, m.Check(context.Background()))
	alert, err := sink.FindByAlertID(context.Background(), "pipe:dead_letter_backlog:events")
	require.NoError(t, err)
	assert.Equal(t, PipelineAlertStatusFiring, alert.Status)
	assert.Equal(t, AlertTypeDataQuality, alert.Type)

	// Back below threshold resolves it.
	m.dead = stubCounter{n: 1}
	require.NoError(t, m.Check(context.Background()))
	alert, err = sink.FindByAlertID(context.Background(), "pipe:dead_letter_backlog:events")
	require.NoError(t, err)
	assert.Equal(t, PipelineAlertStatusResolved, alert.Status)
}

func TestMonitor_MQBacklogThreshold(t *testing.T) {
	sink := newMonitorTestSink(t)
	m := NewPipelineMonitor(sink, nil)
	m.cfg.MQBacklogLimit = 100
	m.dead = stubCounter{n: 5000}

	require.NoError(t, m.Check(context.Background()))
	alert, err := sink.FindByAlertID(context.Background(), "pipe:mq_backlog:events")
	require.NoError(t, err)
	assert.Equal(t, PipelineAlertStatusFiring, alert.Status)
	assert.Equal(t, "critical", alert.Level)
}

func TestMonitor_DedupFiringAlert(t *testing.T) {
	sink := newMonitorTestSink(t)
	conn := &stubConn{connErr: errors.New("down"), queryErr: errors.New("down")}
	m := newTestMonitor(sink, conn)

	require.NoError(t, m.Check(context.Background()))
	require.NoError(t, m.Check(context.Background()))
	require.NoError(t, m.Check(context.Background()))

	// Dedup: still exactly one firing alert for the same key.
	alerts, _, err := sink.List(context.Background(), model.ListAlertsOptions{})
	require.NoError(t, err)
	n := 0
	for _, a := range alerts {
		if a.AlertID == "pipe:clickhouse_unreachable" && a.Status == PipelineAlertStatusFiring {
			n++
		}
	}
	assert.Equal(t, 1, n, "firing alert must be deduplicated, not duplicated")
}

func TestMonitor_RefireAfterResolve(t *testing.T) {
	sink := newMonitorTestSink(t)
	conn := &stubConn{connErr: errors.New("down"), queryErr: errors.New("down")}
	m := newTestMonitor(sink, conn)

	require.NoError(t, m.Check(context.Background()))
	conn.connErr = nil
	conn.queryErr = nil
	require.NoError(t, m.Check(context.Background())) // resolves
	conn.connErr = errors.New("down again")
	conn.queryErr = errors.New("down again")
	require.NoError(t, m.Check(context.Background())) // re-fires

	alert, err := sink.FindByAlertID(context.Background(), "pipe:clickhouse_unreachable")
	require.NoError(t, err)
	assert.Equal(t, PipelineAlertStatusFiring, alert.Status)
}

// stubCounter implements DeadLetterCounter with a fixed length.
type stubCounter struct{ n int64 }

func (s stubCounter) XLen(_ context.Context, _ string) *redis.IntCmd {
	return redis.NewIntResult(s.n, nil)
}
