package admin

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---- v9 helpers ----

var v9AdminDBSeq uint64

// newAdminV9DB creates a fresh in-memory DB per scenario so injected gorm
// callbacks never leak into the shared test database.
func newAdminV9DB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("adminv9_%d", atomic.AddUint64(&v9AdminDBSeq, 1))
	db, err := gorm.Open(gsqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.AutoMigrate(db))
	return db
}

type v9AdminFixture struct {
	db      *gorm.DB
	svcCtx  *svc.ServiceContext
	service *Service
	ctx     context.Context
	adminID uint
}

func newAdminV9Fixture(t *testing.T) *v9AdminFixture {
	t.Helper()
	db := newAdminV9DB(t)
	seedTestPermissions(t, db)
	ctx, adminID := createTestAdminWithContext(t, db, "v9admin", "MyPass123", "admin")
	svcCtx := setupTestServiceContext(t, db)
	return &v9AdminFixture{db: db, svcCtx: svcCtx, service: NewService(svcCtx), ctx: ctx, adminID: adminID}
}

func v9AdminTableOf(tx *gorm.DB) string {
	if tx.Statement == nil {
		return ""
	}
	if tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	if tx.Statement.Schema != nil {
		return tx.Statement.Schema.Table
	}
	return ""
}

// v9AdminFailQueryOn makes queries touching table fail from the from-th hit (1-based).
// It hooks both the Query chain (Find/First/Count) and the Row chain (Table().Scan()).
func v9AdminFailQueryOn(db *gorm.DB, table string, from int) {
	var hits int
	matcher := func(tx *gorm.DB) {
		if v9AdminTableOf(tx) != table {
			return
		}
		hits++
		if hits >= from {
			tx.AddError(errors.New("v9 forced query error on " + table))
		}
	}
	db.Callback().Query().Before("gorm:query").Register("v9_fail_query_"+table, matcher)
	db.Callback().Row().Before("gorm:row").Register("v9_fail_row_"+table, matcher)
}

func v9AdminFailCreateOn(db *gorm.DB, table string) {
	db.Callback().Create().Before("gorm:create").Register("v9_fail_create_"+table, func(tx *gorm.DB) {
		if v9AdminTableOf(tx) == table {
			tx.AddError(errors.New("v9 forced create error on " + table))
		}
	})
}

// v9AdminFailUpdateOn fails updates on table whose destination map contains destKey.
func v9AdminFailUpdateOn(db *gorm.DB, table, destKey string) {
	db.Callback().Update().Before("gorm:update").Register("v9_fail_update_"+table+"_"+destKey, func(tx *gorm.DB) {
		if v9AdminTableOf(tx) != table {
			return
		}
		m, ok := tx.Statement.Dest.(map[string]interface{})
		if !ok || m[destKey] == nil {
			return
		}
		tx.AddError(errors.New("v9 forced update error on " + table + "." + destKey))
	})
}

func v9AdminFailDeleteOn(db *gorm.DB, table string) {
	db.Callback().Delete().Before("gorm:delete").Register("v9_fail_delete_"+table, func(tx *gorm.DB) {
		if v9AdminTableOf(tx) == table {
			tx.AddError(errors.New("v9 forced delete error on " + table))
		}
	})
}

func (f *v9AdminFixture) seedGameV9(t *testing.T, gameID, name, alias string) uint {
	t.Helper()
	game := &model.Game{GameID: gameID, Name: name, AliasName: alias, Enabled: true}
	require.NoError(t, f.db.Create(game).Error)
	return game.ID
}

// ---- List ----

func TestV9AdminListQueryErrorV9(t *testing.T) {
	f := newAdminV9Fixture(t)
	v9AdminFailQueryOn(f.db, "admins", 2) // hit 1 = permission lookup

	_, err := f.service.List(f.ctx, &ListRequest{Page: 1, PageSize: 10})
	require.Error(t, err)
}

func TestV9AdminListRoleMapErrorV9(t *testing.T) {
	f := newAdminV9Fixture(t)
	v9AdminFailQueryOn(f.db, "admin_roles", 1)

	_, err := f.service.List(f.ctx, &ListRequest{Page: 1, PageSize: 10})
	require.Error(t, err)
}

