package incident

// AUD-036 regression cases: the game/env allowlist of external incident tokens -
// both issuing (write) and querying (read) are strictly filtered by the token
// scope, including unauthorized-access negative cases.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/api/bug"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tokenWithAllowlist(games, envs []string) *model.ExternalToken {
	return &model.ExternalToken{
		Name:          "scoped-bot",
		GameAllowlist: stringSliceJSON(games),
		EnvAllowlist:  stringSliceJSON(envs),
	}
}

// ---- model allowlist parsing ----

func TestTokenAllowlistParse(t *testing.T) {
	// null / empty = unrestricted (nil)
	tok := &model.ExternalToken{}
	assert.Nil(t, tok.ScopeGameIDs(), "null game allowlist should be nil")
	assert.Nil(t, tok.ScopeEnvs(), "null env allowlist should be nil")

	tok.GameAllowlist = model.JSON(`["game-a","game-b"]`)
	tok.EnvAllowlist = model.JSON(`["prod","staging"]`)
	assert.Equal(t, []string{"game-a", "game-b"}, tok.ScopeGameIDs())
	assert.Equal(t, []string{"prod", "staging"}, tok.ScopeEnvs())

	// Empty array = unrestricted
	tok.GameAllowlist = model.JSON(`[]`)
	assert.Nil(t, tok.ScopeGameIDs(), "empty array should be nil")

	// Bad JSON = unrestricted fallback
	tok.GameAllowlist = model.JSON(`{bad`)
	assert.Nil(t, tok.ScopeGameIDs())
}

// ---- Write enforcement (RegisterIncident) ----

func TestRegisterIncident_ScopeEnforced(t *testing.T) {
	s, _ := setupExternal(t)
	ctx := context.Background()
	cat := mustCategory(t, s.incSvc, model.IncidentCatOps)

	// Allowlist limited to game-a/prod
	tok := tokenWithAllowlist([]string{"game-a"}, []string{"prod"})

	// In scope -> success
	dto, created, err := s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "面内事故", CategoryID: cat.ID, Subcategory: "部署", GameID: "game-a", Env: "prod",
	}, tok, "scope-ok-1")
	require.NoError(t, err)
	require.True(t, created)
	assert.Equal(t, "game-a", dto.GameID)
	assert.Equal(t, "prod", dto.Env)

	// Unauthorized negative case: another game -> 403
	_, _, err = s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "越权 game", CategoryID: cat.ID, Subcategory: "部署", GameID: "game-b", Env: "prod",
	}, tok, "scope-ko-game")
	require.Equal(t, http.StatusForbidden, codeOf(t, err).Code, err)

	// Unauthorized negative case: another env -> 403
	_, _, err = s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "越权 env", CategoryID: cat.ID, Subcategory: "部署", GameID: "game-a", Env: "staging",
	}, tok, "scope-ko-env")
	require.Equal(t, http.StatusForbidden, codeOf(t, err).Code, err)

	// Missing scope (empty game/env) -> 403: an empty target cannot be proven
	// in-scope, so reject rather than fall back to the global scope
	_, _, err = s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "缺省 scope", CategoryID: cat.ID, Subcategory: "部署", GameID: "", Env: "prod",
	}, tok, "scope-ko-empty")
	require.Equal(t, http.StatusForbidden, codeOf(t, err).Code, err)

	// Token with no allowlist -> any game/env is writable (backwards compatible)
	openTok := &model.ExternalToken{Name: "open-bot"}
	_, _, err = s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "开放 token", CategoryID: cat.ID, Subcategory: "部署", GameID: "game-z", Env: "dev",
	}, openTok, "scope-open-1")
	require.NoError(t, err)

	// Game only (env unrestricted) -> env is free, game is restricted
	gameOnly := tokenWithAllowlist([]string{"game-a"}, nil)
	_, _, err = s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "只限 game", CategoryID: cat.ID, Subcategory: "部署", GameID: "game-a", Env: "any-env",
	}, gameOnly, "scope-gameonly-1")
	require.NoError(t, err)
	_, _, err = s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "只限 game 越权", CategoryID: cat.ID, Subcategory: "部署", GameID: "game-b", Env: "any-env",
	}, gameOnly, "scope-gameonly-ko")
	require.Equal(t, http.StatusForbidden, codeOf(t, err).Code, err)
}

