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

// Published binding execution boundaries: an unbound contract yields 409, a stale
// binding yields 409, and an empty permission is allowed through.

func TestEnsureExecutorBound(t *testing.T) {
	s := &Service{}
	binding := spec.PageFunctionBinding{ID: "b1", FunctionID: "player.ban"}

	// A function missing from the functions table -> the missing case is not reported
	// here (ensureBindingFresh expresses it).
	assert.NoError(t, s.ensureExecutorBound(binding, map[string]spec.FunctionSpec{}))

	// bound / an empty executionState (Normalize canonicalizes it to bound) -> allowed.
	assert.NoError(t, s.ensureExecutorBound(binding, map[string]spec.FunctionSpec{
		"player.ban": {ID: "player.ban", ExecutionState: spec.ExecutionStateBound},
	}))
	assert.NoError(t, s.ensureExecutorBound(binding, map[string]spec.FunctionSpec{
		"player.ban": {ID: "player.ban"},
	}))

	// unbound -> 409 executor_unbound.
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

	// Version matches and all remaining governance fields align -> allowed.
	contract.FunctionVersion = "2.0.0"
	assert.NoError(t, s.ensureBindingFresh(binding, contract, functions))
}

func TestEnforcePublishedBindingGovernance_EmptyPermissionAllows(t *testing.T) {
	// An empty permission -> allowed straight through without touching svcCtx
	// (nil safe).
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
			// FunctionDigest empty -> the whole key is stripped.
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

	md := bindingExecutionMetadata(page, binding, contract, "req-1", governanceValidated, runtimeAPIPublishedExecute)
	assert.Equal(t, "ops.ban", md["page_key"])
	assert.Equal(t, "3", md["publish_version"])
	assert.Equal(t, "req-1", md["page_request_id"])
	assert.Equal(t, "7", md["base_proposal_version"])
	assert.Equal(t, "true", md["snapshot_approval"])
	assert.Equal(t, "two-person", md["snapshot_approval_policy"])
	assert.Equal(t, "ops", md["page_category"])
	assert.Equal(t, "v1", md["renderer_schema_version"])
	assert.NotContains(t, md, "function_digest", "空值 key 必须剔除")

	// BaseProposalVersion=0 -> base_proposal_version is not written.
	page.BaseProposalVersion = 0
	md = bindingExecutionMetadata(page, binding, contract, "req-2", governanceValidated, runtimeAPIPublishedExecute)
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
		{}, // SpecJSON empty -> unmarshal fails or pageKey empty -> skipped
		{SpecJSON: `{broken`},
		{SpecJSON: `{"type":"operation"}`}, // empty pageKey -> skipped
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