// ---- Create ----

func TestV9AdminCreateRoleErrorsV9(t *testing.T) {
	t.Run("assign role error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailCreateOn(f.db, "admin_roles")

		_, err := f.service.Create(f.ctx, &CreateRequest{
			Username: "newbie", Password: "MyPass123", Roles: []string{"admin"},
		})
		require.ErrorContains(t, err, "绑定角色失败")
	})

	t.Run("fetch roles query error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailQueryOn(f.db, "roles", 2) // hit 1 = permission role load

		_, err := f.service.Create(f.ctx, &CreateRequest{
			Username: "newbie", Password: "MyPass123", Roles: []string{"admin"},
		})
		require.ErrorContains(t, err, "查询角色失败")
	})
}

// ---- Get ----

func TestV9AdminGetRolesErrorV9(t *testing.T) {
	f := newAdminV9Fixture(t)
	v9AdminFailQueryOn(f.db, "roles", 2) // hit 1 = permission role load

	_, err := f.service.Get(f.ctx, &GetRequest{ID: fmt.Sprint(f.adminID)})
	require.ErrorContains(t, err, "获取管理员角色失败")
}

// ---- Update ----

func TestV9AdminUpdateColumnErrorsV9(t *testing.T) {
	t.Run("column update error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailUpdateOn(f.db, "admins", "nickname")

		_, err := f.service.Update(f.ctx, &UpdateRequest{ID: fmt.Sprint(f.adminID), Nickname: "n"})
		require.Error(t, err)
	})

	t.Run("token bump error on disable", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailUpdateOn(f.db, "admins", "token_version")

		_, err := f.service.Update(f.ctx, &UpdateRequest{ID: fmt.Sprint(f.adminID), Status: 0})
		require.Error(t, err)
	})
}

func TestV9AdminUpdateRoleErrorsV9(t *testing.T) {
	t.Run("clear roles delete error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailDeleteOn(f.db, "admin_roles")

		_, err := f.service.Update(f.ctx, &UpdateRequest{ID: fmt.Sprint(f.adminID), Roles: []string{}})
		require.ErrorContains(t, err, "清理旧角色失败")
	})

	t.Run("role not found", func(t *testing.T) {
		f := newAdminV9Fixture(t)

		_, err := f.service.Update(f.ctx, &UpdateRequest{ID: fmt.Sprint(f.adminID), Roles: []string{"ghost"}})
		require.ErrorContains(t, err, "角色不存在")
	})

	t.Run("assign role error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailCreateOn(f.db, "admin_roles")

		_, err := f.service.Update(f.ctx, &UpdateRequest{ID: fmt.Sprint(f.adminID), Roles: []string{"admin"}})
		require.ErrorContains(t, err, "分配角色失败")
	})
}

func TestV9AdminUpdateRefetchErrorsV9(t *testing.T) {
	t.Run("refetch admin error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailQueryOn(f.db, "admins", 3) // 1=permission, 2=FindOne in tx

		_, err := f.service.Update(f.ctx, &UpdateRequest{ID: fmt.Sprint(f.adminID), Nickname: "n"})
		require.Error(t, err)
	})

	t.Run("refetch roles error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailQueryOn(f.db, "roles", 2) // 1=permission role load

		_, err := f.service.Update(f.ctx, &UpdateRequest{ID: fmt.Sprint(f.adminID), Nickname: "n"})
		require.Error(t, err)
	})
}

// ---- Delete ----

func TestV9AdminDeleteScopeErrorsV9(t *testing.T) {
	t.Run("role binding delete error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailDeleteOn(f.db, "admin_roles")

		err := f.service.Delete(f.ctx, &DeleteRequest{ID: fmt.Sprint(f.adminID)})
		require.ErrorContains(t, err, "删除角色绑定失败")
	})

	t.Run("game scope delete error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailDeleteOn(f.db, "admin_game_scopes")

		err := f.service.Delete(f.ctx, &DeleteRequest{ID: fmt.Sprint(f.adminID)})
		require.ErrorContains(t, err, "删除游戏范围失败")
	})

	t.Run("env scope delete error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailDeleteOn(f.db, "admin_game_env_scopes")

		err := f.service.Delete(f.ctx, &DeleteRequest{ID: fmt.Sprint(f.adminID)})
		require.ErrorContains(t, err, "删除环境范围失败")
	})
}

