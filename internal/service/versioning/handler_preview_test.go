package versioning

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/model"
)

// previewBodyV9 预览与保存共用的最小有效组合页请求体：详情 + 表格两区块，
// 表格依赖详情的 page_state 联动（与生产提案用例同形态）。
func previewBodyV9(pageKey string) string {
	return `{"pageKey":"` + pageKey + `","sections":[` +
		`{"functionId":"player.get","view":"fields","title":"玩家信息"},` +
		`{"functionId":"order.list","view":"table","title":"订单","refreshOn":["player.get"]}` +
		`]}`
}

// TestV9Handler_PreviewCompositePage_Success：预览端点返回完整 GeneratedPageSpec
// ——区块结构、绑定分配、生成质量都由服务端生成链决定，客户端不再自行拼装。
func TestV9Handler_PreviewCompositePage_Success(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)
	seedCompositeContractsV9(t, db)

	rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite/preview",
		previewBodyV9("composite--preview"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload struct {
		PageKey  string          `json:"pageKey"`
		PageSpec json.RawMessage `json:"pageSpec"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Equal(t, "composite--preview", payload.PageKey)
	require.NotEmpty(t, payload.PageSpec)

	var specView struct {
		Type      string            `json:"type"`
		Quality   string            `json:"quality"`
		Bindings  []json.RawMessage `json:"bindings"`
		Composite struct {
			Sections []map[string]json.RawMessage `json:"sections"`
		} `json:"composite"`
	}
	require.NoError(t, json.Unmarshal(payload.PageSpec, &specView))
	assert.Equal(t, "composite", specView.Type)
	assert.NotEmpty(t, specView.Quality, "预览响应带生成质量，供预览栏就地展示")
	require.Len(t, specView.Composite.Sections, 2)
	require.NotEmpty(t, specView.Bindings, "预览产物必须带绑定分配")
}

// TestV9Handler_PreviewCompositePage_NoWrite：预览是纯只读链路——不产生提案行。
func TestV9Handler_PreviewCompositePage_NoWrite(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)
	seedCompositeContractsV9(t, db)

	rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite/preview",
		previewBodyV9("composite--preview-nowrite"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var n int64
	require.NoError(t, db.Model(&model.PageProposal{}).Count(&n).Error)
	assert.Zero(t, n, "预览不得写入提案")
}

// TestV9Handler_PreviewCompositePage_ContractMissing：预览继承生成链的契约校验，
// 缺契约时按统一错误形状返回，前端可直接展示 message。
func TestV9Handler_PreviewCompositePage_ContractMissing(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)
	seedCompositeContractsV9(t, db)

	rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite/preview",
		`{"pageKey":"composite--missing-fn","sections":[`+
			`{"functionId":"player.get","view":"fields"},`+
			`{"functionId":"no.such.fn","view":"table"}]}`)
	require.NotEqual(t, http.StatusOK, rec.Code)

	var errBody map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errBody))
	_, hasError := errBody["error"]
	assert.True(t, hasError, "错误响应使用统一 error/message 形状")
	assert.NotEmpty(t, errBody["message"])
}

// TestV9Handler_PreviewCompositePage_InvalidBody：坏 JSON 按 400 返回。
func TestV9Handler_PreviewCompositePage_InvalidBody(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)

	rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite/preview", `{bad`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestV9Handler_PreviewCompositePage_SameSpecAsSaved：同一请求体下，预览响应的
// PageSpec 与「保存」入口落库的 PageSpec 逐字段一致——编辑器预览消费的 spec 与
// 发布页来自同一份生成链，不存在「预览一套结构、发布另一套结构」的漂移。
func TestV9Handler_PreviewCompositePage_SameSpecAsSaved(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)
	seedCompositeContractsV9(t, db)

	body := previewBodyV9("composite--preview-eq")

	rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite/preview", body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var preview struct {
		PageSpec json.RawMessage `json:"pageSpec"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preview))
	require.NotEmpty(t, preview.PageSpec)

	rec2 := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite", body)
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())
	var created struct {
		ProposalKey string `json:"proposalKey"`
	}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &created))
	require.NotEmpty(t, created.ProposalKey)

	var proposal model.PageProposal
	require.NoError(t, db.Where("proposal_key = ?", created.ProposalKey).First(&proposal).Error)

	// 预览响应里的 pageSpec 是 GeneratedPageSpec（PageSpec + quality/diagnostics），
	// 提案落库的是 PageSpec 本体：剥离生成元信息后逐字段对拍。
	var previewMap map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(preview.PageSpec, &previewMap))
	delete(previewMap, "quality")
	delete(previewMap, "diagnostics")
	previewSpec, err := json.Marshal(previewMap)
	require.NoError(t, err)

	require.JSONEq(t, string(proposal.PageSpec), string(previewSpec))
}
