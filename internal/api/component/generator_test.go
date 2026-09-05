package component

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dbenum"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
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

// stale 检测：契约变化后 List 标记 builtin 模板过期；重新生成后消除。
func TestListMarksStaleBuiltinTemplates(t *testing.T) {
	db := setupV4DB(t)
	contract := &model.FunctionContract{
		GameID: "demo", Env: "dev", FunctionID: "player.list", ResourceKey: "player",
		Capability:  dbenum.CapabilityCollectionQuery,
		InputSchema: model.JSON(`{"type":"object","properties":{"kw":{"type":"string"}}}`),
	}
	require.NoError(t, db.Create(contract).Error)

	h := NewHandler(model.NewComponentTemplateModel(db), contractMdlFromDB(t, db))
	ctx := context.WithValue(context.Background(), "X-Game-ID", "demo")
	ctx = context.WithValue(ctx, "X-Env", "dev")
	_ = ctx

	// 生成（存储与当前契约一致）
	require.NoError(t, h.GenerateSingleFunctionTemplates(context.Background(), []*model.FunctionContract{contract}))
	require.NoError(t, h.GenerateQueryTemplate(context.Background(), contract))

	// 契约未变化 → 不 stale
	stale := h.computeStaleKeys("demo", "dev", []model.ComponentTemplate{*storedBuiltin(t, db, "fn--player.list")})
	assert.Empty(t, stale, "fresh template must not be stale")

	// 契约能力变化（collection_query → action，落库）：
	// fn--/query-- 两个模板的生成条件消失 → stale
	contract.Capability = dbenum.CapabilityAction
	require.NoError(t, db.Model(contract).Update("capability", contract.Capability).Error)
	items := []model.ComponentTemplate{
		*storedBuiltin(t, db, "fn--player.list"),
		*storedBuiltin(t, db, "query--player.list"),
	}
	stale = h.computeStaleKeys("demo", "dev", items)
	assert.True(t, stale["fn--player.list"], "视图推导变化必须标记 stale")
	assert.True(t, stale["query--player.list"], "生成条件消失必须标记 stale")

	// 自定义模板不参与 stale 检测
	assert.False(t, stale["custom--user"], "custom 模板永不自动标记")
}

func contractMdlFromDB(t *testing.T, db *gorm.DB) *model.FunctionContractModel {
	t.Helper()
	return model.NewFunctionContractModel(db)
}

func storedBuiltin(t *testing.T, db *gorm.DB, key string) *model.ComponentTemplate {
	t.Helper()
	var tpl model.ComponentTemplate
	require.NoError(t, db.Where("key = ?", key).First(&tpl).Error)
	return &tpl
}