// ---- PasswordReset ----

func TestV9AdminPasswordResetErrorsV9(t *testing.T) {
	t.Run("update password error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailUpdateOn(f.db, "admins", "password_hash")

		err := f.service.PasswordReset(f.ctx, &PasswordResetRequest{ID: fmt.Sprint(f.adminID), NewPassword: "MyPass456"})
		require.Error(t, err)
	})

	t.Run("token bump error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailUpdateOn(f.db, "admins", "token_version")

		err := f.service.PasswordReset(f.ctx, &PasswordResetRequest{ID: fmt.Sprint(f.adminID), NewPassword: "MyPass456"})
		require.Error(t, err)
	})
}

// ---- GetGames ----

func TestV9AdminGetGamesErrorsV9(t *testing.T) {
	t.Run("permission ids query error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailQueryOn(f.db, "role_permissions", 1)

		_, err := f.service.GetGames(f.ctx, &GetGamesRequest{ID: fmt.Sprint(f.adminID)})
		require.Error(t, err)
	})

	t.Run("nil game model", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		f.svcCtx.GameModel = nil

		_, err := f.service.GetGames(f.ctx, &GetGamesRequest{ID: fmt.Sprint(f.adminID)})
		require.ErrorContains(t, err, "DB/GameModel 未初始化")
	})

	t.Run("env scope query error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailQueryOn(f.db, "admin_game_env_scopes", 1)

		_, err := f.service.GetGames(f.ctx, &GetGamesRequest{ID: fmt.Sprint(f.adminID)})
		require.ErrorContains(t, err, "query env scopes failed")
	})

	t.Run("game scope query error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailQueryOn(f.db, "admin_game_scopes", 1)

		_, err := f.service.GetGames(f.ctx, &GetGamesRequest{ID: fmt.Sprint(f.adminID)})
		require.ErrorContains(t, err, "query game scopes failed")
	})
}

func TestV9AdminGetGamesShapesV9(t *testing.T) {
	f := newAdminV9Fixture(t)

	ga := f.seedGameV9(t, "alpha", "Alpha", "aa")
	gb := f.seedGameV9(t, "beta", "", "beta-alias")
	gc := f.seedGameV9(t, "gamma", "Gamma", "gamma-alias")
	gd := f.seedGameV9(t, "delta", "", "")

	// Env scopes: blank env row skipped; alias fallback for game with empty name.
	require.NoError(t, f.db.Create(&model.AdminGameEnvScope{AdminID: f.adminID, GameID: ga, Env: "  "}).Error)
	require.NoError(t, f.db.Create(&model.AdminGameEnvScope{AdminID: f.adminID, GameID: ga, Env: "prod"}).Error)
	require.NoError(t, f.db.Create(&model.AdminGameEnvScope{AdminID: f.adminID, GameID: gb, Env: "dev"}).Error)
	require.NoError(t, f.db.Create(&model.AdminGameEnvScope{AdminID: f.adminID, GameID: gd, Env: "test"}).Error)
	// Game-only scope plus duplicate for an already env-scoped game.
	require.NoError(t, f.db.Create(&model.AdminGameScope{AdminID: f.adminID, GameID: gc}).Error)
	require.NoError(t, f.db.Create(&model.AdminGameScope{AdminID: f.adminID, GameID: ga}).Error)

	resp, err := f.service.GetGames(f.ctx, &GetGamesRequest{ID: fmt.Sprint(f.adminID)})
	require.NoError(t, err)
	assert.Len(t, resp.Games, 4)

	byGameID := map[string][]AdminGame{}
	for _, g := range resp.Games {
		byGameID[g.GameId] = append(byGameID[g.GameId], g)
	}
	// Alpha: name present, env scope only.
	assert.Equal(t, []string{"prod"}, byGameID["Alpha"][0].Envs)
	assert.Equal(t, "Alpha", byGameID["Alpha"][0].GameName)
	// Game-only scope resolved through GameModel.
	assert.Equal(t, "gamma-alias", byGameID["Gamma"][0].GameName)
	assert.Empty(t, byGameID["Gamma"][0].Envs)
	// Two games whose Name is empty both map to GameId "" in the response;
	// one falls back to its alias, the other to the (empty) GameId.
	blank := byGameID[""]
	names := []string{blank[0].GameName, blank[1].GameName}
	assert.Contains(t, names, "beta-alias")
	assert.Contains(t, names, "")
	envs := append(blank[0].Envs, blank[1].Envs...)
	assert.ElementsMatch(t, []string{"dev", "test"}, envs)
}

