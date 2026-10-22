package logx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsTerminal(t *testing.T) {
	// 普通文件非终端
	f, err := os.Create(filepath.Join(t.TempDir(), "regular.txt"))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	assert.False(t, isTerminal(f))

	// 字符设备（/dev/null）判为终端
	devNull, err := os.Open("/dev/null")
	require.NoError(t, err)
	defer func() { _ = devNull.Close() }()
	assert.True(t, isTerminal(devNull))
}
