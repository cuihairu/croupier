// UserMetadata 回归：用户自定义实例元数据过滤（保留键 + 空键剥离）。
package agentlocal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserMetadata_NilAndEmpty(t *testing.T) {
	t.Parallel()
	assert.Nil(t, UserMetadata(nil))
	assert.Nil(t, UserMetadata(map[string]string{}))
}

func TestUserMetadata_KeepsUserKeysDropsReserved(t *testing.T) {
	t.Parallel()
	in := map[string]string{
		"serverId":         "s1",
		"pod":              "game-7c4d",
		"sdkLanguage":      "go",   // 保留键：剥
		"gameId":           "demo", // 保留键：剥
		"protocol_version": "1",    // 保留键：剥
	}
	out := UserMetadata(in)
	assert.Equal(t, map[string]string{"serverId": "s1", "pod": "game-7c4d"}, out)
	// 输入不被修改
	assert.Len(t, in, 5)
}

func TestUserMetadata_BlankKeysDropped(t *testing.T) {
	t.Parallel()
	out := UserMetadata(map[string]string{"": "v", "  ": "v2", "real": "kept"})
	assert.Equal(t, map[string]string{"real": "kept"}, out)
}

func TestUserMetadata_AllReservedReturnsNil(t *testing.T) {
	t.Parallel()
	out := UserMetadata(map[string]string{"sdkVersion": "1.0.0", "env": "dev"})
	assert.Nil(t, out, "全部为保留键/空键时应返回 nil，避免空 map 上报")
}

func TestReservedMetadataKeys_ContainPlatformFixedKeys(t *testing.T) {
	t.Parallel()
	// 与 handleProviderConnect 固定字段同步的平台保留键：撞键时 agent 丢弃用户值。
	for _, key := range []string{"sdkLanguage", "sdkVersion", "sdkName", "protocol_version", "gameId", "env"} {
		assert.Contains(t, ReservedMetadataKeys, key)
	}
}
