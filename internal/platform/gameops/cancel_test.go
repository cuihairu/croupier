package gameops

// GAME-010 slice 4：D5 取消确认语义单测。平台侧不变式：
//   - canceled 只能来自游戏信封（无 cancelHint / 无通道 → 拒绝，不发明）；
//   - cancel_failed 保留原 result（不抹除已发放结果）；
//   - 派发失败 / 信封非法 / outcome 不可采信 → 凭据原样不动，返回错误。

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	gsqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// stubCancelInvoker 按脚本回放信封或错误。
type stubCancelInvoker struct {
	payload json.RawMessage
	err     error
	calls   int
}

func (s *stubCancelInvoker) InvokeCancel(_ context.Context, _ string, _ []byte, _, _, _ string) (json.RawMessage, error) {
	s.calls++
	return s.payload, s.err
}

const cancelOpID = "cg:env-a:demo:018f3b27-5c2e-7b10-8000-0000000000aa"

// newCancelTestDB 独立 :memory: 库 + 凭据表（单连接防多实例漂移）。
func newCancelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.BusinessOperationCredential{}))
	return db
}

// seedCancelCredential 登记凭据并直置为 outcome 态（resultOnly=true 时附带
// 部分写入 result 快照，模拟 unknown 前游戏已部分执行）。
func seedCancelCredential(t *testing.T, db *gorm.DB, outcome, cancelHint string, withResult bool) {
	t.Helper()
	store := NewCredentialStore(db)
	_, _, err := store.Register(context.Background(), "cg", "env-a", cancelOpID, "demo.durable", "fp-cancel", []byte(`{"x":1}`), "alice", "demo.query", cancelHint)
	require.NoError(t, err)
	updates := map[string]interface{}{"outcome": outcome}
	if withResult {
		updates["game_receipt"] = "receipt-original"
		updates["result_json"] = model.JSON(`{"granted":true}`)
	}
	require.NoError(t, db.Model(&model.BusinessOperationCredential{}).Where("operation_id = ?", cancelOpID).Updates(updates).Error)
}

func cancelEnvelope(outcome string) json.RawMessage {
	return json.RawMessage(`{"operationId":"` + cancelOpID + `","requestFingerprint":"fp-cancel","outcome":"` + outcome + `","gameReceipt":"receipt-cancel"}`)
}

func TestConfirmCancellation_CanceledApplies(t *testing.T) {
	db := newCancelTestDB(t)
	store := NewCredentialStore(db)
	// 未决操作（pending）：取消确认的唯一目标态（契约 §5 cancel_requested 链）。
	seedCancelCredential(t, db, model.BizOpOutcomePending, "demo.cancel", false)
	invoker := &stubCancelInvoker{payload: cancelEnvelope(EnvelopeOutcomeCanceled)}

	view, err := ConfirmCancellation(context.Background(), store, invoker, "cg", "env-a", cancelOpID, "bob")
	require.NoError(t, err)
	assert.Equal(t, EnvelopeOutcomeCanceled, view.Outcome)
	assert.Equal(t, "receipt-cancel", view.GameReceipt)
	assert.Equal(t, 1, invoker.calls)

	updated := getCancelCredential(t, db)
	assert.Equal(t, model.BizOpOutcomeCanceled, updated.Outcome)
}

// cancel_failed 保留原 result：部分写入的结果不因取消失败被抹除（D5/R4）。
func TestConfirmCancellation_CancelFailedKeepsOriginalResult(t *testing.T) {
	db := newCancelTestDB(t)
	store := NewCredentialStore(db)
	seedCancelCredential(t, db, model.BizOpOutcomeUnknown, "demo.cancel", true)
	invoker := &stubCancelInvoker{payload: cancelEnvelope(EnvelopeOutcomeCancelFailed)}

	view, err := ConfirmCancellation(context.Background(), store, invoker, "cg", "env-a", cancelOpID, "bob")
	require.NoError(t, err)
	assert.Equal(t, EnvelopeOutcomeCancelFailed, view.Outcome)

	updated := getCancelCredential(t, db)
	assert.Equal(t, model.BizOpOutcomeCancelFailed, updated.Outcome)
	assert.Equal(t, "receipt-original", updated.GameReceipt, "原回执保留")
	assert.JSONEq(t, `{"granted":true}`, string(updated.ResultJSON), "原 result 保留")
}

