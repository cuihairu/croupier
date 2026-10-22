package agentscope

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNonBlank(t *testing.T) {
	t.Run("blank and whitespace-only are blank", func(t *testing.T) {
		for _, v := range []string{"", " ", "\t", "\n", " \t\r\n "} {
			assert.False(t, NonBlank(v), "value %q must be blank", v)
		}
	})

	t.Run("content around whitespace is non-blank", func(t *testing.T) {
		for _, v := range []string{"demo", " demo", "demo ", " demo ", "\tprod\n"} {
			assert.True(t, NonBlank(v), "value %q must be non-blank", v)
		}
	})
}

func TestIdentity(t *testing.T) {
	t.Run("all fields present passes", func(t *testing.T) {
		assert.NoError(t, Identity("agent-1", "tcp://server:19090", "demo", "prod"))
	})

	t.Run("missing field reports its agent key", func(t *testing.T) {
		for _, tc := range []struct {
			fix func(id, addr, game, env string) (string, string, string, string)
			key string
		}{
			{func(_, a, g, e string) (string, string, string, string) { return " ", a, g, e }, "agent.id"},
			{func(i, _, g, e string) (string, string, string, string) { return i, "\t", g, e }, "agent.serverAddr"},
			{func(i, a, _, e string) (string, string, string, string) { return i, a, "\n", e }, "agent.gameId"},
			{func(i, a, g, _ string) (string, string, string, string) { return i, a, g, "  " }, "agent.env"},
		} {
			id, addr, game, env := tc.fix("agent-1", "server:19090", "demo", "prod")
			err := Identity(id, addr, game, env)
			assert.EqualError(t, err, tc.key+" is required")
		}
	})

	t.Run("declaration order decides the first error", func(t *testing.T) {
		// 多字段同时缺：固定报声明顺序的第一个（id），不随 map 序漂移。
		err := Identity("", "", "", "")
		assert.EqualError(t, err, "agent.id is required")

		err = Identity("agent-1", "", "", "")
		assert.EqualError(t, err, "agent.serverAddr is required")
	})
}
