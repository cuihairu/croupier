package healthprobe

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeartbeatFileBeatAndAge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run", "agent.heartbeat")
	h := NewHeartbeatFile(path)
	assert.Equal(t, path, h.Path())

	_, ok := h.LastBeat()
	assert.False(t, ok) // 未打点

	require.NoError(t, h.Beat())
	last, ok := h.LastBeat()
	require.True(t, ok)
	assert.WithinDuration(t, time.Now(), last, time.Second)

	age, ok := h.Age()
	require.True(t, ok)
	assert.GreaterOrEqual(t, age, time.Duration(0))
	assert.Less(t, age, time.Second)

	// 再次打点刷新 mtime。
	time.Sleep(5 * time.Millisecond)
	require.NoError(t, h.Beat())
	last2, _ := h.LastBeat()
	assert.False(t, last2.Before(last))
}
