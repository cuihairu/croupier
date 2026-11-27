package gameops

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGenerateAndValidateOperationID(t *testing.T) {
	id, err := GenerateOperationID("demo", "prod", "player")
	require.NoError(t, err)
	parts := splitForTest(id)
	require.Len(t, parts, 4)
	assert.Equal(t, []string{"demo", "prod", "player"}, parts[:3])
	domain, intent, err := ParseOperationID(id)
	require.NoError(t, err)
	assert.Equal(t, "player", domain)
	assert.Len(t, intent, 36)
	assert.NoError(t, ValidateOperationID(id))

	for _, bad := range []string{
		"", "demo:prod:player", "DEMO:prod:player:00000000-0000-0000-0000-000000000000",
		"demo:prod:player:not-a-uuid",
		"demo:prod:player:018f3b27-5c2e-7a10-8000-000000000001", // v8? no—v7 check below
	} {
		// 末段 v7 校验单独断言；这里只确认非 v7 形态被拒。
		if bad == "demo:prod:player:018f3b27-5c2e-7a10-8000-000000000001" {
			continue
		}
		assert.Error(t, ValidateOperationID(bad), "invalid operationId must be rejected: %q", bad)
	}
	// UUIDv4 不是 v7 → 拒绝（D1 时间有序要求）。
	assert.Error(t, ValidateOperationID("demo:prod:player:550e8400-e29b-41d4-a716-446655440000"))
	// 合法 v7（version nibble=7, variant 8-b）。
	assert.NoError(t, ValidateOperationID("demo:prod:player:018f3b27-5c2e-7b10-8000-000000000001"))
}

func splitForTest(id string) []string {
	out := []string{}
	cur := ""
	for _, ch := range id {
		if ch == ':' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(ch)
	}
	return append(out, cur)
}

func TestFingerprintParams(t *testing.T) {
	a := FingerprintParams("player.ban", json.RawMessage(`{"b":2,"a":1}`))
	b := FingerprintParams("player.ban", json.RawMessage(`{"a":1,"b":2}`))
	c := FingerprintParams("player.ban", json.RawMessage(`{"a":2,"b":1}`))
	d := FingerprintParams("mail.ban", json.RawMessage(`{"b":2,"a":1}`))
	assert.Equal(t, a, b, "键序差异不产生指纹漂移")
	assert.NotEqual(t, a, c, "参数值变化必须改变指纹")
	assert.NotEqual(t, a, d, "函数身份变化必须改变指纹")
}

func TestParseEnvelope_UnknownOnMissingOrMismatch(t *testing.T) {
	const opID = "demo:prod:player:018f3b27-5c2e-7b10-8000-000000000001"
	// 空响应 → unknown + error（不谎称成功）。
	env, err := ParseEnvelope(opID, "f", nil)
	require.Error(t, err)
	assert.Equal(t, EnvelopeOutcomeUnknown, env.Outcome)

	// operationId 不匹配 → 整封不采信。
	env, err = ParseEnvelope(opID, "f", json.RawMessage(`{"operationId":"other","outcome":"succeeded","gameReceipt":"r"}`))
	require.Error(t, err)
	assert.Equal(t, EnvelopeOutcomeUnknown, env.Outcome)
	assert.Empty(t, env.GameReceipt)

	// 指纹漂移 → 拒绝（R5）。
	env, err = ParseEnvelope(opID, "f", json.RawMessage(`{"operationId":"`+opID+`","requestFingerprint":"g","outcome":"succeeded"}`))
	require.Error(t, err)
	assert.Equal(t, EnvelopeOutcomeUnknown, env.Outcome)

	// 合法信封 → 原样解析。
	env, err = ParseEnvelope(opID, "f", json.RawMessage(`{"operationId":"`+opID+`","requestFingerprint":"f","outcome":"succeeded","gameReceipt":"r1","result":{"ok":true}}`))
	require.NoError(t, err)
	assert.Equal(t, EnvelopeOutcomeSucceeded, env.Outcome)
	assert.Equal(t, "r1", env.GameReceipt)
	assert.JSONEq(t, `{"ok":true}`, string(env.Result))
}

func TestCredentialStore_RegisterRetryFingerprintAndTransition(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.BusinessOperationCredential{}))
	ctx := context.Background()
	store := NewCredentialStore(db)
	const opID = "demo:prod:player:018f3b27-5c2e-7b10-8000-000000000001"

	// 1) 首次登记 → pending（副作用尚未落定）。
	rec, created, err := store.Register(ctx, "demo", "prod", opID, "player.ban", "fp-a", "alice", "queryFn", "cancelFn")
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, model.BizOpOutcomePending, rec.Outcome)
	assert.Equal(t, "player", rec.Domain)

	// 2) 同 ID 同指纹重放（响应丢失后的重试）→ 不新建行、不重复执行。
	again, created, err := store.Register(ctx, "demo", "prod", opID, "player.ban", "fp-a", "alice", "", "")
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, rec.ID, again.ID)

	// 3) 同 ID 不同指纹 → R5 参数漂移拒绝。
	_, _, err = store.Register(ctx, "demo", "prod", opID, "player.ban", "fp-b", "alice", "", "")
	assert.True(t, errors.Is(err, ErrFingerprintConflict), "got %v", err)

	// 4) 游戏方权威信封落定成功 → 凭据同步；终态不可被再次改写。
	updated, changed, err := store.ApplyEnvelope(ctx, "demo", "prod", opID, &GameOperationResult{
		OperationID: opID, Outcome: EnvelopeOutcomeSucceeded, GameReceipt: "receipt-1", Result: json.RawMessage(`{"ok":true}`),
	})
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, model.BizOpOutcomeSucceeded, updated.Outcome)
	assert.Equal(t, "receipt-1", updated.GameReceipt)

	_, changed, err = store.ApplyEnvelope(ctx, "demo", "prod", opID, &GameOperationResult{
		OperationID: opID, Outcome: EnvelopeOutcomeFailed,
	})
	require.NoError(t, err)
	assert.False(t, changed, "终态凭据不得被后续响应改写")
	final, err := store.Get(ctx, "demo", "prod", opID)
	require.NoError(t, err)
	assert.Equal(t, model.BizOpOutcomeSucceeded, final.Outcome, "成功凭据不许被伪造失败")
}

