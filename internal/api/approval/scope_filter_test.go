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

// Single-company scope model: the approval list must be filtered by
// (game_id, env); entries from other scopes must not leak through.

func TestFilterApprovalItemsByScope(t *testing.T) {
	items := []Approval{
		{ID: "a1", GameID: "g1", Env: "prod"},
		{ID: "a2", GameID: "g2", Env: "prod"},
		{ID: "a3", GameID: "g1", Env: "dev"},
		{ID: "a4", GameID: "G1", Env: "PROD"}, // case-insensitive match
	}

	// Empty scope → the full set.
	all := filterApprovalItemsByScope(items, svc.GameScope{})
	assert.Len(t, all, 4)

	// Exact scope → keep only matches (including case variants), drop other
	// game/env combinations.
	filtered := filterApprovalItemsByScope(items, svc.GameScope{GameID: "g1", Env: "prod"})
	require.Len(t, filtered, 2)
	ids := []string{filtered[0].ID, filtered[1].ID}
	assert.Equal(t, []string{"a1", "a4"}, ids)

	// No match → empty slice (the non-nil semantics are guaranteed by make).
	none := filterApprovalItemsByScope(items, svc.GameScope{GameID: "ghost", Env: "x"})
	assert.NotNil(t, none)
	assert.Empty(t, none)
}

func TestParseApprovalPublishedSnapshot_BadJSONDegraded(t *testing.T) {
	// Empty fields → zero values, no panic.
	page, contracts := parseApprovalPublishedSnapshot(model.PublishedPageSpec{})
	assert.Empty(t, page.PageKey)
	assert.Nil(t, contracts)

	// Corrupt SpecJSON: the json.Unmarshal error is swallowed (`_ =`), so a
	// zero-value PageSpec is returned.
	page, contracts = parseApprovalPublishedSnapshot(model.PublishedPageSpec{
		SpecJSON:             `{broken`,
		BindingContractsJSON: `also-broken`,
	})
	assert.Empty(t, page.PageKey)
	assert.Nil(t, contracts)

	// A valid payload unmarshals normally.
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
