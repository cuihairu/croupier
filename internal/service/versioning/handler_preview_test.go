package versioning

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
// 缺契约时返回 400 + validation_failed + details，前端可把原因定位到
// sections[1].functionId 对应的那一个区块，而不是只看到一条笼统报错。
func TestV9Handler_PreviewCompositePage_ContractMissing(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)
	seedCompositeContractsV9(t, db)

	rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite/preview",
		`{"pageKey":"composite--missing-fn","sections":[`+
			`{"functionId":"player.get","view":"fields"},`+
			`{"functionId":"no.such.fn","view":"table"}]}`)
	details := requireValidationErrorBody(t, rec)
	assert.Contains(t, details["sections[1].functionId"], "no.such.fn contract not found")
}

// TestV9Handler_PreviewCompositePage_PageKeyRequired：缺 pageKey → 400 +
// details.pageKey，原因可指到具体输入项。
func TestV9Handler_PreviewCompositePage_PageKeyRequired(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)
	seedCompositeContractsV9(t, db)

	details := requireValidationErrorBody(t, doVersioningRequestV9(router, http.MethodPost,
		"/api/versioning/pages/composite/preview",
		`{"sections":[{"functionId":"player.get","view":"fields"}]}`))
	assert.Contains(t, details["pageKey"], "pageKey is required")
}

// TestV9Handler_PreviewCompositePage_DuplicateSectionKey：区块 key 冲突时，
// details 键带上发生冲突的区块下标，编辑者可定位到具体区块。
func TestV9Handler_PreviewCompositePage_DuplicateSectionKey(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)
	seedCompositeContractsV9(t, db)

	details := requireValidationErrorBody(t, doVersioningRequestV9(router, http.MethodPost,
		"/api/versioning/pages/composite/preview",
		`{"pageKey":"composite--dup-key","sections":[`+
			`{"key":"k1","functionId":"player.get","view":"fields"},`+
			`{"key":"k1","functionId":"order.list","view":"table"}]}`))
	assert.Contains(t, details["sections[1].key"], `duplicate section key "k1"`)
}

// TestV9Handler_PreviewCompositePage_StaticSectionValidation：static 区块的
// key 缺失与 form.jsonSchema 缺失分别落到对应字段路径。
func TestV9Handler_PreviewCompositePage_StaticSectionValidation(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)

	t.Run("MissingKey", func(t *testing.T) {
		details := requireValidationErrorBody(t, doVersioningRequestV9(router, http.MethodPost,
			"/api/versioning/pages/composite/preview",
			`{"pageKey":"composite--static-no-key","sections":[`+
				`{"static":true,"form":{"jsonSchema":{"type":"object","properties":{"text":{"type":"string"}}}}}]}`))
		assert.Contains(t, details["sections[0].key"], "static section requires key")
	})

	t.Run("MissingForm", func(t *testing.T) {
		details := requireValidationErrorBody(t, doVersioningRequestV9(router, http.MethodPost,
			"/api/versioning/pages/composite/preview",
			`{"pageKey":"composite--static-no-form","sections":[`+
				`{"static":true,"key":"note","title":"说明"}]}`))
		assert.Contains(t, details["sections[0].form.jsonSchema"], "form.jsonSchema")
	})
}

// TestV9Handler_PreviewCompositePage_NoFunctionalSection：全 static 区块无契约可
// 生成（生成器空输入流拒绝），回退原因落在 sections 而非编造字段路径。
func TestV9Handler_PreviewCompositePage_NoFunctionalSection(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)

	details := requireValidationErrorBody(t, doVersioningRequestV9(router, http.MethodPost,
		"/api/versioning/pages/composite/preview",
		`{"pageKey":"composite--static-only","sections":[`+
			`{"static":true,"key":"note","title":"说明",`+
			`"form":{"jsonSchema":{"type":"object","properties":{"text":{"type":"string"}}}}}]}`))
	assert.Contains(t, details["sections"], "no functional section")
}

// TestV9Handler_PreviewCompositePage_InvalidBody：坏 JSON 按 400 返回。
func TestV9Handler_PreviewCompositePage_InvalidBody(t *testing.T) {
	db := setupTestDB(t)
	router := newVersioningHandlerRouterV9(t, db)

	rec := doVersioningRequestV9(router, http.MethodPost, "/api/versioning/pages/composite/preview", `{bad`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// requireValidationErrorBody 断言统一验证错误形状（400 + error=validation_failed
// + 非空结构化 details）并返回 details，供字段路径级断言复用。
func requireValidationErrorBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var body struct {
		Error   string         `json:"error"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "validation_failed", body.Error)
	assert.NotEmpty(t, body.Message)
	require.NotEmpty(t, body.Details, "校验失败须返回可内联展示的字段级 details")
	return body.Details
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
