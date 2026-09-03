package cluster

import (
	"context"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newDBFileV9(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func closeDBV9(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

func TestDBOwnerResolverForSelfV9(t *testing.T) {
	db := newDB(t)
	r := NewDBOwnerResolverForSelf(db, 0, "inst-x")
	assert.Equal(t, 3*time.Minute, r.ownerTTL, "ownerTTL<=0 应取默认 3 分钟")
	ctx := context.Background()
	require.NoError(t, r.EnsureTable(ctx))

	require.NoError(t, r.ClaimOwner(ctx, "ag-1", "game-1", "prod", "inst-x", 4))
	require.NoError(t, r.Touch(ctx, "ag-1"))
	require.NoError(t, r.Release(ctx, "ag-1"))
	rec, err := r.FindOwner(ctx, "ag-1")
	require.NoError(t, err)
	assert.Nil(t, rec, "Release 后记录应删除")

	require.NoError(t, r.ClaimOwner(ctx, "ag-2", "game-2", "dev", "other-inst", 4))
	g, e, ok := r.SelfOwnerScope(ctx, "ag-2")
	assert.False(t, ok)
	assert.Empty(t, g)
	assert.Empty(t, e)

	require.NoError(t, r.ClaimOwner(ctx, "ag-3", "game-3", "staging", "inst-x", 4))
	g, e, ok = r.SelfOwnerScope(ctx, "ag-3")
	assert.True(t, ok)
	assert.Equal(t, "game-3", g)
	assert.Equal(t, "staging", e)

	g, e, ok = r.SelfOwnerScope(ctx, "ghost")
	assert.False(t, ok)
}

func TestDBOwner_ClaimUpdatePathV9(t *testing.T) {
	db := newDB(t)
	r := NewDBOwnerResolver(db, time.Minute)
	ctx := context.Background()
	require.NoError(t, r.EnsureTable(ctx))

	require.NoError(t, r.ClaimOwner(ctx, "ag-1", "g1", "prod", "self", 1))
	require.NoError(t, r.ClaimOwner(ctx, "ag-1", "g2", "dev", "peer-b", 9))
	rec, err := r.FindOwner(ctx, "ag-1")
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "peer-b", rec.InstanceID)
	assert.EqualValues(t, 9, rec.OwnerEpoch)
	assert.Equal(t, "g2", rec.GameID)
	assert.Equal(t, "dev", rec.Env)
}

func TestDBOwner_ListFindScopeV9(t *testing.T) {
	db := newDB(t)
	r := NewDBOwnerResolverForSelf(db, time.Minute, "me")
	ctx := context.Background()
	require.NoError(t, r.EnsureTable(ctx))

	require.NoError(t, r.ClaimOwner(ctx, "ag-2", "g", "e", "me", 1))
	require.NoError(t, r.ClaimOwner(ctx, "ag-1", "g", "e", "other", 1))
	require.NoError(t, r.ClaimOwner(ctx, "ag-3", "g", "e", "other", 1))

	rows, err := r.ListAliveOwners(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, "ag-1", rows[0].AgentID)
	assert.Equal(t, "ag-2", rows[1].AgentID)
	assert.Equal(t, "ag-3", rows[2].AgentID)

	rec, err := r.FindOwner(ctx, "ag-1")
	require.NoError(t, err)
	require.NotNil(t, rec)
	assert.Equal(t, "other", rec.InstanceID)

	rec, err = r.FindOwner(ctx, "ghost")
	require.NoError(t, err)
	assert.Nil(t, rec)
}

func TestDBOwner_ErrorPathsV9(t *testing.T) {
	db := newDB(t)
	r := NewDBOwnerResolver(db, time.Minute)
	ctx := context.Background()
	require.NoError(t, r.EnsureTable(ctx))
	closeDBV9(t, db)

	assert.Error(t, r.ClaimOwner(ctx, "ag", "g", "e", "self", 1))
	_, err := r.ResolveOwner(ctx, "ag")
	assert.Error(t, err)
	_, err = r.CountAgentsByOwner(ctx)
	assert.Error(t, err)
	_, err = r.ListAliveOwners(ctx)
	assert.Error(t, err)
	_, err = r.FindOwner(ctx, "ag")
	assert.Error(t, err)

	db2 := newDB(t)
	r2 := NewDBOwnerResolver(db2, time.Minute)
	require.NoError(t, r2.EnsureTable(ctx))
	assert.NoError(t, r2.Touch(ctx, "any"), "无 mesh/override 时 selfInstanceID 为空串，不应报错")
}

func TestDBMembership_DefaultTTLAndWriteErrorsV9(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/cluster_v9.db"
	db := newDBFileV9(t, path)
	m := NewDBMembership(db, 0)
	assert.Equal(t, 15*time.Second, m.leaseTTL, "leaseTTL<=0 应取默认 15s")
	require.NoError(t, m.EnsureTable(ctx))

	epoch, err := m.Register(ctx, PeerInfo{InstanceID: "a", AdvertiseAddr: "10.0.0.1:8444"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, epoch)

	ro := newDBFileV9(t, "file:"+path+"?mode=ro")
	mRO := NewDBMembership(ro, time.Second)
	_, err = mRO.Register(ctx, PeerInfo{InstanceID: "a", AdvertiseAddr: "10.0.0.1:9999"})
	assert.Error(t, err, "只读库上 Updates 必须失败")

	_, err = mRO.Register(ctx, PeerInfo{InstanceID: "b", AdvertiseAddr: "10.0.0.2:8444"})
	assert.Error(t, err, "只读库上 Create 必须失败")
}

func TestDBMembership_ClosedDBErrorsV9(t *testing.T) {
	ctx := context.Background()
	db := newDB(t)
	m := NewDBMembership(db, time.Second)
	require.NoError(t, m.EnsureTable(ctx))
	closeDBV9(t, db)

	_, err := m.Register(ctx, PeerInfo{InstanceID: "a", AdvertiseAddr: "10.0.0.1:8444"})
	assert.Error(t, err)
	assert.Error(t, m.Renew(ctx, "a"))
	_, err = m.ListAlive(ctx)
	assert.Error(t, err)
	assert.Error(t, m.Resign(ctx, "a"))
}
