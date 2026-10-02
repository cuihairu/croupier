package approval

import (
	"context"
	"strings"

	"github.com/cuihairu/croupier/internal/api/function"
	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/approvals"
	"github.com/cuihairu/croupier/internal/platform/settings"
	policymgr "github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/security/otp"
	"github.com/cuihairu/croupier/internal/svc"
)

// 高危审批 step-up TOTP 门槛（OPEN-ISSUES #75，移动端三段确认的服务端落点）。
//
// 此前 Approve 只读 URI 不读请求体，Web/Mobile 按设计稿发送的 otp 被静默
// 丢弃，step-up TOTP 双端形同虚设。策略在实现处定死如下：
//
//   - 治理风险 high / danger（default policy 中强制审批的两档，与调用路径
//     共用 function.ResolvePolicyRisk 同一口径）→ 必须二次验证：
//     已绑定 TOTP 的账号 otp 必填且须通过校验；未绑定账号直接拒绝
//     （otp_not_enrolled，提示回 Web 绑定），高危批准禁止静默放行。
//   - 总开关 security.approvalStepUpOtp（OPEN-ISSUES #61，默认 true）：
//     关闭后高危档降级为中低风险语义（otp 可选、给了就校验），批准审计记
//     disabled 旁路档——移动端定位是便利，强制门槛必须可由服务端一键降级，
//     但缺省姿态不变（fail-safe）。
//   - 中低风险：otp 可选；提供了就校验，错的拒绝（防误填静默通过）。
//
// 稳定错误码（API Response Contract）：
//
//	otp_required      403：高危审批未带 otp
//	otp_invalid       400：带的 otp 未通过校验（含超长等畸形输入）
//	otp_not_enrolled  403：高危审批但账号未绑定 TOTP
//
// otp 值本身永不写入审计/日志；审计只记结果档位（not_required / totp）。
// 恢复码不参与审批 step-up（仅登录路径可消费），避免审批动作意外烧码。
const (
	stepUpResultNone     = "not_required"
	stepUpResultTOTP     = "totp"
	stepUpResultDisabled = "disabled"
	// maxStepUpOTPLen 只为在入库前拦掉畸形长输入（TOTP 恒 6 位），
	// 真实校验交给 otp.VerifyTOTP。
	maxStepUpOTPLen = 16
)

// stepUpOTPEnforced 读 L3 总开关 security.approvalStepUpOtp（OPEN-ISSUES
// #61）。默认 true：settings 未初始化或键未配置时维持 #75 的强制语义——
// 开关只用于「出问题时一键降级」，不改变缺省安全姿态。
func stepUpOTPEnforced() bool {
	l := settings.Current()
	if l == nil {
		return true
	}
	return l.GetBool(settings.KeySecurityApprovalStepUpOTP, true)
}

// approveStepUpRisk 与调用路径共用同一风险判定口径（OPEN-ISSUES #75）。
func approveStepUpRisk(ctx context.Context, svcCtx *svc.ServiceContext, record *approvals.Approval) policymgr.RiskLevel {
	if svcCtx == nil || record == nil {
		return policymgr.RiskMedium
	}
	return function.ResolvePolicyRisk(ctx, svcCtx, record.FunctionID, record.GameID, record.Env)
}

// lookupOperator 找不到（含 AdminModel 未注入的测试环境）返回 nil，
// 调用侧按「未绑定 TOTP」处理——fail-closed。
func (s *Service) lookupOperator(ctx context.Context, operator string) *model.Admin {
	if s == nil || s.svcCtx == nil || s.svcCtx.AdminModel == nil || operator == "" {
		return nil
	}
	admin, err := s.svcCtx.AdminModel.FindByUsername(ctx, operator)
	if err != nil || admin == nil {
		return nil
	}
	return admin
}

// enforceStepUpOTP 在批准动作落库前执行 step-up 校验；通过时返回结果档位
// （随 approval.approved 审计行透出），未通过时返回稳定错误码错误。
func (s *Service) enforceStepUpOTP(ctx context.Context, operator string, record *approvals.Approval, rawOTP string) (string, error) {
	code := strings.TrimSpace(rawOTP)
	risk := approveStepUpRisk(ctx, s.svcCtx, record)
	highRisk := risk == policymgr.RiskHigh || risk == policymgr.RiskDanger

	admin := s.lookupOperator(ctx, operator)
	enrolled := admin != nil && admin.OTPEnabled && strings.TrimSpace(admin.OTPSecret) != ""

	// 总开关关闭（#61）：高危档降级为中低风险语义，但批准审计记 disabled
	// 旁路档——事后能区分「开关放行」与「天然不需要」。带了 otp 仍校验
	// （与中低档同规：错的拒绝，防误填静默通过）。
	if highRisk && !stepUpOTPEnforced() {
		outcome := stepUpResultDisabled
		if code != "" {
			if len(code) > maxStepUpOTPLen || !enrolled || !otp.VerifyTOTP(admin.OTPSecret, code, 1) {
				s.recordStepUpAudit(ctx, operator, record, "otp_invalid")
				return "", stepUpInvalid()
			}
			outcome = stepUpResultTOTP
		}
		return outcome, nil
	}

	if !highRisk {
		if code == "" {
			return stepUpResultNone, nil
		}
		if len(code) > maxStepUpOTPLen {
			return "", stepUpInvalid()
		}
		if enrolled && otp.VerifyTOTP(admin.OTPSecret, code, 1) {
			return stepUpResultTOTP, nil
		}
		s.recordStepUpAudit(ctx, operator, record, "otp_invalid")
		return "", stepUpInvalid()
	}

	if !enrolled {
		s.recordStepUpAudit(ctx, operator, record, "not_enrolled")
		return "", errorx.NewForbiddenWithCode("otp_not_enrolled",
			"该审批属高危操作，当前账号未绑定两步验证，请先在 Web 端「个人中心-安全设置」绑定后再批准", nil)
	}
	if code == "" {
		s.recordStepUpAudit(ctx, operator, record, "otp_required")
		return "", errorx.NewForbiddenWithCode("otp_required",
			"该审批属高危操作，请输入动态验证码后重试",
			map[string]any{"functionId": record.FunctionID})
	}
	if len(code) > maxStepUpOTPLen || !otp.VerifyTOTP(admin.OTPSecret, code, 1) {
		s.recordStepUpAudit(ctx, operator, record, "otp_invalid")
		return "", stepUpInvalid()
	}
	return stepUpResultTOTP, nil
}

func stepUpInvalid() *errorx.CodeError {
	return errorx.NewBadRequestWithCode("otp_invalid", "动态验证码错误或已过期", nil)
}

// recordStepUpAudit 记 step-up 未通过的审计行（outcome ∈
// not_enrolled / otp_required / otp_invalid），供安全侧追溯重试风暴。
func (s *Service) recordStepUpAudit(ctx context.Context, operator string, record *approvals.Approval, outcome string) {
	if s == nil || s.svcCtx == nil || s.svcCtx.AuditService == nil || record == nil {
		return
	}
	_, _ = s.svcCtx.AuditService.Log(ctx, audit.EventApprovalStepUpFailed,
		audit.WithActorID(operator, "admin", operator),
		audit.WithResourceID("approval", record.ID),
		audit.WithGameID(record.GameID, record.Env),
		audit.WithDetails(map[string]interface{}{
			"approvalId": record.ID,
			"functionId": record.FunctionID,
			"actor":      record.Actor,
			"operator":   operator,
			"outcome":    outcome,
		}),
	)
}
