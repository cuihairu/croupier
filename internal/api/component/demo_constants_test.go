package component

import (
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- 构造函数 ----

func TestBuildDemoConstantTemplates(t *testing.T) {
	templates, err := buildDemoConstantTemplates()
	require.NoError(t, err)
	require.Len(t, templates, len(demoConstants))

	keys := map[string]bool{}
	for _, tpl := range templates {
		keys[tpl.Key] = true
		assert.True(t, len(tpl.Key) > len("consts--demo-"), "key 前缀 consts--demo-")
		assert.Equal(t, "常量", tpl.Category)
		assert.False(t, tpl.Builtin, "示例模板必须可删除（非 builtin）")
		assert.Equal(t, "demo", tpl.CreatedBy)

		// LocalizedText 契约：zh-CN / en-US
		var name map[string]string
		require.NoError(t, json.Unmarshal(tpl.Name, &name))
		assert.Contains(t, name, "zh-CN")
		assert.Contains(t, name, "en-US")

		// 树：单个 staticForm 节点、单常量字段、带 enum 选项
		var tree []map[string]interface{}
		require.NoError(t, json.Unmarshal(tpl.Tree, &tree))
		require.Len(t, tree, 1)
		assert.Equal(t, "staticForm", tree[0]["type"])
		props := tree[0]["props"].(map[string]interface{})
		var schema struct {
			Properties map[string]struct {
				Enum []string `json:"enum"`
			} `json:"properties"`
		}
		require.NoError(t, json.Unmarshal([]byte(props["staticSchema"].(string)), &schema))
		require.Len(t, schema.Properties, 1, "一种常量一个字段")
		for _, p := range schema.Properties {
			assert.NotEmpty(t, p.Enum)
		}
	}
	// 覆盖四类游戏常量
	for _, want := range []string{
		"consts--demo-ban-reason",
		"consts--demo-vip-level",
		"consts--demo-server-status",
		"consts--demo-pay-channel",
	} {
		assert.True(t, keys[want], "缺少示例 %s", want)
	}
}

// ---- HTTP 幂等种子 ----

func newSeedDemoRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewHandler(model.NewComponentTemplateModel(setupV4DB(t)), nil)
	r := gin.New()
	h.Register(r.Group("/api/v1/component-templates"))
	return r
}

func TestSeedDemoConstantsIdempotent(t *testing.T) {
	r := newSeedDemoRouter(t)

	// 第一次：创建 4 个
	w := doReq(r, "POST", "/api/v1/component-templates/seed-demo-constants", "")
	require.Equal(t, 200, w.Code)
	var first struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &first))
	assert.Equal(t, 4, first.Created)
	assert.Equal(t, 0, first.Skipped)

	// 第二次：全部跳过（幂等，不产生重复）
	w = doReq(r, "POST", "/api/v1/component-templates/seed-demo-constants", "")
	require.Equal(t, 200, w.Code)
	var second struct {
		Created int `json:"created"`
		Skipped int `json:"skipped"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &second))
	assert.Equal(t, 0, second.Created)
	assert.Equal(t, 4, second.Skipped)

	// 列表可见，且示例可删除（非 builtin）
	w = doReq(r, "GET", "/api/v1/component-templates?category=常量", "")
	require.Equal(t, 200, w.Code)
	var list struct {
		Items []TemplateDTO `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Items, 4)
	for _, item := range list.Items {
		assert.False(t, item.Builtin)
	}
}