func TestCredentialStore_UnknownConvergenceNeverSubstitutes(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.BusinessOperationCredential{}))
	ctx := context.Background()
	store := NewCredentialStore(db)

	// 场景 A：传输无响应（首个 POST 完成副作用后丢响应）→ 落 unknown 不落 failed。
	idA := "demo:prod:mail:018f3b27-5c2e-7b10-8000-000000000002"
	rec, err := store.MarkUnknownOnTransportFailure(ctx, "demo", "prod", idA, "mail.send", "fp-a", "alice")
	require.NoError(t, err)
	assert.Equal(t, model.BizOpOutcomeUnknown, rec.Outcome, "结果未知绝不能标成失败（R2）")

	// 场景 B：查询收敛后游戏权威回执成功 → unknown 可被查询结果改写。
	updated, changed, err := store.ApplyEnvelope(ctx, "demo", "prod", idA, &GameOperationResult{
		OperationID: idA, Outcome: EnvelopeOutcomeSucceeded, GameReceipt: "receipt-2",
	})
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, model.BizOpOutcomeSucceeded, updated.Outcome)

	// 场景 C：查询上限计量与收敛清单（unknown 未超限进清单，超限不进）。
	idB := "demo:prod:mail:018f3b27-5c2e-7b10-8000-000000000003"
	_, _, err = store.Register(ctx, "demo", "prod", idB, "mail.send", "fp-b", "alice", "", "")
	require.NoError(t, err)
	_, err = store.MarkUnknownOnTransportFailure(ctx, "demo", "prod", idB, "mail.send", "fp-b", "alice")
	require.NoError(t, err)
	rows, err := store.ListConvergeable(ctx, "demo", "prod", 10)
	require.NoError(t, err)
	require.Len(t, rows, 1, "仅未收敛的 unknown 进清单")
	assert.Equal(t, idB, rows[0].OperationID)
	for i := 0; i < model.BizOpQueryAttemptsMax; i++ {
		n, err := store.BumpQueryAttempts(ctx, "demo", "prod", idB)
		require.NoError(t, err)
		assert.Equal(t, i+1, n)
	}
	rows, err = store.ListConvergeable(ctx, "demo", "prod", 10)
	require.NoError(t, err)
	assert.Empty(t, rows, "查询耗尽后不再自动收敛，只告警")
}

func TestCredentialStore_CancellationRequiresGameConfirmation(t *testing.T) {
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.BusinessOperationCredential{}))
	ctx := context.Background()
	store := NewCredentialStore(db)
	const opID = "demo:prod:player:018f3b27-5c2e-7b10-8000-000000000004"
	_, _, err = store.Register(ctx, "demo", "prod", opID, "player.ban", "fp", "alice", "", "cancelFn")
	require.NoError(t, err)

	// 取消确认失败 → cancel_failed，原 result 保留，状态不谎称 canceled。
	_, changed, err := store.ApplyEnvelope(ctx, "demo", "prod", opID, &GameOperationResult{
		OperationID: opID, Outcome: EnvelopeOutcomeCancelFailed, Result: json.RawMessage(`{"granted":true}`),
	})
	require.NoError(t, err)
	assert.True(t, changed)
	rec, err := store.Get(ctx, "demo", "prod", opID)
	require.NoError(t, err)
	assert.Equal(t, model.BizOpOutcomeCancelFailed, rec.Outcome)
	assert.JSONEq(t, `{"granted":true}`, string(rec.ResultJSON), "取消失败不得抹除已发放结果")

	// 游戏确认取消成功 → 才允许 canceled。
	const opID2 = "demo:prod:player:018f3b27-5c2e-7b10-8000-000000000005"
	_, _, err = store.Register(ctx, "demo", "prod", opID2, "player.ban", "fp", "alice", "", "cancelFn")
	require.NoError(t, err)
	_, changed, err = store.ApplyEnvelope(ctx, "demo", "prod", opID2, &GameOperationResult{
		OperationID: opID2, Outcome: EnvelopeOutcomeCanceled,
	})
	require.NoError(t, err)
	assert.True(t, changed)
	rec2, err := store.Get(ctx, "demo", "prod", opID2)
	require.NoError(t, err)
	assert.Equal(t, model.BizOpOutcomeCanceled, rec2.Outcome)
}
