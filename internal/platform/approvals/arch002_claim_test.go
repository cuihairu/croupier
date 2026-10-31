package approvals

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ARCH-002 D7：双实例共享同一 SQL 存储——批准 CAS 只产生一个胜者；胜出者
// ClaimDispatch 得到单调递增 token；其余请求因已领取返回 ErrAlreadyClaimed，
// 且绝不再触发续跑。用两个独立 SQLStore（同共享 :memory: 库，各自连接）模拟
// 双实例。
func TestArch002_SQLTwoInstancesExactlyOneClaimAndDispatch(t *testing.T) {
	db := openSharedTestDB(t)
	storeA, err := NewSQLStore(db)
	require.NoError(t, err)
	storeB, err := NewSQLStore(db)
	require.NoError(t, err)

	_, err = storeA.Create(&Approval{
		ID: "arch002-2inst", State: "pending", FunctionID: "f.x",
		GameID: "g", Env: "prod", Actor: "requester",
		Payload: []byte(`{"x":1}`), Mode: "invoke",
	})
	require.NoError(t, err)

	// 两实例并发批准 + 领取。只有 CAS 胜者（恰好一个）能完成 ClaimDispatch。
	const n = 2
	type outcome struct {
		rec      *Approval
		approved bool
		claimed  bool
		conflict bool
	}
	results := make(chan outcome, n)
	var start sync.WaitGroup
	start.Add(1)
	for i := 0; i < n; i++ {
		store := storeA
		if i == 1 {
			store = storeB
		}
		go func() {
			start.Wait()
			rec, err := store.Approve("arch002-2inst", "op")
			if err != nil {
				results <- outcome{conflict: true}
				return
			}
			claim, err := store.ClaimDispatch(rec.ID, "fp")
			if err != nil {
				results <- outcome{approved: true, conflict: true}
				return
			}
			results <- outcome{approved: true, claimed: true, rec: claim}
		}()
	}
	start.Done()

	winners, claimants := 0, 0
	winnerToken := int64(0)
	for i := 0; i < n; i++ {
		out := <-results
		if out.approved {
			winners++
			if out.claimed {
				claimants++
				winnerToken = out.rec.ClaimToken
				assert.Equal(t, "claimed", out.rec.ClaimStatus)
			}
		}
	}
	assert.Equal(t, 1, winners, "exactly one approve wins")
	assert.Equal(t, 1, claimants, "exactly one claimant wins dispatch intent")
	assert.True(t, winnerToken > 0, "winner token must be positive")

	// 已领取后重复领取 → ErrAlreadyClaimed（不再续跑）。
	_, err = storeA.ClaimDispatch("arch002-2inst", "fp2")
	require.Error(t, err)
	_, ok := AsAlreadyClaimed(err)
	assert.True(t, ok, "duplicate claim must be rejected with ErrAlreadyClaimed")

	// 结果回写按 token 条件完成，状态转 dispatched。
	updated, err := storeB.CompleteDispatch("arch002-2inst", winnerToken, "sync", "", []byte(`{"ok":true}`))
	require.NoError(t, err)
	assert.Equal(t, "dispatched", updated.ClaimStatus)
	assert.Equal(t, "sync", updated.ResultKind)
	assert.Equal(t, `{"ok":true}`, string(updated.Result))

	// 陈旧 token 回写被 fencing 拒绝。
	_, err = storeA.CompleteDispatch("arch002-2inst", winnerToken-1, "sync", "", nil)
	require.Error(t, err)
	_, ok = AsAlreadyClaimed(err)
	assert.True(t, ok, "stale token write-back must be fenced")
}

// ARCH-002 验收：重启可查。同一 SQL 库上关闭并重开 SQLStore，已批准记录仍可读，
// 且领取字段持久（不依赖内存）。
func TestArch002_SQLRestartPersistsApprovalAndClaim(t *testing.T) {
	db := openSharedTestDB(t)
	store1, err := NewSQLStore(db)
	require.NoError(t, err)

	_, err = store1.Create(&Approval{
		ID: "arch002-restart", State: "pending", FunctionID: "f.x",
		GameID: "g", Env: "dev", Actor: "u",
	})
	require.NoError(t, err)
	approved, err := store1.Approve("arch002-restart", "admin")
	require.NoError(t, err)
	claim, err := store1.ClaimDispatch(approved.ID, "fp")
	require.NoError(t, err)

	// 重开一个全新 SQLStore（模拟重启），仍能读到已批准 + 已领取。
	store2, err := NewSQLStore(db)
	require.NoError(t, err)
	got, err := store2.Get("arch002-restart")
	require.NoError(t, err)
	assert.Equal(t, "approved", got.State)
	assert.Equal(t, "claimed", got.ClaimStatus)
	assert.Equal(t, claim.ClaimToken, got.ClaimToken)
	assert.Equal(t, "fp", got.DispatchPayloadFingerprint)

	// 重启后已完成回写。
	done, err := store2.CompleteDispatch("arch002-restart", claim.ClaimToken, "task", "task-1", nil)
	require.NoError(t, err)
	assert.Equal(t, "dispatched", done.ClaimStatus)
	assert.Equal(t, "task-1", done.TaskID)

	// 再次重启可查终态。
	store3, err := NewSQLStore(db)
	require.NoError(t, err)
	final, err := store3.Get("arch002-restart")
	require.NoError(t, err)
	assert.Equal(t, "dispatched", final.ClaimStatus)
	assert.Equal(t, "task-1", final.TaskID)
}

// MemStore 与 SQLStore 的 D7 语义一致（CAS 后领取 + fencing）。
func TestArch002_MemStoreClaimFencing(t *testing.T) {
	store := NewMemStore()
	_, err := store.Create(&Approval{ID: "m-claim", State: "pending"})
	require.NoError(t, err)
	rec, err := store.Approve("m-claim", "op")
	require.NoError(t, err)
	claim, err := store.ClaimDispatch(rec.ID, "fp")
	require.NoError(t, err)
	assert.Equal(t, "claimed", claim.ClaimStatus)
	assert.True(t, claim.ClaimToken > 0)

	_, err = store.ClaimDispatch("m-claim", "fp2")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAlreadyClaimed))

	done, err := store.CompleteDispatch("m-claim", claim.ClaimToken, "sync", "", []byte(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "dispatched", done.ClaimStatus)
	_, err = store.CompleteDispatch("m-claim", claim.ClaimToken-1, "sync", "", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAlreadyClaimed))
}
