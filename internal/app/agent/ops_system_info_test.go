package agent

// §10.5 指纹可先落项用例：SystemInfo 自报 agent 二进制内容 sha256（运行
// 实例指纹，spawn 语义：进程不重启值不变）——失联排查锚点与自升级指纹
// 面的 wire 前置，不依赖 §10.5 四项拍板。

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSystemInfoCarriesSelfBinaryFingerprint(t *testing.T) {
	info := GetSystemInfo("agent-a", "test", nil)
	require.NotNil(t, info)

	self, err := os.Executable()
	require.NoError(t, err)
	want := sha256HexFile(self)
	require.NotEmpty(t, want, "test binary must be hashable")

	fp := info.GetAgentBinarySha256()
	assert.Equal(t, want, fp, "SystemInfo carries the running binary sha256")
	assert.Regexp(t, `^[0-9a-f]{64}$`, fp)
}

func TestSelfBinaryFingerprintStableAcrossCalls(t *testing.T) {
	// spawn 语义：同进程内重复取值不变（缓存一次，不做重复 IO）。
	a := GetSystemInfo("agent-a", "test", nil).GetAgentBinarySha256()
	b := GetSystemInfo("agent-a", "test", nil).GetAgentBinarySha256()
	assert.Equal(t, a, b)
}
