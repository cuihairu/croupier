package functioninvoke

// GAME-009 批准协议 D4 自动收敛循环（GAME-010 slice 3）：扫描凭据表里的
// unknown / retryable_failed 行，分别走查询收敛与同 ID 同快照自动重试。
//
// 派发口径（有意不走 Govern）：收敛是平台内部对「已授权且已执行过一次」意图
// 的续跑——retryable_failed 意味着游戏侧已执行并自报可重试失败，重放的是同一
// operationId + 同一参数快照（指纹钉死），审批意图在首次派发时已兑现；重跑
// 完整治理链会在策略变更时反复新建审批单（审批队列污染）或在操作员撤权后
// 把 unknown 凭据永久滞留。收敛只信任凭据行里已登记的事实（函数、快照、
// operationId），能力声明（x-durable-idempotency.query）本身就是接入方
// 授权平台调用的契约面。边界：凭据 Actor 撤权不影响收敛（意图已登记），
// 审计面由执行留痕（ExecutionLog）承担。

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/gameops"
	"github.com/cuihairu/croupier/internal/svc"
)

// 收敛节拍与上限（GAME-009 D4）：查询 60s 一轮 ×10、重试退避 1s→60s 封顶 ×8。
const (
	convergeScanInterval = 60 * time.Second
	convergeBatchSize    = 50
	retryBackoffCap      = 60 * time.Second
)

// OperationConverger 驱动 D4 自动收敛的后台循环。
type OperationConverger struct {
	svcCtx *svc.ServiceContext
	store  *gameops.CredentialStore
	log    *slog.Logger
}

func NewOperationConverger(svcCtx *svc.ServiceContext, logger *slog.Logger) *OperationConverger {
	if logger == nil {
		logger = slog.Default()
	}
	return &OperationConverger{
		svcCtx: svcCtx,
		store:  gameops.NewCredentialStore(svcCtx.DB),
		log:    logger,
	}
}

// Start 阻塞运行收敛循环（调用方 go converger.Start(ctx)）。DB 未装配时
// 不启动（单测/裁剪构建）。
func (c *OperationConverger) Start(ctx context.Context) {
	if c.svcCtx == nil || c.svcCtx.DB == nil {
		return
	}
	ticker := time.NewTicker(convergeScanInterval)
	defer ticker.Stop()
	c.log.Info("operation converge loop started", "interval", convergeScanInterval.String())
	for {
		select {
		case <-ctx.Done():
			c.log.Info("operation converge loop stopped")
			return
		case <-ticker.C:
			c.tick(ctx)
		}
	}
}

func (c *OperationConverger) tick(ctx context.Context) {
	rows, err := c.store.ListConvergeable(ctx, "", "", convergeBatchSize)
	if err != nil {
		c.log.Warn("operation converge scan failed", "error", err)
		return
	}
	for i := range rows {
		cred := rows[i]
		c.convergeOne(scopedContext(ctx, cred), cred)
	}
}

// scopedContext 把凭据归属 scope 注入 ctx（multiGame 分库路由 + 派发路由
// 校验都以 ctx scope 为准）。
func scopedContext(ctx context.Context, cred model.BusinessOperationCredential) context.Context {
	return svc.WithGameScope(ctx, svc.GameScope{GameID: cred.GameID, Env: cred.Env})
}

func (c *OperationConverger) convergeOne(ctx context.Context, cred model.BusinessOperationCredential) {
	switch cred.Outcome {
	case model.BizOpOutcomeUnknown:
		c.convergeUnknown(ctx, cred)
	case model.BizOpOutcomeRetryableFailed:
		c.convergeRetryable(ctx, cred)
	}
}

