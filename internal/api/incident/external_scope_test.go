package incident

// AUD-036 回归用例：外部 incident token 的 game/env 允许面——签发（写）与
// 查询（读）都强制按 token scope 过滤，含越权访问负例。

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

// ---- model 允许面解析 ----

func TestTokenAllowlistParse(t *testing.T) {
	// null / 空 = 不限（nil）
	tok := &model.ExternalToken{}
	assert.Nil(t, tok.ScopeGameIDs(), "null game allowlist should be nil")
	assert.Nil(t, tok.ScopeEnvs(), "null env allowlist should be nil")

	tok.GameAllowlist = model.JSON(`["game-a","game-b"]`)
	tok.EnvAllowlist = model.JSON(`["prod","staging"]`)
	assert.Equal(t, []string{"game-a", "game-b"}, tok.ScopeGameIDs())
	assert.Equal(t, []string{"prod", "staging"}, tok.ScopeEnvs())

	// 空数组 = 不限
	tok.GameAllowlist = model.JSON(`[]`)
	assert.Nil(t, tok.ScopeGameIDs(), "empty array should be nil")

	// 坏 JSON = 不限兜底
	tok.GameAllowlist = model.JSON(`{bad`)
	assert.Nil(t, tok.ScopeGameIDs())
}

// ---- 写入强制（RegisterIncident）----

func TestRegisterIncident_ScopeEnforced(t *testing.T) {
	s, _ := setupExternal(t)
	ctx := context.Background()
	cat := mustCategory(t, s.incSvc, model.IncidentCatOps)

	// 允许面限定 game-a/prod
	tok := tokenWithAllowlist([]string{"game-a"}, []string{"prod"})

	// 面内 → 成功
	dto, created, err := s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "面内事故", CategoryID: cat.ID, Subcategory: "部署", GameID: "game-a", Env: "prod",
	}, tok, "scope-ok-1")
	require.NoError(t, err)
	require.True(t, created)
	assert.Equal(t, "game-a", dto.GameID)
	assert.Equal(t, "prod", dto.Env)

	// 越权负例：其他 game → 403
	_, _, err = s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "越权 game", CategoryID: cat.ID, Subcategory: "部署", GameID: "game-b", Env: "prod",
	}, tok, "scope-ko-game")
	require.Equal(t, http.StatusForbidden, codeOf(t, err).Code, err)

	// 越权负例：其他 env → 403
	_, _, err = s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "越权 env", CategoryID: cat.ID, Subcategory: "部署", GameID: "game-a", Env: "staging",
	}, tok, "scope-ko-env")
	require.Equal(t, http.StatusForbidden, codeOf(t, err).Code, err)

	// 缺省 scope（空 game/env）→ 403：无法证明在面内，拒绝而非回落全局
	_, _, err = s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "缺省 scope", CategoryID: cat.ID, Subcategory: "部署", GameID: "", Env: "prod",
	}, tok, "scope-ko-empty")
	require.Equal(t, http.StatusForbidden, codeOf(t, err).Code, err)

	// 无允许面 token → 任意 game/env 可写（向后兼容）
	openTok := &model.ExternalToken{Name: "open-bot"}
	_, _, err = s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "开放 token", CategoryID: cat.ID, Subcategory: "部署", GameID: "game-z", Env: "dev",
	}, openTok, "scope-open-1")
	require.NoError(t, err)

	// 只限 game（env 不限）→ env 任意、game 受限
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

// ---- 写入强制（CreateBug）----

func TestCreateBug_ScopeEnforced(t *testing.T) {
	s, _ := setupExternal(t)
	ctx := contextWithUsername(context.Background(), "ext:scoped-bot")

	tok := tokenWithAllowlist([]string{"game-a"}, []string{"prod"})

	// 面内 → 成功
	_, err := s.CreateBug(ctx, &bug.BugCreateRequest{Title: "面内 bug", GameID: "game-a", Env: "prod"}, tok)
	require.NoError(t, err)

	// 越权负例：其他 env → 403
	_, err = s.CreateBug(ctx, &bug.BugCreateRequest{Title: "越权 bug", GameID: "game-a", Env: "staging"}, tok)
	require.Equal(t, http.StatusForbidden, codeOf(t, err).Code, err)
}

// ---- 读取强制（ListIncidents）----

func TestListIncidents_GameEnvScopeFilter(t *testing.T) {
	s, _ := setupExternal(t)
	ctx := context.Background()
	cat := mustCategory(t, s.incSvc, model.IncidentCatOps)
	openTok := &model.ExternalToken{Name: "open-bot"}

	// 播种三条跨 game/env 的事故
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

	// 无过滤条件 → 只收窄到 game-a/prod
	resp, err := s.ListIncidents(ctx, model.IncidentQueryOptions{}, scoped)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "k-game-a-prod", resp.Items[0].Title)
	assert.EqualValues(t, 1, resp.Total)

	// 显式查面外 game → 空（不降级为全量）
	resp, err = s.ListIncidents(ctx, model.IncidentQueryOptions{GameID: "game-b"}, scoped)
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
	assert.EqualValues(t, 0, resp.Total)

	// 显式查面外 env → 空
	resp, err = s.ListIncidents(ctx, model.IncidentQueryOptions{Env: "staging"}, scoped)
	require.NoError(t, err)
	assert.Empty(t, resp.Items)

	// 显式查面内 game → 只回该 game 且 env 仍收窄
	resp, err = s.ListIncidents(ctx, model.IncidentQueryOptions{GameID: "game-a"}, scoped)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "k-game-a-prod", resp.Items[0].Title)

	// 无允许面 token → 全量（向后兼容）
	resp, err = s.ListIncidents(ctx, model.IncidentQueryOptions{}, openTok)
	require.NoError(t, err)
	require.Len(t, resp.Items, 3)
}

// ---- handler 签发持久化 ----

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

	// 落库列已持久化
	var row model.ExternalToken
	require.NoError(t, db.First(&row, "name = ?", "scoped-issue").Error)
	assert.Equal(t, []string{"game-a", "game-b"}, row.ScopeGameIDs())
	assert.Equal(t, []string{"prod", "staging"}, row.ScopeEnvs())
}
