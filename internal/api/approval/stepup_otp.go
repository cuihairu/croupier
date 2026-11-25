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

// Step-up TOTP gate for high-risk approvals (OPEN-ISSUES #75, the server-side
// landing point for the mobile three-part confirmation).
//
// Previously Approve only read the URI and ignored the request body, so the otp
// that Web/Mobile send as designed was silently dropped and the step-up TOTP
// was a dead letter on both ends. The policy is fixed here in the implementation:
//
//   - Governance risk high / danger (the two tiers that force approval in the
//     default policy, sharing the same judgement as the invocation path via
//     function.ResolvePolicyRisk) → second verification is mandatory:
//     accounts bound to TOTP must supply a valid otp; unbound accounts are
//     rejected outright (otp_not_enrolled, pointing back to Web for binding),
//     and high-risk approvals must never be let through silently.
//   - Master switch security.approvalStepUpOtp (OPEN-ISSUES #61, default
//     true): once turned off, high risk degrades to medium/low risk semantics
//     (otp optional, validated when supplied) and the approval audit records the
//     disabled bypass tier — the mobile end is meant to be convenient, so a
//     mandatory gate must be degradable with one server-side toggle,
//     while the default posture stays unchanged (fail-safe).
//   - Medium/low risk: otp is optional; if supplied it is validated, and a wrong
//     one is rejected (preventing a typo from passing silently).
//
// Stable error codes (API Response Contract):
//
//	otp_required      403: a high-risk approval sent without an otp
//	otp_invalid       400: the supplied otp failed validation
//	                   (malformed overlong code and the like)
//	otp_not_enrolled  403: a high-risk approval, but the account has no
//	                   TOTP bound
//
// The otp value itself is never written to the audit log or logs; the audit
// only records the outcome tier (not_required / totp).
// Recovery codes do not take part in the approval step-up (only the login path
// may consume them), avoiding an approval action accidentally burning a code.
const (
	stepUpResultNone     = "not_required"
	stepUpResultTOTP     = "totp"
	stepUpResultDisabled = "disabled"
	// maxStepUpOTPLen exists only to reject malformed overlong input before it
	// is persisted (a TOTP code is always 6 digits);
	// real validation is delegated to otp.VerifyTOTP.
	maxStepUpOTPLen = 16
)

// stepUpOTPEnforced reads the L3 master switch security.approvalStepUpOtp
// (OPEN-ISSUES #61). The default is true: when settings are not initialized or
// the key is not configured, the mandatory semantics of #75 are kept —
// the switch only serves as a "one-click degrade when something goes wrong"
// lever and does not change the default secure posture.
func stepUpOTPEnforced() bool {
	l := settings.Current()
	if l == nil {
		return true
	}
	return l.GetBool(settings.KeySecurityApprovalStepUpOTP, true)
}

// approveStepUpRisk shares the same risk judgement criteria as the invocation
// path (OPEN-ISSUES #75).
// R30-02 (ARCH-025): when the risk source fails, fail closed and treat it as
// high risk (requiring second verification); a read failure must never
// silently downgrade the tier to medium/low risk and relax the step-up gate.
func approveStepUpRisk(ctx context.Context, svcCtx *svc.ServiceContext, record *approvals.Approval) policymgr.RiskLevel {
	if svcCtx == nil || record == nil {
		return policymgr.RiskMedium
	}
	risk, err := function.ResolvePolicyRisk(ctx, svcCtx, record.FunctionID, record.GameID, record.Env)
	if err != nil {
		return policymgr.RiskHigh
	}
	return risk
}

// lookupOperator returns nil when it finds nothing (including test
// environments where AdminModel is not injected);
// the caller then treats it as "TOTP not bound" — fail closed.
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

// enforceStepUpOTP runs the step-up validation before the approve action lands
// in the database; on success it returns the outcome tier
// (surfaced with the approval.approved audit row), and on failure a stable
// error code.
func (s *Service) enforceStepUpOTP(ctx context.Context, operator string, record *approvals.Approval, rawOTP string) (string, error) {
	code := strings.TrimSpace(rawOTP)
	risk := approveStepUpRisk(ctx, s.svcCtx, record)
	highRisk := risk == policymgr.RiskHigh || risk == policymgr.RiskDanger

	admin := s.lookupOperator(ctx, operator)
	enrolled := admin != nil && admin.OTPEnabled && strings.TrimSpace(admin.OTPSecret) != ""

	// Master switch off (#61): high risk degrades to medium/low risk semantics,
	// but the approval audit records the disabled bypass tier — after the fact
	// one can tell apart "let through by the switch" from "not needed at all".
	// A supplied otp is still validated (same rules as the medium/low tiers:
	// a wrong one is rejected, preventing a typo from passing silently).
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

// recordStepUpAudit writes an audit row for a step-up failure (outcome ∈
// not_enrolled / otp_required / otp_invalid), so the security side can trace
// retry storms.
func (s *Service) recordStepUpAudit(ctx context.Context, operator string, record *approvals.Approval, outcome string) {
	if s == nil || s.svcCtx == nil || s.svcCtx.AuditService == nil || record == nil {
		return
	}
	_, auditErr := s.svcCtx.AuditService.Log(ctx, audit.EventApprovalStepUpFailed,
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
	if auditErr != nil {
		s.svcCtx.AuditService.ReportWriteGap(ctx, audit.EventApprovalStepUpFailed, auditErr)
	}
}
