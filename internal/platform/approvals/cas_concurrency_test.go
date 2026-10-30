package approvals

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var sharedDBSeq atomic.Int64

// openSharedTestDB 打开共享缓存的 sqlite 内存库：多 goroutine/多连接共享同一
// 个 :memory: 库（默认 :memory: 是每连接一个，跨连接看不见 AutoMigrate 建的表）。
// 库名按测试唯一，避免并行测试互相污染。
func openSharedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := fmt.Sprintf("cas_shared_%d", sharedDBSeq.Add(1))
	db, err := gorm.Open(gsqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	return db
}

// ── ARCH-001 D1/D4：pending → 终态 CAS + 重复请求规则 ────────────────
// 覆盖批准/拒绝竞争、重复批准、批准后拒绝（先到先得），两种存储实现同语义。

func seedPendingApproval(t *testing.T, store Store, id string) {
	t.Helper()
	_, err := store.Create(&Approval{ID: id, State: "pending", FunctionID: "f.x", Actor: "requester"})
	require.NoError(t, err)
}

func assertConflict(t *testing.T, err error, wantState string) *Approval {
	t.Helper()
	require.Error(t, err)
	record, ok := AsAlreadyReviewed(err)
	require.True(t, ok, "err should carry ErrAlreadyReviewed, got %v", err)
	assert.Equal(t, wantState, record.State)
	return record
}

func runApproveRejectRaceCases(t *testing.T, store Store, createdID string) {
	t.Helper()

	t.Run("approve then duplicate approve", func(t *testing.T) {
		seedPendingApproval(t, store, "ar-ap-ap")
		winner, err := store.Approve("ar-ap-ap", "op-a")
		require.NoError(t, err)
		require.Equal(t, "approved", winner.State)
		assert.Equal(t, "op-a", winner.Approver)

		_, err = store.Approve("ar-ap-ap", "op-b")
		conflict := assertConflict(t, err, "approved")
		assert.Equal(t, "op-a", conflict.Approver, "first decision stands")

		got, err := store.Get("ar-ap-ap")
		require.NoError(t, err)
		assert.Equal(t, "op-a", got.Approver)
	})

	t.Run("approve then reject", func(t *testing.T) {
		seedPendingApproval(t, store, "ar-ap-rj")
		_, err := store.Approve("ar-ap-rj", "op-a")
		require.NoError(t, err)

		_, err = store.Reject("ar-ap-rj", "late reject", "op-b")
		conflict := assertConflict(t, err, "approved")
		assert.Equal(t, "op-a", conflict.Approver)

		got, err := store.Get("ar-ap-rj")
		require.NoError(t, err)
		assert.Equal(t, "approved", got.State, "first-wins: reject cannot flip terminal record")
	})

	t.Run("reject then approve", func(t *testing.T) {
		seedPendingApproval(t, store, "ar-rj-ap")
		_, err := store.Reject("ar-rj-ap", "early reject", "op-a")
		require.NoError(t, err)

		_, err = store.Approve("ar-rj-ap", "op-b")
		conflict := assertConflict(t, err, "rejected")
		assert.Equal(t, "op-a", conflict.Approver)
	})

	t.Run("duplicate reject", func(t *testing.T) {
		seedPendingApproval(t, store, "ar-rj-rj")
		_, err := store.Reject("ar-rj-rj", "first", "op-a")
		require.NoError(t, err)

		_, err = store.Reject("ar-rj-rj", "second", "op-b")
		conflict := assertConflict(t, err, "rejected")

		got, err := store.Get("ar-rj-rj")
		require.NoError(t, err)
		assert.Equal(t, "first", got.Reason, "first reason stands")
		assert.Equal(t, "op-a", conflict.Approver)
	})

	t.Run("not found stays not found", func(t *testing.T) {
		_, err := store.Approve("ar-missing", "op")
		require.Error(t, err)
		_, ok := AsAlreadyReviewed(err)
		assert.False(t, ok, "missing record is not a reviewed conflict")
		_, err = store.Reject("ar-missing", "r", "op")
		_, ok = AsAlreadyReviewed(err)
		assert.False(t, ok)
	})
}

func TestMemStoreApproveRejectRace(t *testing.T) {
	store := NewMemStore()
	runApproveRejectRaceCases(t, store, "unused")
}

