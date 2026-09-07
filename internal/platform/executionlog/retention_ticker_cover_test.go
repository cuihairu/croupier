package executionlog

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetentionRunSweepsOnTickerUntilCancelled(t *testing.T) {
	db := newTestDB(t)
	r := NewRetention(db, RetentionConfig{
		ExecutionLogDays: 7,
		TaskLogDays:      7,
		Interval:         time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	r.Run(ctx)
	time.Sleep(50 * time.Millisecond)
	cancel()

	summary := r.Sweep(context.Background())
	require.Equal(t, int64(0), summary.ExecutionLogsDeleted)
	require.Equal(t, int64(0), summary.TaskRunsDeleted)
}