// convergeUnknown 走 x-durable-idempotency.query 声明的读结果函数查询收敛：
// 每轮一次查询 + 一次计量；查询拿到权威信封即迁移凭据；查询耗尽（10 次）
// 落告警（alerts 表，平台级）交人工处理——绝不发明终态。
func (c *OperationConverger) convergeUnknown(ctx context.Context, cred model.BusinessOperationCredential) {
	if strings.TrimSpace(cred.QueryHint) == "" {
		// 未声明查询函数：unknown 只能人工收敛（D3 契约），不计量、不重试。
		return
	}
	payload, _ := json.Marshal(map[string]string{"operationId": cred.OperationID})
	envelope, dispatchErr := c.dispatchInternal(ctx, cred.QueryHint, payload, cred)
	if dispatchErr != nil {
		c.bumpQueryAndCheck(ctx, cred, "query dispatch failed: "+dispatchErr.Error())
		return
	}
	if envelope == nil {
		c.bumpQueryAndCheck(ctx, cred, "query returned no result")
		return
	}
	updated, _, applyErr := c.store.ApplyEnvelope(ctx, cred.GameID, cred.Env, cred.OperationID, envelope)
	if applyErr != nil {
		c.log.Warn("operation converge apply failed", "operationId", cred.OperationID, "error", applyErr)
		return
	}
	if updated != nil && gameops.IsTerminalOutcome(updated.Outcome) {
		c.log.Info("operation converged via query", "operationId", cred.OperationID, "outcome", updated.Outcome)
	}
}

// bumpQueryAndCheck 计量一次查询；达到上限落告警（不发明终态，凭据保持
// unknown 等人工）。
func (c *OperationConverger) bumpQueryAndCheck(ctx context.Context, cred model.BusinessOperationCredential, reason string) {
	n, err := c.store.BumpQueryAttempts(ctx, cred.GameID, cred.Env, cred.OperationID)
	if err != nil {
		c.log.Warn("operation converge bump failed", "operationId", cred.OperationID, "error", err)
		return
	}
	c.log.Warn("operation query inconclusive", "operationId", cred.OperationID, "attempts", n, "reason", reason)
	if n >= model.BizOpQueryAttemptsMax {
		c.raiseExhaustedAlert(ctx, cred, "query", n)
	}
}

// convergeRetryable 走同 operationId 同参数快照自动重试（退避 1s→60s 封顶，
// ≤8 次）。重放复用凭据登记的幂等语义：同 ID 同指纹命中同一行，游戏侧按
// operationId 去重（R1/R3）。退避未到点的行本轮跳过（updated_at 是上次
// 动作时间戳）。
func (c *OperationConverger) convergeRetryable(ctx context.Context, cred model.BusinessOperationCredential) {
	if len(cred.RequestPayload) == 0 {
		// 无快照（0070 前登记）：自动重试无从重放，交人工，不计量。
		return
	}
	backoff := retryBackoff(cred.RetryAttempts)
	if time.Since(cred.UpdatedAt) < backoff {
		return
	}
	envelope, invokeErr := c.dispatchInternal(ctx, cred.FunctionID, cred.RequestPayload, cred)
	if invokeErr != nil {
		c.log.Warn("operation retry dispatch failed", "operationId", cred.OperationID, "error", invokeErr)
	}
	if envelope != nil {
		if updated, _, applyErr := c.store.ApplyEnvelope(ctx, cred.GameID, cred.Env, cred.OperationID, envelope); applyErr != nil {
			c.log.Warn("operation converge apply failed", "operationId", cred.OperationID, "error", applyErr)
		} else if updated != nil && gameops.IsTerminalOutcome(updated.Outcome) {
			c.log.Info("operation converged via retry", "operationId", cred.OperationID, "outcome", updated.Outcome)
		}
	}
	// 无论收敛结果如何，本次尝试都已消耗，计量。
	n, err := c.store.BumpRetryAttempts(ctx, cred.GameID, cred.Env, cred.OperationID)
	if err != nil {
		c.log.Warn("operation retry bump failed", "operationId", cred.OperationID, "error", err)
		return
	}
	if n >= model.BizOpRetryAttemptsMax {
		c.raiseExhaustedAlert(ctx, cred, "retry", n)
	}
}