func TestSQLStoreApproveRejectRace(t *testing.T) {
	store, err := NewSQLStore(openSharedTestDB(t))
	require.NoError(t, err)
	runApproveRejectRaceCases(t, store, "unused")
}

func TestSQLStoreCASUpdatesColumns(t *testing.T) {
	store, err := NewSQLStore(openSharedTestDB(t))
	require.NoError(t, err)
	seedPendingApproval(t, store, "cas-cols")
	before, err := store.Get("cas-cols")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	got, err := store.Approve("cas-cols", "op-a")
	require.NoError(t, err)
	assert.Equal(t, "approved", got.State)
	assert.Equal(t, "op-a", got.Approver)
	require.NotNil(t, got.ReviewedAt)
	assert.True(t, got.ReviewedAt.After(before.CreatedAt), "reviewed_at stamped by the CAS update")
	assert.True(t, got.UpdatedAt.After(before.CreatedAt), "updated_at stamped by the CAS update")
}

// 并发同一 pending 记录 N 次 Approve：恰好一次成功，其余全部 ErrAlreadyReviewed，
// 且赢家是唯一一次批准（D1/D2 的存储层前提）。
func TestSQLStoreConcurrentApprovesExactlyOneWins(t *testing.T) {
	store, err := NewSQLStore(openSharedTestDB(t))
	require.NoError(t, err)
	seedPendingApproval(t, store, "cas-race")

	const n = 8
	type outcome struct {
		state    string
		err      error
		approver string
	}
	results := make(chan outcome, n)
	var start sync.WaitGroup
	start.Add(1)
	for i := 0; i < n; i++ {
		go func(i int) {
			op := "op-" + string(rune('a'+i))
			start.Wait()
			rec, err := store.Approve("cas-race", op)
			out := outcome{err: err}
			if rec != nil {
				out.state, out.approver = rec.State, rec.Approver
			}
			results <- out
		}(i)
	}
	start.Done()

	winners, conflicts := 0, 0
	for i := 0; i < n; i++ {
		out := <-results
		if out.err == nil {
			winners++
			assert.Equal(t, "approved", out.state, "winner flips to approved")
			assert.NotEmpty(t, out.approver, "winner carries approver")
			continue
		}
		if _, ok := AsAlreadyReviewed(out.err); ok {
			conflicts++
		} else {
			t.Fatalf("unexpected error: %v", out.err)
		}
	}
	assert.Equal(t, 1, winners, "exactly one approve wins the pending→approved transition")
	assert.Equal(t, n-1, conflicts, "all losers report already-reviewed")

	got, err := store.Get("cas-race")
	require.NoError(t, err)
	assert.Equal(t, "approved", got.State)
}

func TestMemStoreConcurrentApprovesExactlyOneWins(t *testing.T) {
	store := NewMemStore()
	seedPendingApproval(t, store, "cas-race-mem")

	const n = 8
	var mu sync.Mutex
	winners, conflicts := 0, 0
	var start sync.WaitGroup
	start.Add(1)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			op := "op-" + string(rune('a'+i))
			start.Wait()
			_, err := store.Approve("cas-race-mem", op)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				winners++
			case errors.Is(err, ErrAlreadyReviewed):
				conflicts++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	start.Done()
	wg.Wait()

	assert.Equal(t, 1, winners)
	assert.Equal(t, n-1, conflicts)
}

// ── ARCH-001 D7 / ARCH-002：批准续跑的持久执行领取 ───────────────────

