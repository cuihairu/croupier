package console

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
)

// 发布绑定执行边界：unbound 契约 409、stale binding 409、空 permission 放行。

func TestEnsureExecutorBound(t *testing.T) {
	s := &Service{}
	binding := spec.PageFunctionBinding{ID: "b1", FunctionID: "player.ban"}

	// 函数不在 functions 表 → 不在此重复 missing（由 ensureBindingFresh 表达）。
	assert.NoError(t, s.ensureExecutorBound(binding, map[string]spec.FunctionSpec{}))

	// bound / 空 executionState（Normalize 归一为 bound）→ 放行。
	assert.NoError(t, s.ensureExecutorBound(binding, map[string]spec.FunctionSpec{
		"player.ban": {ID: "player.ban", ExecutionState: spec.ExecutionStateBound},
	}))
	assert.NoError(t, s.ensureExecutorBound(binding, map[string]spec.FunctionSpec{
		"player.ban": {ID: "player.ban"},
	}))

	// unbound → 409 executor_unbound。
	err := s.ensureExecutorBound(binding, map[string]spec.FunctionSpec{
		"player.ban": {ID: "player.ban", ExecutionState: spec.ExecutionStateUnbound},
	})
	require.Error(t, err)
	var codeErr interface{ ErrorCode() string }
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, "executor_unbound", codeErr.ErrorCode())
}

func TestEnsureBindingFresh_FunctionMissing(t *testing.T) {
	s := &Service{}
	binding := spec.PageFunctionBinding{ID: "b1", FunctionID: "ghost.fn"}
	contract := spec.BindingContractSnapshot{BindingID: "b1", FunctionID: "ghost.fn"}

	err := s.ensureBindingFresh(binding, contract, map[string]spec.FunctionSpec{})
	require.Error(t, err)
	var codeErr interface{ ErrorCode() string }
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, "binding_stale", codeErr.ErrorCode())
}

func TestEnsureBindingFresh_VersionStale(t *testing.T) {
	s := &Service{}
	binding := spec.PageFunctionBinding{
		ID:         "b1",
		FunctionID: "player.ban",
		Execution:  spec.PageBindingExecution{Mode: spec.PageExecutionModeSync},
	}
	contract := spec.BindingContractSnapshot{
		BindingID:       "b1",
		FunctionID:      "player.ban",
		FunctionVersion: "1.0.0",
		ExecutionMode:   spec.PageExecutionModeSync,
	}
	functions := map[string]spec.FunctionSpec{
		"player.ban": {ID: "player.ban", Version: "2.0.0"},
	}
	err := s.ensureBindingFresh(binding, contract, functions)
	require.Error(t, err)
	var codeErr interface{ ErrorCode() string }
	require.ErrorAs(t, err, &codeErr)
	assert.Equal(t, "binding_stale", codeErr.ErrorCode())

	// 版本一致且其余治理字段均对齐 → 放行。
	contract.FunctionVersion = "2.0.0"
	assert.NoError(t, s.ensureBindingFresh(binding, contract, functions))
}

func TestEnforcePublishedBindingGovernance_EmptyPermissionAllows(t *testing.T) {
	// permission 为空 → 直接放行，不触碰 svcCtx（nil 安全）。
	s := &Service{}
	err := s.enforcePublishedBindingGovernance(context.Background(),
		spec.BindingContractSnapshot{Permission: "  "})
	assert.NoError(t, err)
}

func TestPublishedBindingExecutionMetadata_StripsEmptyAndAddsBaseVersion(t *testing.T) {
	page := spec.PublishedPageSpec{
		PageSpec: spec.PageSpec{
			PageKey:  "ops.ban",
			Category: spec.PageCategorySpec{Key: "ops"},
			// FunctionDigest 空 → 整 key 被剔除。
		},
		Version:               3,
		RendererSchemaVersion: "v1",
		BaseProposalKey:       "prop-1",
		BaseProposalVersion:   7,
	}
	binding := spec.PageFunctionBinding{ID: "b1", FunctionID: "player.ban"}
	contract := spec.BindingContractSnapshot{
		BindingID:       "b1",
		FunctionID:      "player.ban",
		FunctionVersion: "1.2.0",
		ExecutionMode:   "sync",
		Risk:            "warning",
		Approval:        spec.ApprovalPolicy{Required: true, PolicyKey: "two-person"},
	}

	md := publishedBindingExecutionMetadata(page, binding, contract, "req-1")
	assert.Equal(t, "ops.ban", md["page_key"])
	assert.Equal(t, "3", md["publish_version"])
	assert.Equal(t, "req-1", md["page_request_id"])
	assert.Equal(t, "7", md["base_proposal_version"])
	assert.Equal(t, "true", md["snapshot_approval"])
	assert.Equal(t, "two-person", md["snapshot_approval_policy"])
	assert.Equal(t, "ops", md["page_category"])
	assert.Equal(t, "v1", md["renderer_schema_version"])
	assert.NotContains(t, md, "function_digest", "空值 key 必须剔除")

	// BaseProposalVersion=0 → 不写 base_proposal_version。
	page.BaseProposalVersion = 0
	md = publishedBindingExecutionMetadata(page, binding, contract, "req-2")
	assert.NotContains(t, md, "base_proposal_version")
}

func TestIsNullRawJSON(t *testing.T) {
	assert.True(t, isNullRawJSON(nil))
	assert.True(t, isNullRawJSON(json.RawMessage(`null`)))
	assert.True(t, isNullRawJSON(json.RawMessage("  null \n")))
	assert.False(t, isNullRawJSON(json.RawMessage(`{}`)))
	assert.False(t, isNullRawJSON(json.RawMessage(`0`)))
}

func TestParsePublishedPages_SkipsInvalidRows(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	models := []model.PublishedPageSpec{
		{}, // SpecJSON 空 → unmarshal 失败或 pageKey 空 → 跳过
		{SpecJSON: `{broken`},
		{SpecJSON: `{"type":"operation"}`}, // pageKey 空 → 跳过
		{
			GameID:                "g1",
			Env:                   "prod",
			PageKey:               "ops.ban",
			Version:               2,
			SpecJSON:              `{"pageKey":"ops.ban","type":"operation","bindings":[]}`,
			BindingContractsJSON:  `[{"bindingId":"b1","functionId":"player.ban"}]`,
			RendererSchemaVersion: "v1",
			BaseProposalKey:       "prop-1",
			BaseProposalVersion:   7,
			PublishedAt:           now,
			PublishedBy:           "alice",
		},
	}
	pages := parsePublishedPages(models)
	require.Len(t, pages, 1)
	assert.Equal(t, "ops.ban", pages[0].PageKey)
	assert.Equal(t, "g1", pages[0].GameID)
	assert.Equal(t, 2, pages[0].Version)
	require.Len(t, pages[0].BindingContracts, 1)
	assert.Equal(t, "player.ban", pages[0].BindingContracts[0].FunctionID)
}
