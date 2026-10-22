package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cuihairu/croupier/internal/common/labels"
)

// agent 注册自报的系统标签（os/arch/hostname/ip）必须保留进会话：
// /ops/nodes 的连接来源 IP 经 LB 后全为 LB 地址，自报标签才是区分
// agent 的关键。同时限制数量与长度，防止注册路径被塞入超大标签集。
func TestSanitizeAgentLabels(t *testing.T) {
	t.Run("nil 与空输入返回空 map", func(t *testing.T) {
		assert.Empty(t, sanitizeAgentLabels(nil))
		assert.Empty(t, sanitizeAgentLabels(map[string]string{}))
	})

	t.Run("保留正常系统标签", func(t *testing.T) {
		out := sanitizeAgentLabels(map[string]string{
			"os": "linux", "arch": "amd64", "hostname": "agent-1", "ip": "172.18.0.5",
		})
		assert.Equal(t, map[string]string{
			"os": "linux", "arch": "amd64", "hostname": "agent-1", "ip": "172.18.0.5",
		}, out)
	})

	t.Run("丢弃空键与空值", func(t *testing.T) {
		out := sanitizeAgentLabels(map[string]string{"": "v", "k": "", "  ": "v"})
		assert.Empty(t, out)
	})

	t.Run("超长值截断且键数封顶", func(t *testing.T) {
		long := strings.Repeat("x", 500)
		out := sanitizeAgentLabels(map[string]string{"big": long})
		assert.Len(t, out["big"], 256)

		many := map[string]string{}
		for i := 0; i < 50; i++ {
			many["k"+string(rune('a'+i%26))+string(rune('0'+i/26))] = "v"
		}
		assert.Len(t, sanitizeAgentLabels(many), 32)
	})

	t.Run("超长键丢弃", func(t *testing.T) {
		out := sanitizeAgentLabels(map[string]string{strings.Repeat("k", 100): "v"})
		assert.Empty(t, out)
	})

	// 平台契约键（agent 注册自报的运行二进制指纹，system-info 持久锚点
	// 唯一数据源）豁免数量上限：Go map 迭代次序随机，满额 break 会按
	// 运气静默丢弃锚点——限额约束的是用户自选标签量，平台键固定 1 个不占额度。
	t.Run("平台契约键豁免限额不被丢弃", func(t *testing.T) {
		many := map[string]string{}
		for i := 0; i < 50; i++ {
			many["k"+string(rune('a'+i%26))+string(rune('0'+i/26))] = "v"
		}
		many[labels.AgentBinarySHA256] = "fp-anchor"
		out := sanitizeAgentLabels(many)
		assert.Equal(t, "fp-anchor", out[labels.AgentBinarySHA256])
		assert.Len(t, out, 33) // 32 用户标签 + 1 平台键
	})

	t.Run("平台契约键空值不入集合", func(t *testing.T) {
		many := map[string]string{}
		for i := 0; i < 50; i++ {
			many["k"+string(rune('a'+i%26))+string(rune('0'+i/26))] = "v"
		}
		many[labels.AgentBinarySHA256] = "   "
		out := sanitizeAgentLabels(many)
		_, ok := out[labels.AgentBinarySHA256]
		assert.False(t, ok)
		assert.Len(t, out, 32)
	})

	t.Run("平台契约键超长值截断后仍保留", func(t *testing.T) {
		out := sanitizeAgentLabels(map[string]string{
			labels.AgentBinarySHA256: strings.Repeat("f", 500),
		})
		assert.Len(t, out[labels.AgentBinarySHA256], 256)
	})
}
