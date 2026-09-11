package audit

import (
	"context"
	"testing"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 双实例链 sequence 冲突回归（2026-09-11 线上暴露）：两个 store 共享同一
// DB、各自独立 memCache（= 两台 server 实例）。实例 B 的 memCache 停留在
// 旧链尾（A 持续写入而 B 无感知），B 按 +1 分配的 sequence 撞 A 已写行——
// 修复前唯一约束冲突被 AuditService 之外的上层吞掉，owner 侧审计静默丢失。
func TestAuditService_LogChainSequenceConflictAcrossInstances(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuditModel{}))

	// 实例 A 先建链（seq 1）。
	storeA, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	svcA := NewAuditService(storeA, nil)
	require.NoError(t, logPlain(t, svcA, "evt-a1"))

	// 实例 B 启动（加载当时链尾 seq=1），随后 A 独自推进两轮（seq 2、3），
	// B 的 memCache 仍停在 1。
	storeB, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	svcB := NewAuditService(storeB, nil)
	require.NoError(t, logPlain(t, svcA, "evt-a2"))
	require.NoError(t, logPlain(t, svcA, "evt-a3"))

	// B 写入：按过期 memCache 分配 seq=2 → 撞唯一约束 → 刷缓存重查真实
	// 链尾（seq=3）重算 → seq=4 落库成功。
	recB, err := svcB.Log(context.Background(), EventLogin,
		WithActorID("server-b", "user", "server-b"),
		WithDetails(map[string]interface{}{"forwarded": true}),
	)
	require.NoError(t, err, "conflict must be retried with the real chain tail")
	require.EqualValues(t, 4, recB.ChainInfo.Sequence)

	// 全链校验：sequence 连续且 prev_hash 逐条衔接（多实例写不产生分叉）。
	var count int64
	require.NoError(t, db.Model(&AuditModel{}).Count(&count).Error)
	require.EqualValues(t, 4, count)
	records, err := storeA.GetChainRange(1, count)
	require.NoError(t, err)
	require.Len(t, records, 4)
	prevHash := ""
	for i, r := range records {
		require.EqualValues(t, i+1, r.ChainInfo.Sequence, "sequence must be dense")
		if prevHash != "" {
			require.Equal(t, prevHash, r.ChainInfo.PrevHash, "chain must not fork at seq %d", r.ChainInfo.Sequence)
		}
		prevHash = r.ChainInfo.Hash
	}
}

// 重试耗尽仍冲突时返回原始存储错误（调用方可感知，不再静默）。
func TestAuditService_LogChainConflictRetryExhausted(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&AuditModel{}))

	store, err := NewSQLAuditStore(db)
	require.NoError(t, err)
	svc := NewAuditService(&chainStuckStore{SQLAuditStore: store}, nil)

	_, err = svc.Log(context.Background(), EventLogin)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrChainSequenceConflict)
}

// chainStuckStore 把每次 Create 都报成链冲突（模拟对端持续抢占，
// 重试 3 次仍耗尽的极端并发）。嵌入满足 AuditStore 其余方法。
type chainStuckStore struct {
	*SQLAuditStore
}

func (s *chainStuckStore) Create(record *AuditRecord) error {
	return ErrChainSequenceConflict
}

func logPlain(t *testing.T, svc *AuditService, action string) error {
	t.Helper()
	_, err := svc.Log(context.Background(), EventLogin, WithActorID(action, "user", action))
	return err
}
