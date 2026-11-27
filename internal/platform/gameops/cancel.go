package gameops

// GAME-009 批准协议 D5 取消确认边界：平台绝不自行把凭据标 canceled，
// 必须调用契约 x-durable-idempotency.cancel 声明的游戏侧确认函数，由游戏
// 返回权威信封（canceled / cancel_failed）才可迁移状态。cancel_failed
// 保留原 result（不抹除已发放结果）；撤销/补偿永远是新审批 + 新 operationId。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/model"
)

// CancelInvoker 是取消确认函数的调用面（平台内部派发的最小接口，便于
// 单测替换；生产由 functioninvoke 包注入 dispatcher 调用）。
type CancelInvoker interface {
	InvokeCancel(ctx context.Context, functionID string, payload []byte, gameID, env, actor string) (json.RawMessage, error)
}

// ErrCancelNotSupported 表示凭据未声明 cancelHint：平台没有游戏侧确认能力，
// 不得报告 canceled。ErrCancelNotApplicable 表示凭据已带权威终态（succeeded/
// failed/canceled/cancel_failed）：契约状态机规定终态只能被游戏查询结果修正，
// 撤销既有结果走补偿/撤销（新审批 + 新 operationId），不再进入取消确认流。
var (
	ErrCancelNotSupported       = errors.New("credential has no cancel confirmation function (x-durable-idempotency.cancel)")
	ErrCancelNotApplicable      = errors.New("credential already has a terminal outcome; undo/compensation requires a new operationId")
	ErrCancelChannelUnavailable = errors.New("cancel confirmation channel unavailable")
)

// ConfirmCancellation 调游戏侧 cancelHint 函数并迁移凭据：
//   - 信封 canceled → 凭据 canceled（D5 唯一允许报 canceled 的路径）；
//   - 信封 cancel_failed → 凭据 cancel_failed，原 result 保留；
//   - 其他 outcome / 非法信封 / 派发失败 → 凭据维持原状态并返回错误，
//     绝不发明取消成功，也不把「派发失败」谎报 cancel_failed（原始结果
//     仍在凭据里，调用方可以再次确认）。
func ConfirmCancellation(ctx context.Context, store *CredentialStore, invoker CancelInvoker, gameID, env, operationID, actor string) (*CredentialReconcile, error) {
	if store == nil || invoker == nil {
		return nil, ErrCancelChannelUnavailable
	}
	cred, err := store.Get(ctx, gameID, env, operationID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cred.CancelHint) == "" {
		return nil, ErrCancelNotSupported
	}
	switch cred.Outcome {
	case model.BizOpOutcomeSucceeded, model.BizOpOutcomeFailed, model.BizOpOutcomeCanceled, model.BizOpOutcomeCancelFailed:
		// 终态不可改写（契约 §5）：不派发取消确认，提示走补偿/撤销。
		return nil, ErrCancelNotApplicable
	}
	payload, err := json.Marshal(map[string]string{
		"operationId": operationID,
		"gameReceipt": cred.GameReceipt,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal cancel payload: %w", err)
	}
	response, invokeErr := invoker.InvokeCancel(ctx, cred.CancelHint, payload, gameID, env, actor)
	if invokeErr != nil {
		return nil, fmt.Errorf("cancel confirmation dispatch failed: %w", invokeErr)
	}
	envelope, parseErr := ParseEnvelope(operationID, cred.RequestFingerprint, response)
	if parseErr != nil {
		return nil, parseErr
	}
	if envelope.Outcome != EnvelopeOutcomeCanceled && envelope.Outcome != EnvelopeOutcomeCancelFailed {
		return nil, fmt.Errorf("cancel confirmation returned outcome %q (expected canceled/cancel_failed)", envelope.Outcome)
	}
	updated, _, applyErr := store.ApplyEnvelope(ctx, gameID, env, operationID, envelope)
	if applyErr != nil {
		return nil, applyErr
	}
	return &CredentialReconcile{
		OperationID: updated.OperationID,
		Outcome:     updated.Outcome,
		GameReceipt: updated.GameReceipt,
		Result:      json.RawMessage(updated.ResultJSON),
	}, nil
}

// CredentialReconcile 是取消确认/查询凭据的对外视图（API DTO 同构源）。
type CredentialReconcile struct {
	OperationID string          `json:"operationId"`
	Outcome     string          `json:"outcome"`
	GameReceipt string          `json:"gameReceipt,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// QueryCredential 供 API 查询凭据收敛结果（D4 客户端凭 operationId 查）。
func QueryCredential(ctx context.Context, store *CredentialStore, gameID, env, operationID string) (*CredentialReconcile, error) {
	cred, err := store.Get(ctx, gameID, env, operationID)
	if err != nil {
		return nil, err
	}
	return &CredentialReconcile{
		OperationID: cred.OperationID,
		Outcome:     cred.Outcome,
		GameReceipt: cred.GameReceipt,
		Result:      json.RawMessage(cred.ResultJSON),
	}, nil
}
