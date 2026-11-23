package audit

import (
	"context"
)

// 敏感写路径的统一留痕三态（AUD-049）。
const (
	StateSuccess = "success"
	StateFailure = "failure"
	StateDenied  = "denied"
)

// LogWrite 是敏感写路径的统一审计出口：成功/失败/denied 三态共用同一事件
// 类型与字段形态，差异只在 outcome。
//
// 调用方契约（与 ARCH-008 D4 一致）：业务动作已经（或将要）发生，审计写
// 失败不得把业务结果改成 5xx；因此本函数吞掉 Log 的返回错误并转
// ReportWriteGap 记缺口。
//
// actor 由调用方从请求上下文解析后传入（audit 包不得 import 业务侧的
// actor 解析器，否则与 logic/utils → svc → audit 成环）；解析不到时留空，
// 记录仍带事件类型/outcome/details，可凭 eventType + details 回查。
func LogWrite(s *AuditService, ctx context.Context, event AuditEventType, state, actor, resourceType, resourceID string, details map[string]interface{}) {
	if s == nil || ctx == nil {
		return
	}
	opts := []AuditOption{WithOutcome(state, "")}
	if resourceType != "" {
		opts = append(opts, WithResourceID(resourceType, resourceID))
	}
	if len(details) > 0 {
		opts = append(opts, WithDetails(details))
	}
	if actor != "" {
		opts = append(opts, WithActorID(actor, "user", actor))
	}
	if _, err := s.Log(ctx, event, opts...); err != nil {
		s.ReportWriteGap(ctx, event, err)
	}
}