func TestClaimForDispatchExactlyOnce(t *testing.T) {
	t.Run("sql claim persists and rejects repeat", func(t *testing.T) {
		store, err := NewSQLStore(openSharedTestDB(t))
		require.NoError(t, err)
		seedPendingApproval(t, store, "claim-sql")
		_, err = store.Approve("claim-sql", "op-a")
		require.NoError(t, err)

		digest := "digest-sql"
		winner, err := store.ClaimForDispatch("claim-sql", "token-sql", digest)
		require.NoError(t, err)
		require.Equal(t, "claimed", winner.ClaimStatus)
		require.Equal(t, "token-sql", winner.ClaimToken)
		require.NotNil(t, winner.ClaimedAt)
		require.Equal(t, digest, winner.PayloadDigest)

		// New instance / reopened read sees the same durable intent.
		reloaded, err := store.Get("claim-sql")
		require.NoError(t, err)
		require.Equal(t, "claimed", reloaded.ClaimStatus)
		require.Equal(t, "token-sql", reloaded.ClaimToken)
		require.Equal(t, digest, reloaded.PayloadDigest)

		_, err = store.ClaimForDispatch("claim-sql", "token-next", digest)
		conflict := assertConflict(t, err, "approved")
		assert.Equal(t, "token-sql", conflict.ClaimToken)
	})

	t.Run("sql claim survives store reopen", func(t *testing.T) {
		// 文件库模拟进程重启：写库、关闭连接、以全新 store/连接重开，读回
		// 同一持久意图。这是 ARCH-002「重启后可查」的验收（B 项）。
		dbPath := t.TempDir() + "/claim-reopen.db"
		db, err := gorm.Open(gsqlite.Open(dbPath), &gorm.Config{})
		require.NoError(t, err)
		store, err := NewSQLStore(db)
		require.NoError(t, err)
		seedPendingApproval(t, store, "claim-reopen")
		_, err = store.Approve("claim-reopen", "op-a")
		require.NoError(t, err)
		token, err := NewClaimToken()
		require.NoError(t, err)
		digest := PayloadDigest(&Approval{ID: "claim-reopen", FunctionID: "f.x", Actor: "requester"})
		_, err = store.ClaimForDispatch("claim-reopen", token, digest)
		require.NoError(t, err)

		sqlDB, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		reopened, err := gorm.Open(gsqlite.Open(dbPath), &gorm.Config{})
		require.NoError(t, err)
		store2, err := NewSQLStore(reopened)
		require.NoError(t, err)
		got, err := store2.Get("claim-reopen")
		require.NoError(t, err)
		assert.Equal(t, "approved", got.State)
		assert.Equal(t, "claimed", got.ClaimStatus)
		assert.Equal(t, token, got.ClaimToken)
		assert.Equal(t, digest, got.PayloadDigest)
		require.NotNil(t, got.ClaimedAt)

		// 新实例再次领取必须失败（持久意图只成立一次）。
		_, err = store2.ClaimForDispatch("claim-reopen", "token-reopen-2", digest)
		conflict := assertConflict(t, err, "approved")
		assert.Equal(t, token, conflict.ClaimToken)
	})

	t.Run("sql concurrent claims exactly one wins", func(t *testing.T) {
		store, err := NewSQLStore(openSharedTestDB(t))
		require.NoError(t, err)
		seedPendingApproval(t, store, "claim-sql-race")
		_, err = store.Approve("claim-sql-race", "op-a")
		require.NoError(t, err)

		const n = 8
		results := make(chan error, n)
		var start sync.WaitGroup
		start.Add(1)
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				start.Wait()
				_, err := store.ClaimForDispatch("claim-sql-race", fmt.Sprintf("token-%d", i), "digest")
				results <- err
			}(i)
		}
		start.Done()
		wg.Wait()

		winners, conflicts := 0, 0
		for i := 0; i < n; i++ {
			err := <-results
			switch {
			case err == nil:
				winners++
			case errors.Is(err, ErrAlreadyReviewed):
				conflicts++
			default:
				t.Fatalf("unexpected error: %v", err)
			}
		}
		assert.Equal(t, 1, winners, "exactly one dispatch claim may win")
		assert.Equal(t, n-1, conflicts, "all other claims must conflict")
	})

	t.Run("memory matches sql semantics", func(t *testing.T) {
		store := NewMemStore()
		seedPendingApproval(t, store, "claim-mem")
		_, err := store.Approve("claim-mem", "op-a")
		require.NoError(t, err)

		winner, err := store.ClaimForDispatch("claim-mem", "token-mem", "digest")
		require.NoError(t, err)
		assert.Equal(t, "claimed", winner.ClaimStatus)
		assert.Equal(t, "token-mem", winner.ClaimToken)

		_, err = store.ClaimForDispatch("claim-mem", "token-next", "digest")
		assertConflict(t, err, "approved")
	})

	t.Run("pending cannot claim", func(t *testing.T) {
		store := NewMemStore()
		seedPendingApproval(t, store, "claim-pending")
		_, err := store.ClaimForDispatch("claim-pending", "token", "digest")
		require.Error(t, err)
		_, ok := AsAlreadyReviewed(err)
		assert.False(t, ok)
	})
}