// ---- Write enforcement (CreateBug) ----

func TestCreateBug_ScopeEnforced(t *testing.T) {
	s, _ := setupExternal(t)
	ctx := contextWithUsername(context.Background(), "ext:scoped-bot")

	tok := tokenWithAllowlist([]string{"game-a"}, []string{"prod"})

	// In scope -> success
	_, err := s.CreateBug(ctx, &bug.BugCreateRequest{Title: "面内 bug", GameID: "game-a", Env: "prod"}, tok)
	require.NoError(t, err)

	// Unauthorized negative case: another env -> 403
	_, err = s.CreateBug(ctx, &bug.BugCreateRequest{Title: "越权 bug", GameID: "game-a", Env: "staging"}, tok)
	require.Equal(t, http.StatusForbidden, codeOf(t, err).Code, err)
}

// ---- Read enforcement (ListIncidents) ----

func TestListIncidents_GameEnvScopeFilter(t *testing.T) {
	s, _ := setupExternal(t)
	ctx := context.Background()
	cat := mustCategory(t, s.incSvc, model.IncidentCatOps)
	openTok := &model.ExternalToken{Name: "open-bot"}

	// Seed three incidents spread across game/env
	seeds := []struct {
		key, game, env string
	}{
		{"k-game-a-prod", "game-a", "prod"},
		{"k-game-b-prod", "game-b", "prod"},
		{"k-game-a-staging", "game-a", "staging"},
	}
	for _, sd := range seeds {
		_, _, err := s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
			Title: sd.key, CategoryID: cat.ID, Subcategory: "部署", GameID: sd.game, Env: sd.env,
		}, openTok, sd.key)
		require.NoError(t, err)
	}

	scoped := tokenWithAllowlist([]string{"game-a"}, []string{"prod"})

	// No filter conditions -> narrowed to game-a/prod only
	resp, err := s.ListIncidents(ctx, model.IncidentQueryOptions{}, scoped)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "k-game-a-prod", resp.Items[0].Title)
	assert.EqualValues(t, 1, resp.Total)

	// Explicit out-of-scope game query -> empty (no degrade to the full list)
	resp, err = s.ListIncidents(ctx, model.IncidentQueryOptions{GameID: "game-b"}, scoped)
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
	assert.EqualValues(t, 0, resp.Total)

	// Explicit out-of-scope env query -> empty
	resp, err = s.ListIncidents(ctx, model.IncidentQueryOptions{Env: "staging"}, scoped)
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	// Explicit in-scope game query -> only that game, and env is still narrowed
	resp, err = s.ListIncidents(ctx, model.IncidentQueryOptions{GameID: "game-a"}, scoped)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "k-game-a-prod", resp.Items[0].Title)

	// Token with no allowlist -> full list (backwards compatible)
	resp, err = s.ListIncidents(ctx, model.IncidentQueryOptions{}, openTok)
	require.NoError(t, err)
	require.Len(t, resp.Items, 3)
}

// ---- handler issuance persistence ----

func TestCreateToken_PersistsAllowlist(t *testing.T) {
	s, db := setupExternal(t)
	gin.SetMode(gin.TestMode)
	h := NewExternalHandler(s, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("username", "admin-tester")
	body := `{"name":"scoped-issue","scope":{"categories":["ops"],"gameIds":["game-a","game-b"],"envs":["prod","staging"]}}`
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/ext/tokens", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	h.CreateToken(c)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp ExternalTokenDTO
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, []interface{}{"game-a", "game-b"}, resp.Scope["gameIds"])
	assert.Equal(t, []interface{}{"prod", "staging"}, resp.Scope["envs"])

	// The allowlist columns are persisted in the database
	var row model.ExternalToken
	require.NoError(t, db.First(&row, "name = ?", "scoped-issue").Error)
	assert.Equal(t, []string{"game-a", "game-b"}, row.ScopeGameIDs())
	assert.Equal(t, []string{"prod", "staging"}, row.ScopeEnvs())
}
