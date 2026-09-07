package routes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// strings.Split 恒返回非空切片，"other" 兜底分支仅在缝隙注入
// 空切片时可达。验证兜底值与缝隙还原。
func TestExtractObjectName_EmptyPartsFallbackSeam(t *testing.T) {
	orig := splitFunctionID
	splitFunctionID = func(string, string) []string { return nil }
	t.Cleanup(func() { splitFunctionID = orig })

	logic := &GetRoutesLogic{}
	assert.Equal(t, "other", logic.extractObjectName("player.get"))
}
