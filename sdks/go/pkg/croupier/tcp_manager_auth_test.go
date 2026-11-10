package croupier

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Provider 握手鉴权（CR-FIX-11）：connectMetadata 把 config.AuthToken 注入
// 握手 metadata["authToken"]（平台保留键，agent 校验后丢弃、绝不入会话
// 元数据），且不得污染用户配置的 InstanceMetadata map。
func TestTCPManagerConnectMetadataCarriesAuthToken(t *testing.T) {
	m := &TCPManager{config: ClientConfig{
		InstanceMetadata: map[string]string{"serverId": "s1"},
		AuthToken:        " secret ",
	}}
	meta := m.connectMetadata()
	assert.Equal(t, "secret", meta["authToken"], "token must be trimmed into metadata")
	assert.Equal(t, "s1", meta["serverId"], "user metadata must survive")
	_, polluted := m.config.InstanceMetadata["authToken"]
	assert.False(t, polluted, "user's InstanceMetadata map must not be mutated")
}

func TestTCPManagerConnectMetadataOmitsEmptyToken(t *testing.T) {
	m := &TCPManager{config: ClientConfig{
		InstanceMetadata: map[string]string{"serverId": "s1"},
		AuthToken:        "   ",
	}}
	meta := m.connectMetadata()
	_, ok := meta["authToken"]
	assert.False(t, ok, "blank token must not be sent")
	assert.Equal(t, "s1", meta["serverId"])
}
