package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	corelogs "github.com/cuihairu/croupier/core/logs"
	"github.com/cuihairu/croupier/core/report"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// TestUpstreamClient_ReportMetrics_FoldsLogs verifies the metrics reporting
// path folds tailed application logs into MetricsReport.logs.
func TestUpstreamClient_ReportMetrics_FoldsLogs(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	require.NoError(t, os.WriteFile(logPath, []byte("INFO hello\nERROR boom\n"), 0o644))

	client := &UpstreamClient{agentID: "a1"}
	tailer := corelogs.NewTailer([]string{logPath}, 0, 0)
	client.WithLogTailer(tailer)

	// Capture report via a fake reporter.
	got := make(chan *opsv1.MetricsReport, 1)
	fakeConn := &fakeConn{got: got}
	client.reporter = report.New(func() report.Conn { return fakeConn })

	client.reportMetrics(context.Background())
	select {
	case report := <-got:
		require.NotNil(t, report)
		assert.Len(t, report.Logs, 2)
		assert.Equal(t, "INFO hello", report.Logs[0].Message)
		assert.Equal(t, "ERROR boom", report.Logs[1].Message)
	case <-time.After(2 * time.Second):
		t.Fatal("no metrics report captured")
	}

	// Second cycle: no new lines -> no logs.
	client.reportMetrics(context.Background())
	select {
	case report := <-got:
		assert.Empty(t, report.Logs)
	case <-time.After(2 * time.Second):
		t.Fatal("no second metrics report captured")
	}
}

type fakeConn struct {
	got chan *opsv1.MetricsReport
}

func (c *fakeConn) Connected() bool { return true }
func (c *fakeConn) SendMetricEvent(ctx context.Context, data []byte) error {
	var r opsv1.MetricsReport
	if err := proto.Unmarshal(data, &r); err != nil {
		return err
	}
	select {
	case c.got <- &r:
	default:
	}
	return nil
}
func (c *fakeConn) SendAlertEvent(ctx context.Context, data []byte) error { return nil }
func (c *fakeConn) SendTaskEvent(ctx context.Context, data []byte) error  { return nil }
