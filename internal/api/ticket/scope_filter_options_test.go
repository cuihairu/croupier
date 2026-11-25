package ticket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestList_ScopeHeaderOverridesQueryParams reproduces the core requirement
// of #21: after the page dropped the game/env filter, list ownership is
// decided by the top-bar scope (injected by GameDBMiddleware), and gameId/env
// in the request parameters are no longer used as query criteria.
func TestList_ScopeHeaderOverridesQueryParams(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	m := model.NewTicketModel(db)
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "A1", Category: "bug", Assignee: "alice", GameID: "game-a", Env: "prod"}))
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "A2", Category: "billing", Assignee: "bob", GameID: "game-a", Env: "prod"}))
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "B1", Category: "bug", Assignee: "carol", GameID: "game-b", Env: "dev"}))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets?gameId=game-b&env=dev", "")
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "game-a", Env: "prod"}))
	handler.List(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp ListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, int64(2), resp.Total)
	for _, item := range resp.Items {
		assert.Equal(t, "game-a", item.GameId)
	}
}

// TestCreate_StampScopeFromContext: a newly created ticket's ownership
// follows the request scope, and gameId/env in the body are no longer
// trusted (under a scoped group the middleware has already authenticated).
func TestCreate_StampScopeFromContext(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	ctx, rec := newTicketRequest(http.MethodPost, "/api/v1/tickets",
		`{"title":"S1","content":"c","category":"bug","gameId":"game-b","env":"dev"}`)
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "game-a", Env: "prod"}))
	handler.Create(ctx)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var created CreateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	assert.Equal(t, "game-a", created.GameId)
	assert.Equal(t, "prod", created.Env)
}

// TestFilterOptions_ServerAggregatesUnderScope: the category/assignee
// filter options are aggregated and provided by the server — the distinct
// full set plus counts, empties excluded, filtered by scope (#21 item 2).
func TestFilterOptions_ServerAggregatesUnderScope(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	m := model.NewTicketModel(db)
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T1", Category: "bug", Assignee: "alice", GameID: "game-a", Env: "prod"}))
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T2", Category: "bug", Assignee: "bob", GameID: "game-a", Env: "prod"}))
	// An empty category / empty assignee must not become a dropdown option
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T3", Category: "billing", GameID: "game-a", Env: "prod"}))
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T4", Category: "other-game", Assignee: "carol", GameID: "game-b", Env: "dev"}))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets/filter-options", "")
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "game-a", Env: "prod"}))
	handler.FilterOptions(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp FilterOptionsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, []TicketFilterOption{
		{Name: "billing", Count: 1},
		{Name: "bug", Count: 2},
	}, resp.Categories)
	assert.Equal(t, []TicketFilterOption{
		{Name: "alice", Count: 1},
		{Name: "bob", Count: 1},
	}, resp.Assignees)
}

// TestFilterOptions_AssigneesErrorSurfaced: when the category aggregation
// succeeds but the assignee aggregation fails, the second error branch of
// service.go (ListAssignees err → nil, err) must be propagated rather than
// returning a half-built options set. DropColumn("assignee") simulates the
// missing-column shape of a pre-existing old database: the category query is
// unaffected while the assignee query fails deterministically (isomorphic to
// the migration-misconfiguration incident).
func TestFilterOptions_AssigneesErrorSurfaced(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	m := model.NewTicketModel(db)
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T1", Category: "bug", Assignee: "alice", GameID: "game-a", Env: "prod"}))
	require.NoError(t, db.Migrator().DropColumn(&model.Ticket{}, "assignee"))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets/filter-options", "")
	handler.FilterOptions(ctx)

	require.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertTicketErrorShape(t, rec)
}

// TestFilterOptions_CategoriesErrorSurfaced: when the category aggregation
// fails, the first error branch of service.go (ListCategories err → nil, err)
// short-circuits and returns — the assignee aggregation must not execute, and
// the dropdown must not return hollow options (#21 item 2 contract: options
// are either real or an error is raised).
func TestFilterOptions_CategoriesErrorSurfaced(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	m := model.NewTicketModel(db)
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T1", Category: "bug", Assignee: "alice", GameID: "game-a", Env: "prod"}))
	require.NoError(t, db.Migrator().DropColumn(&model.Ticket{}, "category"))

	ctx, rec := newTicketRequest(http.MethodGet, "/api/v1/tickets/filter-options", "")
	handler.FilterOptions(ctx)

	require.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())
	assertTicketErrorShape(t, rec)
	assert.NotContains(t, rec.Body.String(), `"categories"`)
}

// TestFilterOptions_EmptyScopeStillReturnsShape: with no scope (the direct-
// caller compatibility path) the response still returns the full aggregation
// with empty arrays rather than null.
func TestFilterOptions_EmptyScopeStillReturnsShape(t *testing.T) {
	db := newTicketTestDB(t)
	handler := newTicketHandler(db)

	m := model.NewTicketModel(db)
	require.NoError(t, m.Create(context.Background(), &model.Ticket{Title: "T1", Category: "bug", Assignee: "alice", GameID: "game-a", Env: "prod"}))

	ctx, rec := newTicketRequest(http.MethodGet, fmt.Sprintf("/api/v1/tickets/filter-options"), "")
	handler.FilterOptions(ctx)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp FilterOptionsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, []TicketFilterOption{{Name: "bug", Count: 1}}, resp.Categories)
	assert.Equal(t, []TicketFilterOption{{Name: "alice", Count: 1}}, resp.Assignees)
}
