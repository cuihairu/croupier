package functioninvoke

// GAME-010 slice 3：D4 自动收敛循环的确定性单测。不依赖 60s ticker——
// 直接调用 convergeOne/tick，用 per-function stub caller 返回权威信封或
// 失败，断言凭据状态、计量与告警闸。派发经 Dispatcher（不经 Govern）的
// 口径说明见 converge.go 文件头。

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/cuihairu/croupier/core/transport"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/gameops"
	"github.com/cuihairu/croupier/internal/svc"
	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

const convergeOpID = "test-game:test-env:demo:018f3b27-5c2e-7b10-8000-000000000010"

// perFuncCaller 按 FunctionId 回放不同 payload：查询函数（demo.query）回
// 权威信封，业务函数（demo.durable）回可重试失败——模拟「直调失败但查询
// 面可答」的真实游戏拓扑。
type perFuncCaller struct {
	payloads map[string][]byte
	err      error
}

func (c *perFuncCaller) Call(_ context.Context, msgID uint32, body []byte) (uint32, []byte, error) {
	if c.err != nil {
		return 0, nil, c.err
	}
	switch msgID {
	case protocol.MsgInvokeRequest:
		req := &sdkv1.InvokeRequest{}
		if err := proto.Unmarshal(body, req); err != nil {
			return 0, nil, err
		}
		payload, ok := c.payloads[req.GetFunctionId()]
		if !ok {
			return 0, nil, assert.AnError
		}
		resp := &sdkv1.InvokeResponse{Payload: payload}
		b, merr := proto.Marshal(resp)
		if merr != nil {
			return 0, nil, merr
		}
		return protocol.MsgInvokeResponse, b, nil
	}
	return 0, nil, nil
}

func setupConverger(t *testing.T, caller transport.SessionCaller) (*OperationConverger, *svc.ServiceContext) {
	t.Helper()
	svcCtx, _ := setupDispatchSvc(t, map[string]transport.SessionCaller{"a1": caller})
	require.NoError(t, svcCtx.DB.AutoMigrate(&model.BusinessOperationCredential{}, &model.Alert{}))
	svcCtx.AlertModel = model.NewAlertModel(svcCtx.DB)
	registerDispatchAgent(t, svcCtx.RegistryStore, "a1", "demo.query", "demo.durable")
	return NewOperationConverger(svcCtx, slog.New(slog.NewTextHandler(io.Discard, nil))), svcCtx
}

func insertCredential(t *testing.T, svcCtx *svc.ServiceContext, outcome string, queryHint string) model.BusinessOperationCredential {
	t.Helper()
	cred := model.BusinessOperationCredential{
		GameID: "test-game", Env: "test-env", OperationID: convergeOpID,
		Domain: "demo", FunctionID: "demo.durable",
		RequestFingerprint: "fp", RequestPayload: model.JSON(`{"x":1}`),
		Outcome: outcome, QueryHint: queryHint,
		CreatedAt: time.Now().Add(-time.Hour), UpdatedAt: time.Now().Add(-time.Hour),
	}
	require.NoError(t, svcCtx.DB.Create(&cred).Error)
	return cred
}

func getCredential(t *testing.T, svcCtx *svc.ServiceContext) model.BusinessOperationCredential {
	t.Helper()
	var cred model.BusinessOperationCredential
	require.NoError(t, svcCtx.DB.Where("operation_id = ?", convergeOpID).First(&cred).Error)
	return cred
}

func succeedQueryPayload() map[string][]byte {
	return map[string][]byte{
		"demo.query": []byte(`{"operationId":"` + convergeOpID + `","outcome":"succeeded","gameReceipt":"from-query"}`),
	}
}

func TestConvergeUnknown_QueryEnvelopeConverges(t *testing.T) {
	caller := &perFuncCaller{payloads: succeedQueryPayload()}
	converger, svcCtx := setupConverger(t, caller)
	cred := insertCredential(t, svcCtx, model.BizOpOutcomeUnknown, "demo.query")

	ctx := scopedContext(context.Background(), cred)
	converger.convergeOne(ctx, cred)

	updated := getCredential(t, svcCtx)
	assert.Equal(t, model.BizOpOutcomeSucceeded, updated.Outcome)
	assert.Equal(t, "from-query", updated.GameReceipt)
	assert.Equal(t, 0, updated.QueryAttempts, "权威信封一次收敛，不烧计量")
}

func TestConvergeUnknown_NoQueryHintStaysUnknown(t *testing.T) {
	caller := &perFuncCaller{payloads: succeedQueryPayload()}
	converger, svcCtx := setupConverger(t, caller)
	cred := insertCredential(t, svcCtx, model.BizOpOutcomeUnknown, "")

	converger.convergeOne(scopedContext(context.Background(), cred), cred)
	updated := getCredential(t, svcCtx)
	assert.Equal(t, model.BizOpOutcomeUnknown, updated.Outcome, "未声明查询函数不得自动收敛")
	assert.Equal(t, 0, updated.QueryAttempts)
}

