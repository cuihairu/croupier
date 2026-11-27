package model

import "time"

// BusinessOperationCredential 是高风险业务操作的游戏侧执行凭据（GAME-009
// 批准协议 D2，2026-11-27 用户拍板）：一个 operationId 唯一对应一次业务意图，
// 凭据行是响应丢失后查询收敛（D4）与「取消不谎称回滚」（D5）的事实来源。
// 与 taskId/页面幂等（平台传输层）互不替代：本表是业务 exactly-once 的平台
// 半边，游戏侧持久去重由接入方按 durableIdempotency 能力声明自证（D3）。
type BusinessOperationCredential struct {
	ID uint `gorm:"primaryKey"`
	// 唯一作用域 (game_id, env, operation_id)：operationId 本身已编码
	// scope+domain（D1 复合标识），唯一索引是同 ID 重复执行的物理判据。
	GameID      string `gorm:"size:64;not null;uniqueIndex:idx_biz_op_cred_scope,priority:1"`
	Env         string `gorm:"size:64;not null;uniqueIndex:idx_biz_op_cred_scope,priority:2"`
	OperationID string `gorm:"size:128;not null;uniqueIndex:idx_biz_op_cred_scope,priority:3;index"`
	// Domain 是意图域（player/mail/payment 等），取自 operationId 第三段。
	Domain string `gorm:"size:64;not null;index"`
	// FunctionID 是承载该业务操作的契约函数。
	FunctionID string `gorm:"size:128;not null;index"`
	// RequestFingerprint 是「契约函数 + 参数规范化 JSON」的 SHA-256：
	// 同 operationId 携带不同参数的重放（R5 参数漂移）在此判 409。
	RequestFingerprint string `gorm:"size:64;not null"`
	// RequestPayload 是首次登记时的参数快照（D4 自动重试须同 operationId 同
	// 快照重放）；重放命中既有行不覆盖快照。列经 0070 加列迁移补齐。
	RequestPayload JSON `gorm:"type:json"`
	// Outcome 取值见 Outcome* 常量：pending（已登记、副作用未落定）→
	// succeeded/failed/retryable_failed/unknown → canceled/cancel_failed。
	// unknown 只能经查询收敛改写，绝不允许被直接改写为 succeeded（D4）。
	Outcome string `gorm:"size:32;not null;index"`
	// GameReceipt 是游戏侧回执（GameOperationResult.gameReceipt）原文；
	// 列表展示脱敏，操作员/审计走受控全读路径。
	GameReceipt string `gorm:"size:512"`
	// ResultJSON 是信封 result 字段的 JSON 快照。
	ResultJSON JSON `gorm:"type:json"`
	// QueryHint 是契约 durableIdempotency 声明的读结果函数（D4 自动查询
	// 据此调用游戏方）；空表示接入方未声明查询函数，unknown 只能人工收敛。
	QueryHint string `gorm:"size:128"`
	// CancelHint 是契约 durableIdempotency 声明的取消确认函数（D5）；
	// 高风险函数取消必须经它取得游戏确认后才可报 canceled。
	CancelHint string `gorm:"size:128"`
	// QueryAttempts / RetryAttempts 分别计量 D4 自动查询（上限 10）与
	// 自动重试（上限 8，同 operationId 同快照）。
	QueryAttempts int `gorm:"not null;default:0"`
	RetryAttempts int `gorm:"not null;default:0"`
	// Actor 是发起业务意图的主体（审批/操作员工单），审计链引用。
	Actor     string    `gorm:"size:128;index"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (BusinessOperationCredential) TableName() string {
	return "business_operation_credentials"
}

// 凭据 outcome 状态机（D4/D5）。状态迁移规则：
// pending → succeeded/failed/retryable_failed/unknown（执行回执）；
// retryable_failed/unknown →（同 ID 同快照自动重试，≤8 次）任意终态；
// unknown →（读结果函数查询，≤10 次）succeeded/failed，查询耗尽保持 unknown 并告警；
// pending/运行中 →（取消确认函数确认后）canceled；确认失败 → cancel_failed
// 且原 result 保留——取消从不隐含回滚。
const (
	BizOpOutcomePending         = "pending"
	BizOpOutcomeSucceeded       = "succeeded"
	BizOpOutcomeFailed          = "failed"
	BizOpOutcomeRetryableFailed = "retryable_failed"
	BizOpOutcomeUnknown         = "unknown"
	BizOpOutcomeCanceled        = "canceled"
	BizOpOutcomeCancelFailed    = "cancel_failed"
)

// D4 收敛上限：未知结果自动查询 60s×10、自动重试上限 8（1s→60s 退避）。
const (
	BizOpQueryAttemptsMax = 10
	BizOpRetryAttemptsMax = 8
)
