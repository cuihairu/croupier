package croupier

// GAME-009 批准协议 D3 游戏侧信封（GAME-010 SDK 半边）：高风险函数必须以
// GameOperationResult 信封返回业务结果——operationId 原样回传（身份锚点），
// outcome 是游戏方对副作用的权威分类（succeeded/failed/retryable_failed/
// unknown/conflict/canceled/cancel_failed），而非传输层成败。平台据此收敛
// 凭据（D4 查询/自动重试，D5 取消确认）；信封缺席时平台只能记 unknown，
// 绝不发明终态。
//
// operationId 由平台派发时经 invoke metadata 携带（"operationId" 键），游戏
// 侧用 OperationIDFromContext 取出后原样回传即可；requestFingerprint 可省略
// （平台侧钉死同 ID 同参数，省略不产生漂移风险）。

import (
	"context"
	"encoding/json"
	"strings"
)

// MetadataOperationID 是平台派发 durable 幂等函数时 invoke metadata 的键。
const MetadataOperationID = "operationId"

// durable operationId 的 ctx 传播键（tcp invoke 路径经 WithOperationID 注入）。
type operationIDContextKey struct{}

// WithOperationID 把平台派发的 operationId 写入 handler ctx（tcp invoke 路径
// 自动调用；无则原样返回）。
func WithOperationID(ctx context.Context, operationID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(operationID) == "" {
		return ctx
	}
	return context.WithValue(ctx, operationIDContextKey{}, operationID)
}

// OperationIDFromContext 返回本次调用平台登记的 operationId（GAME-009 D1
// 复合标识）；非 durable 函数/未携带时返回空串。
func OperationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(operationIDContextKey{}).(string); ok {
		return v
	}
	return ""
}

// GameOperationResult 是高风险函数必须返回的业务信封（GAME-009 D3）。字段
// 与平台 internal/platform/gameops 的解析面同构（wire 契约，键名固定
// lowerCamelCase，勿改）。
type GameOperationResult struct {
	// OperationID 必须原样回传请求携带的 operationId（身份锚点）。
	OperationID string `json:"operationId"`
	// RequestFingerprint 平台计算的参数指纹；可省略（平台以 operationId
	// 钉死同参数重放），回传时必须与请求一致。
	RequestFingerprint string `json:"requestFingerprint,omitempty"`
	// Outcome 见 EnvelopeOutcome* 常量；空值按 unknown 处理（不谎称成功）。
	Outcome string `json:"outcome"`
	// GameReceipt 游戏侧持久凭据/回执标识（查询收敛与取消确认的引用锚点）。
	GameReceipt string `json:"gameReceipt,omitempty"`
	// Result 业务结果负载原文。
	Result json.RawMessage `json:"result,omitempty"`
	// RetryableAfter 秒数：游戏方建议的重试冷却（0 = 无建议）。
	RetryableAfter int `json:"retryableAfter,omitempty"`
	// QueryHint 覆盖契约声明的读结果函数（响应内动态指定）。
	QueryHint string `json:"queryHint,omitempty"`
}

// EnvelopeOutcome 常量与平台侧同词汇表。
const (
	EnvelopeOutcomeSucceeded       = "succeeded"
	EnvelopeOutcomeFailed          = "failed"
	EnvelopeOutcomeRetryableFailed = "retryable_failed"
	EnvelopeOutcomeUnknown         = "unknown"
	EnvelopeOutcomeConflict        = "conflict"
	EnvelopeOutcomeCanceled        = "canceled"
	EnvelopeOutcomeCancelFailed    = "cancel_failed"
)

// NewOperationEnvelope 构造绑定当前调用的信封：operationId 取 handler ctx
// （平台派发时注入）。非 durable 调用（无 operationId）返回错误——此时不应
// 使用信封协议。
func NewOperationEnvelope(ctx context.Context, outcome string) (*GameOperationResult, error) {
	operationID := OperationIDFromContext(ctx)
	if strings.TrimSpace(operationID) == "" {
		return nil, ErrOperationIDMissing
	}
	return &GameOperationResult{
		OperationID: operationID,
		Outcome:     outcome,
	}, nil
}

// ErrOperationIDMissing 表示本次调用未携带平台 operationId（非 durable 调用）。
var ErrOperationIDMissing = errorString("operationId missing from invoke context (not a durable invocation)")

// errorString 是极简 error 实现（SDK 侧避免引入 errors 包样板）。
type errorString string

func (e errorString) Error() string { return string(e) }

// Marshal 序列化信封为 handler 返回体。
func (e *GameOperationResult) Marshal() ([]byte, error) {
	if e == nil {
		return nil, errorString("nil envelope")
	}
	return json.Marshal(e)
}

// SucceededWith 便捷构造：成功信封 + 回执 + 结果负载。
func SucceededWith(ctx context.Context, gameReceipt string, result json.RawMessage) ([]byte, error) {
	envelope, err := NewOperationEnvelope(ctx, EnvelopeOutcomeSucceeded)
	if err != nil {
		return nil, err
	}
	envelope.GameReceipt = gameReceipt
	envelope.Result = result
	return envelope.Marshal()
}

// FailedWith 便捷构造：最终失败信封（未产生副作用，需新 operationId 重走审批）。
func FailedWith(ctx context.Context, reason string) ([]byte, error) {
	envelope, err := NewOperationEnvelope(ctx, EnvelopeOutcomeFailed)
	if err != nil {
		return nil, err
	}
	envelope.Result = json.RawMessage(`{"reason":` + mustJSONString(reason) + `}`)
	return envelope.Marshal()
}

// RetryableFailedWith 便捷构造：可重试失败信封（同 operationId 平台会自动重试）。
func RetryableFailedWith(ctx context.Context, reason string, retryableAfterSeconds int) ([]byte, error) {
	envelope, err := NewOperationEnvelope(ctx, EnvelopeOutcomeRetryableFailed)
	if err != nil {
		return nil, err
	}
	envelope.RetryableAfter = retryableAfterSeconds
	envelope.Result = json.RawMessage(`{"reason":` + mustJSONString(reason) + `}`)
	return envelope.Marshal()
}

// CanceledWith / CancelFailedWith 便捷构造：取消确认信封（D5）。cancel_failed
// 时 result 指向已发放结果（平台保留原凭据，不因取消失败抹除）。
func CanceledWith(ctx context.Context, gameReceipt string) ([]byte, error) {
	envelope, err := NewOperationEnvelope(ctx, EnvelopeOutcomeCanceled)
	if err != nil {
		return nil, err
	}
	envelope.GameReceipt = gameReceipt
	return envelope.Marshal()
}

func CancelFailedWith(ctx context.Context, reason string) ([]byte, error) {
	envelope, err := NewOperationEnvelope(ctx, EnvelopeOutcomeCancelFailed)
	if err != nil {
		return nil, err
	}
	envelope.Result = json.RawMessage(`{"reason":` + mustJSONString(reason) + `}`)
	return envelope.Marshal()
}

func mustJSONString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}