// ---- UpdateGames ----

func TestV9AdminUpdateGamesErrorsV9(t *testing.T) {
	t.Run("permission ids query error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailQueryOn(f.db, "role_permissions", 1)

		_, err := f.service.UpdateGames(f.ctx, &UpdateGamesRequest{ID: fmt.Sprint(f.adminID)})
		require.Error(t, err)
	})

	t.Run("nil game model", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		f.svcCtx.GameModel = nil

		_, err := f.service.UpdateGames(f.ctx, &UpdateGamesRequest{ID: fmt.Sprint(f.adminID)})
		require.ErrorContains(t, err, "DB/GameModel 未初始化")
	})

	t.Run("delete env scope error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailDeleteOn(f.db, "admin_game_env_scopes")

		_, err := f.service.UpdateGames(f.ctx, &UpdateGamesRequest{ID: fmt.Sprint(f.adminID)})
		require.Error(t, err)
	})

	t.Run("delete game scope error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		v9AdminFailDeleteOn(f.db, "admin_game_scopes")

		_, err := f.service.UpdateGames(f.ctx, &UpdateGamesRequest{ID: fmt.Sprint(f.adminID)})
		require.Error(t, err)
	})

	t.Run("create game scope error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		f.seedGameV9(t, "g1", "G", "")
		v9AdminFailCreateOn(f.db, "admin_game_scopes")

		_, err := f.service.UpdateGames(f.ctx, &UpdateGamesRequest{
			ID:    fmt.Sprint(f.adminID),
			Games: []AdminGame{{GameId: "G"}},
		})
		require.Error(t, err)
	})

	t.Run("create env scope error", func(t *testing.T) {
		f := newAdminV9Fixture(t)
		f.seedGameV9(t, "g1", "G", "")
		v9AdminFailCreateOn(f.db, "admin_game_env_scopes")

		_, err := f.service.UpdateGames(f.ctx, &UpdateGamesRequest{
			ID:    fmt.Sprint(f.adminID),
			Games: []AdminGame{{GameId: "G", Envs: []string{"prod"}}},
		})
		require.Error(t, err)
	})

	t.Run("game not found", func(t *testing.T) {
		f := newAdminV9Fixture(t)

		_, err := f.service.UpdateGames(f.ctx, &UpdateGamesRequest{
			ID:    fmt.Sprint(f.adminID),
			Games: []AdminGame{{GameId: "missing"}},
		})
		require.ErrorContains(t, err, "game not found")
	})
}

func TestV9AdminUpdateGamesSuccessV9(t *testing.T) {
	f := newAdminV9Fixture(t)
	f.seedGameV9(t, "g1", "G", "")

	// Blank game entry and blank env entries are skipped, valid ones persist.
	resp, err := f.service.UpdateGames(f.ctx, &UpdateGamesRequest{
		ID: fmt.Sprint(f.adminID),
		Games: []AdminGame{
			{GameId: "   "},
			{GameId: "G", Envs: []string{"   ", "prod"}},
		},
	})
	require.NoError(t, err)
	require.Len(t, resp.Games, 1)
	assert.Equal(t, []string{"prod"}, resp.Games[0].Envs)

	var scopes int64
	require.NoError(t, f.db.Model(&model.AdminGameScope{}).Count(&scopes).Error)
	assert.Equal(t, int64(1), scopes)
}
