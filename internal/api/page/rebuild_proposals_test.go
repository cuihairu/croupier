package page

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	contractsvc "github.com/cuihairu/croupier/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormDatatypes "gorm.io/datatypes"
)

// TestRebuildAllProposals_RegeneratesStaleGeneratorVersion verifies the P5
// cleanup path: a proposal written by an older generator (page-generator:1,
// with Title Case fallback labels) must compare as changed against the
// current generator version and be regenerated with raw-key labels. Published
// pages are intentionally not touched by the rebuild.
func TestRebuildAllProposals_RegeneratesStaleGeneratorVersion(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)

	const (
		gameID = "demo-game"
		env    = "development"
		key    = "resource:player"
	)

	proposalModel := model.NewPageProposalModel(service.svcCtx.DB)
	stale, err := proposalModel.FindByScopeAndKey(ctx, gameID, env, key)
	require.NoError(t, err, "seeded resource:player proposal must exist")
	require.Equal(t, contractsvc.PageProposalGeneratorVersion, stale.GeneratorVersion)

	// Downgrade the proposal to simulate a stale generator artifact with
	// Title Case fallback labels baked in.
	staleSpec := map[string]interface{}{
		"columns": []map[string]interface{}{{
			"key":   "createdAt",
			"title": map[string]string{"zh-CN": "Created At", "en-US": "Created At"},
		}},
	}
	specJSON, err := json.Marshal(staleSpec)
	require.NoError(t, err)
	stale.GeneratorVersion = "page-generator:1"
	stale.Title = gormDatatypes.JSONMap{"zh-CN": "Player", "en-US": "Player"}
	stale.PageSpec = model.JSON(specJSON)
	require.NoError(t, proposalModel.UpsertProposal(ctx, stale))

	resp, err := service.RebuildAllProposals(ctx)
	require.NoError(t, err)
	assert.Equal(t, gameID, resp.GameID)
	assert.Equal(t, env, resp.Env)

	updated, err := proposalModel.FindByScopeAndKey(ctx, gameID, env, key)
	require.NoError(t, err)
	assert.Equal(t, contractsvc.PageProposalGeneratorVersion, updated.GeneratorVersion)
	assert.NotContains(t, string(updated.PageSpec), "Created At",
		"regenerated spec must not keep Title Case fallback labels")
}

// TestRebuildAllProposals_UpToDateProposalsStayUntouched ensures idempotency:
// rebuilding when every proposal already matches the current generator must
// not rewrite rows (no churn, no new snapshots).
func TestRebuildAllProposals_UpToDateProposalsStayUntouched(t *testing.T) {
	service, ctx, _ := newPageTestService(t, "pages:edit")
	seedResourceProposal(t, service)

	proposalModel := model.NewPageProposalModel(service.svcCtx.DB)
	before, err := proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)

	resp, err := service.RebuildAllProposals(ctx)
	require.NoError(t, err)
	assert.Equal(t, "demo-game", resp.GameID)

	after, err := proposalModel.FindByScopeAndKey(ctx, "demo-game", "development", "resource:player")
	require.NoError(t, err)
	assert.Equal(t, before.GeneratorVersion, after.GeneratorVersion)
	assert.Equal(t, strings.TrimSpace(before.FunctionDigest), strings.TrimSpace(after.FunctionDigest))
}

// seedResourceProposal registers a collection_query function through the real
// pipeline and rebuilds the player resource so a resource:player proposal
// exists.
func seedResourceProposal(t *testing.T, service *Service) {
	t.Helper()
	db := service.svcCtx.DB
	ctx := context.Background()
	rebuildFunctionContract(t, db, ctx, "player.query", reg.FunctionMeta{
		Enabled:      true,
		Version:      "1.0.0",
		Risk:         "safe",
		Permission:   "player:query",
		Resource:     "player",
		Operation:    "list",
		Capability:   "collection_query",
		InputSchema:  `{"type":"object","properties":{"page":{"type":"integer"}}}`,
		OutputSchema: `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"createdAt":{"type":"string"}}}},"total":{"type":"number"}}}`,
	})
	contractSvc := contractsvc.NewContractService(db)
	require.NoError(t, contractSvc.RebuildResourceCapability(ctx, "demo-game", "development", "player"))
	require.NoError(t, contractSvc.RebuildProposalsForResource(ctx, "demo-game", "development", "player"))
}
