package analytics

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The warehouse scope suffix and its args must map one-to-one onto the
// placeholders; a mismatch means cross-game data leakage.

func TestWarehouseScopeSuffix(t *testing.T) {
	assert.Empty(t, warehouseScopeSuffix("", ""), "空值不加过滤（作用域守卫保证调用方不会传入空值，见 warehouseScope）")
	assert.Equal(t, " AND game_id = ?", warehouseScopeSuffix("demo", ""))
	assert.Equal(t, " AND env = ?", warehouseScopeSuffix("", "prod"))
	assert.Equal(t, " AND game_id = ? AND env = ?", warehouseScopeSuffix("demo", "prod"))
}

// AUD-048: the scope is forced from the current scope. Request params may only
// narrow it (matching values pass through), never replace it (a cross-scope
// request is a 403); a missing scope is rejected outright, never expanded into
// an unfiltered query.
func TestWarehouseScope_ForcesCurrentScope(t *testing.T) {
	cases := []struct {
		name     string
		scope    svc.GameScope
		reqGame  string
		reqEnv   string
		wantGame string
		wantEnv  string
		wantErr  string
	}{
		{name: "缺省沿用 current scope", scope: svc.GameScope{GameID: "game-a", Env: "prod"}, wantGame: "game-a", wantEnv: "prod"},
		{name: "参数与 scope 一致视为收窄", scope: svc.GameScope{GameID: "game-a", Env: "prod"}, reqGame: "game-a", reqEnv: "prod", wantGame: "game-a", wantEnv: "prod"},
		{name: "请求替换 game 被拒", scope: svc.GameScope{GameID: "game-a", Env: "prod"}, reqGame: "game-b", wantErr: "gameId 与当前游戏作用域不一致"},
		{name: "请求替换 env 被拒", scope: svc.GameScope{GameID: "game-a", Env: "prod"}, reqEnv: "dev", wantErr: "env 与当前环境作用域不一致"},
		{name: "无 scope 拒绝而非无过滤", reqGame: "game-b", wantErr: "仓库查询需要游戏作用域"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := svc.WithGameScope(context.Background(), tc.scope)
			gotGame, gotEnv, err := warehouseScope(ctx, tc.reqGame, tc.reqEnv)
			if tc.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.wantGame, gotGame)
				assert.Equal(t, tc.wantEnv, gotEnv)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// AUD-048 end-to-end: a cross-scope gameId must not be pushed down into the
// query (403, the connection is never touched); when omitted, the query
// carries the current-scope filters.
func TestWarehouseDAU_Aud048_CrossScopeRejected(t *testing.T) {
	conn := &fakeConn{rows: [][]any{}}
	withFakeConn(t, conn)

	_, err := NewService(nil).WarehouseDAU(warehouseScopeCtx("game-a", "prod"),
		&WarehouseDAURequest{GameId: "game-b", Env: "prod"})
	require.Error(t, err)
	assert.Empty(t, conn.query, "跨域请求不得触达查询")

	_, err = NewService(nil).WarehouseDAU(context.Background(), &WarehouseDAURequest{})
	require.Error(t, err)
	assert.Empty(t, conn.query, "无作用域不得触达查询")
}

func TestWarehouseScopeArgs_MatchesPlaceholders(t *testing.T) {
	cases := []struct {
		gameID, env string
		want        []any
	}{
		{"", "", []any{}},
		{"demo", "", []any{"demo"}},
		{"", "prod", []any{"prod"}},
		{"demo", "prod", []any{"demo", "prod"}},
	}
	for _, tc := range cases {
		suffix := warehouseScopeSuffix(tc.gameID, tc.env)
		args := warehouseScopeArgs(tc.gameID, tc.env)
		placeholders := 0
		for _, r := range suffix {
			if r == '?' {
				placeholders++
			}
		}
		assert.Len(t, args, placeholders,
			"suffix=%q 的占位符数必须等于 args 数", suffix)
		assert.Equal(t, tc.want, args)
	}
}
