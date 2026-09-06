// 统一校验规则回归：提案创建即运行发布级 selector 校验（单一规则源
// CollectBindingSelectorIssues）——违规前置为 error 诊断并降级 needs_review，
// 不再「保存看似可发布、accept-and-publish 才 422」。
package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func diagnosticsByCode(proposalDiagnostics []byte) map[string]spec.Diagnostic {
	out := map[string]spec.Diagnostic{}
	var diags []spec.Diagnostic
	if json.Unmarshal(proposalDiagnostics, &diags) != nil {
		return out
	}
	for _, d := range diags {
		if d.Code == "publish_validation_failed" {
			out[d.Field] = d
		}
	}
	return out
}

// 必填参数被显式映射为非法 source kind → 统一校验在保存期即失败：
// 质量降级 needs_review + publish_validation_failed 诊断（发布前可见）。
func TestCreateCompositeProposalPublishValidationDowngradesQuality(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)
	seedCompositeV9(t, svc, ctx)

	proposal, err := svc.CreateCompositeProposal(ctx, "g9", "e9", "composite--unified-check", []CompositeSectionRequest{
		{FunctionID: "player.get", View: "fields", Title: "玩家信息"},
		{
			FunctionID: "order.list", View: "table", Title: "订单",
			// row 来源在无 detail 视图的 query 区块上下文中不合法 →
			// ValidateSelector: "source kind not allowed in this context"
			InputAssignments: []CompositeInputAssignmentRequest{{
				Target: "/playerId", Kind: "row", Path: "/playerId",
			}},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, proposal)

	assert.Equal(t, string(spec.GeneratedPageQualityNeedsReview), proposal.Quality,
		"发布级违规必须降级为 needs_review（不可标记 ready/basic）")
	issues := diagnosticsByCode(proposal.Diagnostics)
	require.NotEmpty(t, issues, "必须产出 publish_validation_failed 诊断")
	foundInvalidSource := false
	for _, d := range issues {
		if d.Message == "source kind not allowed in this context" {
			foundInvalidSource = true
		}
	}
	assert.True(t, foundInvalidSource, "诊断必须包含 source kind 违规，实际: %v", issues)
}

// 显式参数映射补齐必填参数 → 不产生违规、质量不被降级（同一规则源通过）。
func TestCreateCompositeProposalExplicitAssignmentPassesUnifiedCheck(t *testing.T) {
	db := setupTestDBFileV9(t)
	ctx := context.Background()
	svc := NewContractService(db)
	seedCompositeV9(t, svc, ctx)

	proposal, err := svc.CreateCompositeProposal(ctx, "g9", "e9", "composite--unified-ok", []CompositeSectionRequest{
		{FunctionID: "player.get", View: "fields", Title: "玩家信息"},
		{
			FunctionID: "order.list", View: "table", Title: "订单",
			InputAssignments: []CompositeInputAssignmentRequest{{
				Target: "/playerId", Kind: "literal", Value: []byte(`"demo-player"`),
			}},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, proposal)

	assert.Empty(t, diagnosticsByCode(proposal.Diagnostics), "显式映射补齐后不应有发布级违规")
	assert.NotEqual(t, string(spec.GeneratedPageQualityNeedsReview), proposal.Quality,
		"通过统一校验的提案不应被降级")
}

// 单一规则源本身：disabled 函数同样在提案创建期暴露。
func TestCollectBindingSelectorIssuesDisabledFunction(t *testing.T) {
	page := spec.PageSpec{
		PageKey: "p", Type: spec.PageTypeOperation,
		Bindings: []spec.PageFunctionBinding{{FunctionID: "off.fn", Selectors: nil}},
	}
	issues := CollectBindingSelectorIssues(FunctionSpecsFromContracts(nil), page)
	assert.Equal(t, "bound function contract does not exist", issues["bindings[0].functionId"])
}
