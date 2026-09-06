package component

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// U6 模板参数化：validateTemplateParams 契约（key 唯一/nodeId 存在/prop 白名单）。

func treeJSON(t *testing.T, nodes string) json.RawMessage {
	t.Helper()
	require.JSONEq(t, nodes, string(json.RawMessage(nodes))) // ensure valid JSON
	return json.RawMessage(nodes)
}

func TestValidateTemplateParams(t *testing.T) {
	tree := treeJSON(t, `[{"id":"table","type":"fnTable","props":{"title":"列表","autoRun":true},"children":[{"id":"form","type":"fnForm","props":{}}]}]`)

	t.Run("合法参数原样通过", func(t *testing.T) {
		raw := json.RawMessage(`[{"key":"table.title","nodeId":"table","prop":"title","default":"玩家列表"},{"key":"table.autoRun","nodeId":"table","prop":"autoRun","default":false}]`)
		out, err := validateTemplateParams(raw, tree)
		require.NoError(t, err)
		var params []TemplateParam
		require.NoError(t, json.Unmarshal(out, &params))
		assert.Len(t, params, 2)
		assert.Equal(t, "table.title", params[0].Key)
	})

	t.Run("空与 null 规范化为 nil", func(t *testing.T) {
		for _, raw := range []json.RawMessage{nil, []byte(`null`), []byte(`[]`)} {
			out, err := validateTemplateParams(raw, tree)
			require.NoError(t, err)
			assert.Nil(t, out)
		}
	})

	t.Run("key 空或重复拒绝", func(t *testing.T) {
		out, err := validateTemplateParams(json.RawMessage(`[{"key":"  ","nodeId":"table","prop":"title"}]`), tree)
		assert.Nil(t, out)
		assert.ErrorContains(t, err, "key 不能为空")

		out, err = validateTemplateParams(json.RawMessage(`[{"key":"a","nodeId":"table","prop":"title"},{"key":"a","nodeId":"form","prop":"title"}]`), tree)
		assert.Nil(t, out)
		assert.ErrorContains(t, err, "重复")
	})

	t.Run("nodeId 不在 tree 拒绝（含子树节点命中）", func(t *testing.T) {
		out, err := validateTemplateParams(json.RawMessage(`[{"key":"a","nodeId":"ghost","prop":"title"}]`), tree)
		assert.Nil(t, out)
		assert.ErrorContains(t, err, "不存在于 tree")

		// 子树节点合法
		out, err = validateTemplateParams(json.RawMessage(`[{"key":"a","nodeId":"form","prop":"title"}]`), tree)
		require.NoError(t, err)
		assert.NotNil(t, out)
	})

	t.Run("prop 白名单外拒绝（执行类配置不可参数化）", func(t *testing.T) {
		for _, prop := range []string{"functionId", "rowActions", "onSuccess", "inputAssignments"} {
			raw := json.RawMessage(`[{"key":"a","nodeId":"table","prop":"` + prop + `"}]`)
			out, err := validateTemplateParams(raw, tree)
			assert.Nil(t, out, prop)
			assert.ErrorContains(t, err, "白名单", prop)
		}
	})
}

func TestCollectTreeNodeIDs(t *testing.T) {
	tree := json.RawMessage(`[{"id":"a","children":[{"id":"a1"},{"id":"a2"}]},{"id":"b"}]`)
	out := map[string]bool{}
	collectTreeNodeIDs(tree, out)
	assert.True(t, out["a"])
	assert.True(t, out["a1"])
	assert.True(t, out["a2"])
	assert.True(t, out["b"])
	assert.False(t, out["c"])
}