// retryBackoff 1s→2s→4s…封顶 60s（D4：1s→60s 退避，按已重试次数翻倍）。
func retryBackoff(attempts int) time.Duration {
	if attempts < 0 {
		attempts = 0
	}
	if attempts > 10 {
		return retryBackoffCap
	}
	d := time.Second << uint(attempts)
	if d > retryBackoffCap {
		return retryBackoffCap
	}
	return d
}

// dispatchInternal 平台内部收敛派发（直连 Dispatcher，不经 Govern，见文件头
// 口径说明）。返回游戏侧信封由调用方单次落库（ApplyEnvelope 终态保护在
// store 内）；传输失败/响应非信封时凭据落 unknown（R2：绝不发明终态）。
func (c *OperationConverger) dispatchInternal(ctx context.Context, functionID string, payload []byte, cred model.BusinessOperationCredential) (*gameops.GameOperationResult, error) {
	metadata := map[string]string{
		"async":       "false",
		"operationId": cred.OperationID,
		"gameId":      cred.GameID,
		"env":         cred.Env,
		"actor":       cred.Actor,
		"converge":    "true",
	}
	resp, err := c.svcCtx.Dispatcher.InvokeRequest(ctx, utils.BuildInvokeRequest(strings.TrimSpace(functionID), payload, metadata))
	if err != nil {
		// 传输失败 → 凭据落 unknown（R2：绝不落 failed），等待查询收敛。
		if _, markErr := c.store.MarkUnknownOnTransportFailure(ctx, cred.GameID, cred.Env, cred.OperationID, cred.FunctionID, cred.RequestFingerprint, cred.RequestPayload, cred.Actor); markErr != nil {
			c.log.Warn("operation converge mark-unknown failed", "operationId", cred.OperationID, "error", markErr)
		}
		return nil, err
	}
	if resp == nil || len(resp.GetPayload()) == 0 {
		return nil, nil
	}
	envelope, parseErr := gameops.ParseEnvelope(cred.OperationID, cred.RequestFingerprint, resp.GetPayload())
	if parseErr != nil {
		// 有响应但不是合法信封：按未知收敛（平台不发明终态）。
		if _, markErr := c.store.MarkUnknownOnTransportFailure(ctx, cred.GameID, cred.Env, cred.OperationID, cred.FunctionID, cred.RequestFingerprint, cred.RequestPayload, cred.Actor); markErr != nil {
			c.log.Warn("operation converge mark-unknown failed", "operationId", cred.OperationID, "error", markErr)
		}
		return nil, parseErr
	}
	return envelope, nil
}

// raiseExhaustedAlert 查询/重试耗尽告警（alerts 表，platform 级——凭据
// scope 随 Details 携带）。告警失败只记日志，不影响凭据状态。
func (c *OperationConverger) raiseExhaustedAlert(ctx context.Context, cred model.BusinessOperationCredential, kind string, attempts int) {
	if c.svcCtx.AlertModel == nil {
		return
	}
	alertID := "bizop-exhausted-" + cred.OperationID
	if existing, err := c.svcCtx.AlertModel.FindByAlertID(ctx, alertID); err == nil && existing != nil {
		return
	}
	err := c.svcCtx.AlertModel.Create(ctx, &model.Alert{
		AlertID: alertID,
		Type:    "business_operation_converge_exhausted",
		Level:   "warning",
		Message: "业务操作凭据自动收敛耗尽，需人工处理（" + kind + " " + itoa(attempts) + " 次）",
		Source:  "gameops",
		Status:  "firing",
		Details: map[string]interface{}{
			"operationId": cred.OperationID,
			"functionId":  cred.FunctionID,
			"outcome":     cred.Outcome,
			"kind":        kind,
			"attempts":    attempts,
		},
		GameID: cred.GameID,
		Env:    cred.Env,
	})
	if err != nil {
		c.log.Warn("operation converge alert create failed", "operationId", cred.OperationID, "error", err)
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
