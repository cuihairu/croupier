package settings

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogsSettings_DefaultAndL3 日志维护快照（OPEN-ISSUES #54）：
// 默认 0（跟随配置文件），L3 覆盖后值与来源同步更新。
func TestLogsSettings_DefaultAndL3(t *testing.T) {
	resetForTest()
	store := newStore(t)
	l := InitLayered(context.Background(), &ConfigInput{}, store)

	snap := l.LogsSettings()
	assert.Equal(t, 0, snap.RetentionDays)
	assert.Equal(t, "default", snap.Sources["retentionDays"])

	require.NoError(t, store.Set(context.Background(), KeyLogRetentionDays, json.RawMessage(`30`), "tester"))
	l.Reload(context.Background(), store)

	snap = l.LogsSettings()
	assert.Equal(t, 30, snap.RetentionDays)
	assert.Equal(t, "database", snap.Sources["retentionDays"])
}