// 直调函数回 retryable_failed 信封 → 收敛循环重放（同快照同 ID）→ 游戏
// 回 succeeded 信封 → 凭据收敛终态，重试计量 +1。
func TestConvergeRetryable_ReplaySameSnapshot(t *testing.T) {
	caller := &perFuncCaller{payloads: map[string][]byte{
		"demo.query":   succeedQueryPayload()["demo.query"],
		"demo.durable": []byte(`{"operationId":"` + convergeOpID + `","outcome":"retryable_failed"}`),
	}}
	converger, svcCtx := setupConverger(t, caller)
	cred := insertCredential(t, svcCtx, model.BizOpOutcomeRetryableFailed, "")

	converger.convergeOne(scopedContext(context.Background(), cred), cred)
	updated := getCredential(t, svcCtx)
	assert.Equal(t, model.BizOpOutcomeRetryableFailed, updated.Outcome, "游戏自报可重试失败保持 retryable_failed")
	assert.Equal(t, 1, updated.RetryAttempts)

	// 游戏侧修复后重放回 succeeded。
	caller.payloads["demo.durable"] = []byte(`{"operationId":"` + convergeOpID + `","outcome":"succeeded","gameReceipt":"retry-ok"}`)
	require.NoError(t, svcCtx.DB.Model(&model.BusinessOperationCredential{}).
		Where("operation_id = ?", convergeOpID).
		Update("updated_at", time.Now().Add(-time.Hour)).Error)
	converger.convergeOne(scopedContext(context.Background(), cred), cred)
	updated = getCredential(t, svcCtx)
	assert.Equal(t, model.BizOpOutcomeSucceeded, updated.Outcome)
	assert.Equal(t, 2, updated.RetryAttempts)
	assert.Equal(t, "retry-ok", updated.GameReceipt)
}

// 退避闸：attempts=1 → 2s 退避；updated_at 刚更新 → 本轮不派发、不计量。
func TestConvergeRetryable_BackoffGateSkipsFreshRow(t *testing.T) {
	caller := &perFuncCaller{payloads: succeedQueryPayload()}
	converger, svcCtx := setupConverger(t, caller)
	cred := insertCredential(t, svcCtx, model.BizOpOutcomeRetryableFailed, "")
	require.NoError(t, svcCtx.DB.Model(&model.BusinessOperationCredential{}).
		Where("operation_id = ?", convergeOpID).
		Update("retry_attempts", 1).Error)
	cred.RetryAttempts = 1
	cred.UpdatedAt = time.Now() // 刚失败，退避未到点

	converger.convergeOne(scopedContext(context.Background(), cred), cred)
	updated := getCredential(t, svcCtx)
	assert.Equal(t, model.BizOpOutcomeRetryableFailed, updated.Outcome)
	assert.Equal(t, 1, updated.RetryAttempts, "退避未到点不派发不计量")
}

// 查询耗尽：10 轮全失败 → 凭据保持 unknown（绝不发明终态）+ 落告警 +
// ListConvergeable 不再供给。
func TestConvergeExhaustion_NeverInventsTerminalAndAlerts(t *testing.T) {
	caller := &perFuncCaller{payloads: map[string][]byte{}, err: context.DeadlineExceeded}
	converger, svcCtx := setupConverger(t, caller)
	insertCredential(t, svcCtx, model.BizOpOutcomeUnknown, "demo.query")

	for i := 0; i < model.BizOpQueryAttemptsMax; i++ {
		current := getCredential(t, svcCtx)
		require.NoError(t, svcCtx.DB.Model(&model.BusinessOperationCredential{}).
			Where("operation_id = ?", convergeOpID).
			Update("updated_at", time.Now().Add(-time.Hour)).Error)
		current.UpdatedAt = time.Now().Add(-time.Hour)
		converger.convergeOne(scopedContext(context.Background(), current), current)
		after := getCredential(t, svcCtx)
		require.Equal(t, i+1, after.QueryAttempts, "每轮查询计量一次")
	}

	updated := getCredential(t, svcCtx)
	assert.Equal(t, model.BizOpOutcomeUnknown, updated.Outcome, "查询耗尽仍 unknown")
	var alerts []model.Alert
	require.NoError(t, svcCtx.DB.Where("alert_id = ?", "bizop-exhausted-"+convergeOpID).Find(&alerts).Error)
	require.Len(t, alerts, 1, "耗尽必须落告警")
	assert.Equal(t, "business_operation_converge_exhausted", alerts[0].Type)

	// 耗尽后不再自动收敛。
	rows, err := gameops.NewCredentialStore(svcCtx.DB).ListConvergeable(context.Background(), "test-game", "test-env", 10)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// tick 只处理 ListConvergeable 供给的行；终态行不被改写。
func TestConvergeTick_TerminalRowUntouched(t *testing.T) {
	caller := &perFuncCaller{payloads: succeedQueryPayload()}
	converger, svcCtx := setupConverger(t, caller)
	// 直接以终态信封落定（不经收敛循环），再跑 tick。
	store := gameops.NewCredentialStore(svcCtx.DB)
	insertCredential(t, svcCtx, model.BizOpOutcomePending, "demo.query")
	_, _, err := store.ApplyEnvelope(context.Background(), "test-game", "test-env", convergeOpID, &gameops.GameOperationResult{
		OperationID: convergeOpID, Outcome: gameops.EnvelopeOutcomeSucceeded, GameReceipt: "original",
	})
	require.NoError(t, err)

	converger.tick(context.Background())
	updated := getCredential(t, svcCtx)
	assert.Equal(t, model.BizOpOutcomeSucceeded, updated.Outcome)
	assert.Equal(t, "original", updated.GameReceipt, "终态凭据不得被收敛循环改写")
}

func TestRetryBackoffCap(t *testing.T) {
	assert.Equal(t, time.Second, retryBackoff(0))
	assert.Equal(t, 2*time.Second, retryBackoff(1))
	assert.Equal(t, 16*time.Second, retryBackoff(4))
	assert.Equal(t, retryBackoffCap, retryBackoff(20))
}
