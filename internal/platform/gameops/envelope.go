package gameops

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EnvelopeOutcome 取值与 model.BizOpOutcome* 同词汇：游戏侧信封自报的业务
// 结果分类。unknown 只能被查询收敛，平台永不把它改写成成功（D4）。
const (
	EnvelopeOutcomeSucceeded       = "succeeded"
	EnvelopeOutcomeFailed          = "failed"
	EnvelopeOutcomeRetryableFailed = "retryable_failed"
	EnvelopeOutcomeUnknown         = "unknown"
	EnvelopeOutcomeConflict        = "conflict"
	EnvelopeOutcomeCanceled        = "canceled"
	EnvelopeOutcomeCancelFailed    = "cancel_failed"
)

// GameOperationResult 是高风险函数必须返回的业务信封（D3）：游戏 SDK 把
// operationId/requestFingerprint 原样回传，平台据此核对身份与快照一致性；
// outcome 是游戏方对副作用的权威分类，而非传输层成败。
type GameOperationResult struct {
	// OperationID 必须与请求携带的 operationId 一致；不一致视为游戏方
	// 违约，信封整体按 unknown 处理（不采信任何字段）。
	OperationID string `json:"operationId"`
	// RequestFingerprint 必须与平台计算指纹一致；不一致即 R5 参数漂移。
	RequestFingerprint string `json:"requestFingerprint,omitempty"`
	// Outcome 见 EnvelopeOutcome*；空值按 unknown 处理（不谎称成功）。
	Outcome string `json:"outcome"`
	// GameReceipt 是游戏侧持久凭据/回执标识（查询收敛的引用锚点）。
	GameReceipt string `json:"gameReceipt,omitempty"`
	// Result 是业务结果负载原文。
	Result json.RawMessage `json:"result,omitempty"`
	// RetryableAfter 秒数：游戏方建议的重试冷却（0 表示无建议）。
	RetryableAfter int `json:"retryableAfter,omitempty"`
	// QueryHint 覆盖契约声明的读结果函数（游戏方可在响应中动态指定）。
	QueryHint string `json:"queryHint,omitempty"`
}

// ParseEnvelope 校验并归一游戏侧返回：信封缺失/身份不匹配一律按 unknown
// 归一（R2：传输成功≠业务成功；宁可 unknown 不可假成功）。
func ParseEnvelope(operationID, fingerprint string, payload json.RawMessage) (*GameOperationResult, error) {
	if len(payload) == 0 {
		return &GameOperationResult{OperationID: operationID, Outcome: EnvelopeOutcomeUnknown},
			fmt.Errorf("empty response: game did not return a %s envelope", "GameOperationResult")
	}
	var envelope GameOperationResult
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return &GameOperationResult{OperationID: operationID, Outcome: EnvelopeOutcomeUnknown},
			fmt.Errorf("response is not a GameOperationResult envelope: %w", err)
	}
	if strings.TrimSpace(envelope.OperationID) != operationID {
		// 身份不匹配：返回的信封不可归属本操作，字段一概不采信。
		return &GameOperationResult{OperationID: operationID, Outcome: EnvelopeOutcomeUnknown},
			fmt.Errorf("envelope operationId %q does not match request %q", envelope.OperationID, operationID)
	}
	if fingerprint != "" && strings.TrimSpace(envelope.RequestFingerprint) != "" &&
		envelope.RequestFingerprint != fingerprint {
		return &GameOperationResult{OperationID: operationID, Outcome: EnvelopeOutcomeUnknown},
			fmt.Errorf("envelope requestFingerprint mismatch: replay may be drifting parameters")
	}
	if strings.TrimSpace(envelope.Outcome) == "" {
		envelope.Outcome = EnvelopeOutcomeUnknown
	}
	return &envelope, nil
}

// IsTerminalOutcome 判定凭据是否已达不再自动处理的终态：
// succeeded/failed/canceled 是终态；retryable_failed/unknown 参与自动重试
// 与查询收敛；cancel_failed 保持原凭据、只走补偿（新审批 + 新 operationId）。
func IsTerminalOutcome(outcome string) bool {
	switch outcome {
	case EnvelopeOutcomeSucceeded, EnvelopeOutcomeFailed, EnvelopeOutcomeCanceled:
		return true
	}
	return false
}
