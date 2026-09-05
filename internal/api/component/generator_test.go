package component

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func queryContract(fid string, properties int) *model.FunctionContract {
	props := map[string]json.RawMessage{}
	for i := 0; i < properties; i++ {
		props[string(rune('a'+i))] = json.RawMessage(`{"type":"string"}`)
	}
	raw, _ := json.Marshal(map[string]interface{}{"type": "object", "properties": props})
	return &model.FunctionContract{
		FunctionID:  fid,
		ResourceKey: "player",
		Capability:  dbenum.CapabilityCollectionQuery,
		InputSchema: raw,
		Enabled:     true,
	}
}

func TestGenerateQueryTemplate(t *testing.T) {
	h := NewHandler(model.NewComponentTemplateModel(setupV4DB(t)), nil)
	c := queryContract("player.list", 2)
	require.NoError(t, h.GenerateQueryTemplate(context.Background(), c))

	tpl, err := h.model.FindByKey(context.Background(), "query--player.list")
	require.NoError(t, err)
	assert.Equal(t, "查询组合", tpl.Category)
	assert.True(t, tpl.Builtin)

	var nodes []struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Props struct {
			AutoRun       bool     `json:"autoRun"`
			RefreshOnNode []string `json:"refreshOnNode"`
		} `json:"props"`
	}
	require.NoError(t, json.Unmarshal(tpl.Tree, &nodes))
	require.Len(t, nodes, 2)
	assert.Equal(t, "fnForm", nodes[0].Type)
	assert.False(t, nodes[0].Props.AutoRun, "查询表单不应 autoRun")
	assert.Equal(t, "fnTable", nodes[1].Type)
	assert.Equal(t, []string{"qform"}, nodes[1].Props.RefreshOnNode, "表格 refreshOn 引用表单节点")
}

func TestGenerateQueryTemplateSkipsWithoutParams(t *testing.T) {
	h := NewHandler(model.NewComponentTemplateModel(setupV4DB(t)), nil)
	c := queryContract("player.list", 0)
	require.NoError(t, h.GenerateQueryTemplate(context.Background(), c))
	_, err := h.model.FindByKey(context.Background(), "query--player.list")
	assert.Error(t, err, "无查询参数时不生成查询组合模板")
}

func TestGenerateQueryTemplateSkipsNonCollection(t *testing.T) {
	h := NewHandler(model.NewComponentTemplateModel(setupV4DB(t)), nil)
	c := queryContract("player.get", 2)
	c.Capability = dbenum.CapabilityItemQuery
	require.NoError(t, h.GenerateQueryTemplate(context.Background(), c))
	_, err := h.model.FindByKey(context.Background(), "query--player.get")
	assert.Error(t, err)
}
