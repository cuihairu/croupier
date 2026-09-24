package approval

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

// 单公司 scope 模型：审批列表必须按 (game_id, env) 过滤，跨 scope 条目不得泄漏。

func TestFilterApprovalItemsByScope(t *testing.T) {
	items := []Approval{
		{ID: "a1", GameID: "g1", Env: "prod"},
		{ID: "a2", GameID: "g2", Env: "prod"},
		{ID: "a3", GameID: "g1", Env: "dev"},
		{ID: "a4", GameID: "G1", Env: "PROD"}, // 大小写不敏感匹配
	}

	// 空 scope → 全集。
	all := filterApprovalItemsByScope(items, svc.GameScope{})
	assert.Len(t, all, 4)

	// 精确 scope → 只保留匹配（含大小写变体），跨 game/env 丢弃。
	filtered := filterApprovalItemsByScope(items, svc.GameScope{GameID: "g1", Env: "prod"})
	require.Len(t, filtered, 2)
	ids := []string{filtered[0].ID, filtered[1].ID}
	assert.Equal(t, []string{"a1", "a4"}, ids)

	// 无匹配 → 空切片（非 nil 语义由 make 保证）。
	none := filterApprovalItemsByScope(items, svc.GameScope{GameID: "ghost", Env: "x"})
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

func TestParseApprovalPublishedSnapshot_BadJSONDegraded(t *testing.T) {
	// 空字段 → 零值，不 panic。
	page, contracts := parseApprovalPublishedSnapshot(model.PublishedPageSpec{})
	assert.Empty(t, page.PageKey)
	assert.Nil(t, contracts)

	// 损坏的 SpecJSON：json.Unmarshal 错误被吞（`_ =`），返回零值 PageSpec。
	page, contracts = parseApprovalPublishedSnapshot(model.PublishedPageSpec{
		SpecJSON:             `{broken`,
		BindingContractsJSON: `also-broken`,
	})
	assert.Empty(t, page.PageKey)
	assert.Nil(t, contracts)

	// 合法 payload 正常解出。
	page, contracts = parseApprovalPublishedSnapshot(model.PublishedPageSpec{
		SpecJSON:             `{"pageKey":"ops.ban","type":"operation"}`,
		BindingContractsJSON: `[{"bindingId":"b1","functionId":"player.ban"}]`,
	})
	assert.Equal(t, "ops.ban", page.PageKey)
	require.Len(t, contracts, 1)
	assert.Equal(t, "player.ban", contracts[0].FunctionID)
}

func TestApprovalBindingFreshnessStatuses_JoinsNonEmpty(t *testing.T) {
	assert.Empty(t, approvalBindingFreshnessStatuses(nil))
	assert.Empty(t, approvalBindingFreshnessStatuses([]spec.BindingFreshnessDiagnostic{}))

	assert.Equal(t, "binding_stale,input_schema_stale",
		approvalBindingFreshnessStatuses([]spec.BindingFreshnessDiagnostic{
			{Status: "binding_stale"},
			{Status: ""},
			{Status: "input_schema_stale"},
		}))
}

func TestFormatReviewedAt(t *testing.T) {
	assert.Empty(t, formatReviewedAt(nil))
	zero := time.Time{}
	assert.Empty(t, formatReviewedAt(&zero))

	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	got := formatReviewedAt(&at)
	assert.NotEmpty(t, got)
	assert.Contains(t, got, "2026")
}
