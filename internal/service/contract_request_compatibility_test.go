package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SUI-011: the request compatibility findings the upload path can see must
// reach the stored contract, because the projected input schema has already
// flattened the parameters and dropped the media types behind them. Both the
// unbound upload path and the bound registration path go through the same
// merge.

func sui011RequestDiagnostic() spec.Diagnostic {
	return spec.Diagnostic{
		Code:       spec.DiagSchemaCompatibilityManual,
		Severity:   spec.SeverityWarning,
		FunctionID: "player.get",
		Field:      "$.body.content",
		Message:    "request body declares no application/json media type; no configured executor path delivers those media types from a generated page",
	}
}

func TestCreateUnboundContractStoresRequestCompatibilityDiagnostics(t *testing.T) {
	db := setupTestDBFileV9(t)
	svc := NewContractService(db)
	ctx := context.Background()
	input := t2ContractInput("player.get")
	input.RequestDiagnostics = []spec.Diagnostic{sui011RequestDiagnostic()}

	created, err := svc.CreateUnboundContract(ctx, "g-sui011", "e-sui011", "openapi", input)
	require.NoError(t, err)
	require.True(t, created)

	stored, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "g-sui011", "e-sui011", "player.get")
	require.NoError(t, err)
	assert.Equal(t, string(spec.ExecutionStateUnbound), stored.ExecutionState)

	var diagnostics []spec.Diagnostic
	require.NoError(t, json.Unmarshal(stored.Diagnostics, &diagnostics))
	assert.Contains(t, diagnostics, sui011RequestDiagnostic())
}

func TestRebuildContractMergesRequestCompatibilityDiagnostics(t *testing.T) {
	db := setupTestDBFileV9(t)
	svc := NewContractService(db)
	ctx := context.Background()
	input := t2ContractInput("player.get")
	input.RequestDiagnostics = []spec.Diagnostic{sui011RequestDiagnostic()}

	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-sui011b", "e-sui011b", "agent-1", input))

	stored, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "g-sui011b", "e-sui011b", "player.get")
	require.NoError(t, err)

	var diagnostics []spec.Diagnostic
	require.NoError(t, json.Unmarshal(stored.Diagnostics, &diagnostics))
	assert.Contains(t, diagnostics, sui011RequestDiagnostic(),
		"the bound registration path must keep the source-side findings too")
}

func TestRebuildContractDedupesRequestCompatibilityDiagnostics(t *testing.T) {
	db := setupTestDBFileV9(t)
	svc := NewContractService(db)
	ctx := context.Background()
	input := t2ContractInput("player.get")
	input.RequestDiagnostics = []spec.Diagnostic{sui011RequestDiagnostic(), sui011RequestDiagnostic()}

	require.NoError(t, svc.RebuildContractFromFunctionMeta(ctx, "g-sui011c", "e-sui011c", "agent-1", input))

	stored, err := model.NewFunctionContractModel(db).FindByScopeAndFunctionID(ctx, "g-sui011c", "e-sui011c", "player.get")
	require.NoError(t, err)

	var diagnostics []spec.Diagnostic
	require.NoError(t, json.Unmarshal(stored.Diagnostics, &diagnostics))
	count := 0
	for _, diagnostic := range diagnostics {
		if diagnostic == sui011RequestDiagnostic() {
			count++
		}
	}
	assert.Equal(t, 1, count, "identical findings must not accumulate on re-registration")
}