// 已带权威终态（succeeded）的凭据不可再取消：契约 §5 终态只能被游戏查询结果
// 修正，撤销既有结果走补偿/撤销（新审批 + 新 operationId），不派发确认。
func TestConfirmCancellation_TerminalCredentialRefused(t *testing.T) {
	for _, outcome := range []string{model.BizOpOutcomeSucceeded, model.BizOpOutcomeFailed, model.BizOpOutcomeCanceled, model.BizOpOutcomeCancelFailed} {
		db := newCancelTestDB(t)
		store := NewCredentialStore(db)
		seedCancelCredential(t, db, outcome, "demo.cancel", true)
		invoker := &stubCancelInvoker{payload: cancelEnvelope(EnvelopeOutcomeCanceled)}

		_, err := ConfirmCancellation(context.Background(), store, invoker, "cg", "env-a", cancelOpID, "bob")
		require.ErrorIs(t, err, ErrCancelNotApplicable, "outcome %s", outcome)
		assert.Equal(t, 0, invoker.calls, "终态凭据不得派发取消确认")
		updated := getCancelCredential(t, db)
		assert.Equal(t, outcome, updated.Outcome)
	}
}

// 游戏回非取消 outcome（如 succeeded）→ 不可采信，凭据原样、报错。
func TestConfirmCancellation_RejectsNonCancelOutcome(t *testing.T) {
	db := newCancelTestDB(t)
	store := NewCredentialStore(db)
	seedCancelCredential(t, db, model.BizOpOutcomePending, "demo.cancel", false)
	invoker := &stubCancelInvoker{payload: cancelEnvelope(EnvelopeOutcomeSucceeded)}

	_, err := ConfirmCancellation(context.Background(), store, invoker, "cg", "env-a", cancelOpID, "bob")
	require.Error(t, err)
	updated := getCancelCredential(t, db)
	assert.Equal(t, model.BizOpOutcomePending, updated.Outcome)
}

// 派发失败 → 凭据原样不动（不谎报 cancel_failed），可再次确认。
func TestConfirmCancellation_DispatchFailureLeavesCredentialIntact(t *testing.T) {
	db := newCancelTestDB(t)
	store := NewCredentialStore(db)
	seedCancelCredential(t, db, model.BizOpOutcomePending, "demo.cancel", false)
	invoker := &stubCancelInvoker{err: context.DeadlineExceeded}

	_, err := ConfirmCancellation(context.Background(), store, invoker, "cg", "env-a", cancelOpID, "bob")
	require.Error(t, err)
	updated := getCancelCredential(t, db)
	assert.Equal(t, model.BizOpOutcomePending, updated.Outcome)
}

func TestConfirmCancellation_RequiresCancelHint(t *testing.T) {
	db := newCancelTestDB(t)
	store := NewCredentialStore(db)
	seedCancelCredential(t, db, model.BizOpOutcomePending, "", false)
	invoker := &stubCancelInvoker{payload: cancelEnvelope(EnvelopeOutcomeCanceled)}

	_, err := ConfirmCancellation(context.Background(), store, invoker, "cg", "env-a", cancelOpID, "bob")
	require.ErrorIs(t, err, ErrCancelNotSupported)
	assert.Equal(t, 0, invoker.calls, "未声明取消函数不得派发")
}

func TestConfirmCancellation_NotFoundAndNilGuard(t *testing.T) {
	db := newCancelTestDB(t)
	store := NewCredentialStore(db)
	_, err := ConfirmCancellation(context.Background(), store, &stubCancelInvoker{}, "cg", "env-a", cancelOpID, "bob")
	require.ErrorIs(t, err, ErrCredentialNotFound)

	_, err = ConfirmCancellation(context.Background(), store, nil, "cg", "env-a", cancelOpID, "bob")
	require.ErrorIs(t, err, ErrCancelChannelUnavailable)
}

// QueryCredential 只读视图：回传凭据当前 outcome/result，不改状态。
func TestQueryCredentialView(t *testing.T) {
	db := newCancelTestDB(t)
	store := NewCredentialStore(db)
	seedCancelCredential(t, db, model.BizOpOutcomeUnknown, "demo.cancel", true)

	view, err := QueryCredential(context.Background(), store, "cg", "env-a", cancelOpID)
	require.NoError(t, err)
	assert.Equal(t, model.BizOpOutcomeUnknown, view.Outcome)
	assert.Equal(t, "receipt-original", view.GameReceipt)
	assert.JSONEq(t, `{"granted":true}`, string(view.Result))

	updated := getCancelCredential(t, db)
	assert.Equal(t, model.BizOpOutcomeUnknown, updated.Outcome)
}

func getCancelCredential(t *testing.T, db *gorm.DB) *model.BusinessOperationCredential {
	t.Helper()
	store := NewCredentialStore(db)
	cred, err := store.Get(context.Background(), "cg", "env-a", cancelOpID)
	require.NoError(t, err)
	return cred
}
